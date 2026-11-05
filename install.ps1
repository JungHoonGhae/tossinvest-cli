#Requires -Version 5.1
<#
.SYNOPSIS
    tossinvest-cli (tossctl) Windows installer
.DESCRIPTION
    Downloads the latest tossctl release from GitHub, verifies the SHA256
    checksum, extracts it to %LOCALAPPDATA%\tossctl, and adds that directory
    to the current user's PATH.
.EXAMPLE
    irm https://raw.githubusercontent.com/JungHoonGhae/tossinvest-cli/main/install.ps1 | iex
#>

function Install-Tossctl {
    $ErrorActionPreference = "Stop"

    # Windows PowerShell 5.1 on older hosts may negotiate TLS 1.0/1.1, which
    # GitHub rejects. Ensure TLS 1.2 is enabled before any HTTPS request.
    try {
        [Net.ServicePointManager]::SecurityProtocol =
            [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
    } catch {}

    $REPO        = "JungHoonGhae/tossinvest-cli"
    $BINARY      = "tossctl.exe"
    $ASSET       = "tossctl-windows-amd64"
    $INSTALL_DIR = Join-Path $env:LOCALAPPDATA "tossctl"

    function Write-Step([string]$msg) { Write-Host $msg -ForegroundColor Cyan }
    function Write-Ok([string]$msg)   { Write-Host $msg -ForegroundColor Green }
    function Write-Warn([string]$msg) { Write-Host "Warning: $msg" -ForegroundColor Yellow }

    Write-Step "tossinvest-cli installer for Windows"
    Write-Host ""

    # ── Architecture notice ───────────────────────────────────────────────────
    $arch = $env:PROCESSOR_ARCHITECTURE
    # PROCESSOR_ARCHITEW6432 is set to the native arch when running under WOW64
    if ($env:PROCESSOR_ARCHITEW6432) { $arch = $env:PROCESSOR_ARCHITEW6432 }
    if ($arch -eq "ARM64") {
        Write-Warn "ARM64 detected. No native arm64 build is available yet."
        Write-Warn "The amd64 binary runs under x64 emulation on Windows 11 ARM."
        Write-Host ""
    }

    # ── Resolve download URLs ─────────────────────────────────────────────────
    # TOSSCTL_VERSION pins the payload to the release tag the updater resolved
    # (e.g. v0.54.1). Manual runs default to latest.
    $tag = $env:TOSSCTL_VERSION
    if ([string]::IsNullOrEmpty($tag)) { $tag = "latest" }
    if ($tag -eq "latest") {
        $base = "https://github.com/$REPO/releases/latest/download"
    } else {
        $base = "https://github.com/$REPO/releases/download/$tag"
    }
    $zipUrl    = "$base/$ASSET.zip"
    $sha256Url = "$base/$ASSET.zip.sha256"

    # ── Temporary directory ───────────────────────────────────────────────────
    $tmpDir = Join-Path ([System.IO.Path]::GetTempPath()) ([System.IO.Path]::GetRandomFileName())
    New-Item -ItemType Directory -Path $tmpDir | Out-Null

    try {
        # ── Download ──────────────────────────────────────────────────────────
        Write-Step "Downloading $ASSET.zip..."
        $zipPath    = Join-Path $tmpDir "$ASSET.zip"
        $sha256Path = Join-Path $tmpDir "$ASSET.zip.sha256"

        Invoke-WebRequest -Uri $zipUrl    -OutFile $zipPath    -UseBasicParsing
        Invoke-WebRequest -Uri $sha256Url -OutFile $sha256Path -UseBasicParsing

        # ── Verify checksum ───────────────────────────────────────────────────
        Write-Step "Verifying checksum..."
        $expectedLine = (Get-Content $sha256Path -Raw).Trim()
        $expected = ($expectedLine -split '\s+')[0].ToLower()
        if ($expected -notmatch '^[0-9a-f]{64}$') {
            throw "Could not parse SHA256 from checksum file (got: '$expected'). The asset download may have failed."
        }
        $actual = (Get-FileHash $zipPath -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) {
            throw "Checksum mismatch!`n  expected: $expected`n  actual:   $actual"
        }

        # ── Extract ───────────────────────────────────────────────────────────
        Write-Step "Extracting..."
        $extractDir = Join-Path $tmpDir "extracted"
        Expand-Archive -Path $zipPath -DestinationPath $extractDir -Force

        # ── Install ───────────────────────────────────────────────────────────
        Write-Step "Installing to $INSTALL_DIR..."
        if (-not (Test-Path $INSTALL_DIR)) {
            New-Item -ItemType Directory -Path $INSTALL_DIR | Out-Null
        }

        $destBin = Join-Path $INSTALL_DIR $BINARY
        if (Get-Process -Name "tossctl" -ErrorAction SilentlyContinue) {
            Write-Warn "tossctl is currently running. Close it before upgrading or the file copy may fail."
        }
        Copy-Item (Join-Path $extractDir $BINARY) $destBin -Force
        $srcHelper = Join-Path $extractDir "auth-helper"
        if (Test-Path $srcHelper) {
            $dstHelper = Join-Path $INSTALL_DIR "auth-helper"
            $bakHelper = "$dstHelper.bak"
            # Atomic swap: the old helper stays in place until the new one lands.
            if (Test-Path $dstHelper) {
                if (Test-Path $bakHelper) { Remove-Item $bakHelper -Recurse -Force }
                Rename-Item $dstHelper ([System.IO.Path]::GetFileName($bakHelper))
                try {
                    Copy-Item $srcHelper $dstHelper -Recurse -Force
                    Remove-Item $bakHelper -Recurse -Force
                } catch {
                    Rename-Item $bakHelper ([System.IO.Path]::GetFileName($dstHelper))
                    throw
                }
            } else {
                Copy-Item $srcHelper $dstHelper -Recurse -Force
            }
        }

        # ── PATH (exact-segment, null-safe, REG_EXPAND_SZ-preserving) ─────────
        # Read the RAW (unexpanded) user PATH from the registry. [Environment]::
        # GetEnvironmentVariable expands %VAR% and SetEnvironmentVariable rewrites
        # the value as REG_SZ — that downgrade bakes in / breaks other entries
        # like `%JAVA_HOME%\bin`. We read with DoNotExpandEnvironmentNames and
        # write back as ExpandString (REG_EXPAND_SZ) to leave them intact.
        $envKey  = "HKCU:\Environment"
        $rawPath = (Get-Item $envKey).GetValue(
            "Path", "", [Microsoft.Win32.RegistryValueOptions]::DoNotExpandEnvironmentNames)
        $segments = @()
        if (-not [string]::IsNullOrEmpty($rawPath)) {
            $segments = $rawPath -split ';' | Where-Object { $_ -ne '' }
        }
        if ($segments -notcontains $INSTALL_DIR) {
            $newRaw = (($segments + $INSTALL_DIR) -join ';')
            Set-ItemProperty -Path $envKey -Name "Path" -Value $newRaw -Type ExpandString
            Write-Ok "Added $INSTALL_DIR to PATH (user scope)."
        }
        # Make the new dir usable in the current session too (whether or not it
        # was already persisted), so `tossctl` works without reopening a shell.
        if (($env:PATH -split ';') -notcontains $INSTALL_DIR) {
            $env:PATH = "$env:PATH;$INSTALL_DIR"
        }

        # ── Google Chrome check ───────────────────────────────────────────────
        Write-Host ""
        Write-Step "Checking for Google Chrome (required for auth login)..."
        # Some roots can be undefined (e.g. ProgramFiles(x86) on 32-bit Windows);
        # filter nulls first so Join-Path never throws under -ErrorAction Stop.
        $chromeRoots = @($env:ProgramFiles, ${env:ProgramFiles(x86)}, $env:LOCALAPPDATA) |
            Where-Object { $_ }
        $chromePaths = $chromeRoots | ForEach-Object {
            Join-Path $_ "Google\Chrome\Application\chrome.exe"
        }
        $chromeFound = $chromePaths | Where-Object { Test-Path $_ }
        if ($chromeFound) {
            Write-Ok "Google Chrome found."
        } else {
            Write-Warn "Google Chrome was NOT found. tossctl auth login requires Chrome."
            Write-Host "  Install from: https://www.google.com/chrome/" -ForegroundColor Yellow
        }

        # ── Python / playwright ───────────────────────────────────────────────
        if ($pyCmd) {
            # Pinned to the range auth-helper declares (pyproject.toml): an
            # unpinned `pip install playwright` would float the
            # browser-automation dependency of a tool that handles
            # live-trading credentials.
            $playwrightSpec = "playwright>=1.52,<2"
            # pip prints notices/progress to stderr even on success. With
            # $ErrorActionPreference='Stop', merging stderr via 2>&1 surfaces
            # those lines as a fatal NativeCommandError *before* we can check the
            # exit code (see issue #35). Relax error handling around the native
            # call and rely on $LASTEXITCODE instead.
            $prevEAP = $ErrorActionPreference
            $prevNative = $null
            if (Test-Path Variable:\PSNativeCommandUseErrorActionPreference) {
                $prevNative = $PSNativeCommandUseErrorActionPreference
                $PSNativeCommandUseErrorActionPreference = $false
            }
            $ErrorActionPreference = 'Continue'
            $pipOut = & $pyCmd -m pip install --quiet $playwrightSpec 2>&1
            $pipExit = $LASTEXITCODE
            $ErrorActionPreference = $prevEAP
            if ($null -ne $prevNative) {
                $PSNativeCommandUseErrorActionPreference = $prevNative
            }
            if ($pipExit -ne 0) {
                Write-Warn "Failed to install $playwrightSpec. Run '$pyCmd -m pip install $playwrightSpec' manually."
                Write-Host ($pipOut | Out-String) -ForegroundColor DarkGray
            } else {
                Write-Ok "playwright installed via $pyCmd."
            }
        } else {
            Write-Warn "Python not found. Install Python 3.11+ from https://python.org"
            Write-Host "  Then run: python -m pip install playwright" -ForegroundColor Yellow
        }

        # ── Done ──────────────────────────────────────────────────────────────
        Write-Host ""
        Write-Ok "Installed tossctl to $destBin"
        Write-Host ""
        Write-Host "NOTE: Open a new terminal window so PATH changes take effect." -ForegroundColor Yellow
        Write-Host ""
        Write-Host "Next steps:"
        Write-Host "  tossctl doctor"
        Write-Host "  tossctl auth login"

    } finally {
        Remove-Item $tmpDir -Recurse -Force -ErrorAction SilentlyContinue
    }
}

Install-Tossctl
