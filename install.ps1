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
    $zipUrl    = "https://github.com/$REPO/releases/latest/download/$ASSET.zip"
    $sha256Url = "https://github.com/$REPO/releases/latest/download/$ASSET.zip.sha256"

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
            if (Test-Path $dstHelper) {
                Remove-Item $dstHelper -Recurse -Force
            }
            Copy-Item $srcHelper $dstHelper -Recurse -Force
        }

        # ── PATH (exact-segment, null-safe) ───────────────────────────────────
        $userPath = [Environment]::GetEnvironmentVariable("PATH", "User")
        if ([string]::IsNullOrEmpty($userPath)) {
            $newPath = $INSTALL_DIR
        } else {
            $segments = $userPath -split ';' | Where-Object { $_ -ne '' }
            if ($segments -notcontains $INSTALL_DIR) {
                $newPath = ($segments + $INSTALL_DIR) -join ';'
            } else {
                $newPath = $userPath
            }
        }
        if ($newPath -ne $userPath) {
            [Environment]::SetEnvironmentVariable("PATH", $newPath, "User")
            $env:PATH = "$env:PATH;$INSTALL_DIR"
            Write-Ok "Added $INSTALL_DIR to PATH (user scope)."
        }

        # ── Google Chrome check ───────────────────────────────────────────────
        Write-Host ""
        Write-Step "Checking for Google Chrome (required for auth login)..."
        $chromePaths = @(
            (Join-Path $env:ProgramFiles       "Google\Chrome\Application\chrome.exe"),
            (Join-Path ${env:ProgramFiles(x86)} "Google\Chrome\Application\chrome.exe"),
            (Join-Path $env:LOCALAPPDATA       "Google\Chrome\Application\chrome.exe")
        )
        $chromeFound = $chromePaths | Where-Object { Test-Path $_ }
        if ($chromeFound) {
            Write-Ok "Google Chrome found."
        } else {
            Write-Warn "Google Chrome was NOT found. tossctl auth login requires Chrome."
            Write-Host "  Install from: https://www.google.com/chrome/" -ForegroundColor Yellow
        }

        # ── Python / playwright ───────────────────────────────────────────────
        Write-Host ""
        Write-Step "Installing Python dependencies for auth-helper..."
        $pyCmd = $null
        foreach ($candidate in @("python", "python3", "py")) {
            if (Get-Command $candidate -ErrorAction SilentlyContinue) {
                $pyCmd = $candidate
                break
            }
        }
        if ($pyCmd) {
            $pipOut = & $pyCmd -m pip install --quiet playwright 2>&1
            if ($LASTEXITCODE -ne 0) {
                Write-Warn "Failed to install playwright. Run '$pyCmd -m pip install playwright' manually."
                Write-Host $pipOut -ForegroundColor DarkGray
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
