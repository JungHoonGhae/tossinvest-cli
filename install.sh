#!/bin/sh
set -e

REPO="JungHoonGhae/tossinvest-cli"
BINARY="tossctl"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"
SHARE_DIR="${SHARE_DIR:-/usr/local/share/tossctl}"

main() {
    os=$(detect_os)
    arch=$(detect_arch)

    if [ "$os" = "windows" ]; then
        echo "Error: this script does not support Windows. Use PowerShell instead:"
        echo '  Invoke-WebRequest -Uri "https://github.com/'"$REPO"'/releases/latest/download/tossctl-windows-amd64.zip" -OutFile tossctl.zip'
        echo '  Expand-Archive tossctl.zip -DestinationPath .'
        exit 1
    fi

    asset="${BINARY}-${os}-${arch}.tar.gz"
    # TOSSCTL_VERSION pins the payload to the release tag the updater resolved
    # (e.g. v0.54.1). Manual runs default to latest. Never points at mutable
    # branch content: only the versioned release assets below are executed.
    TAG="${TOSSCTL_VERSION:-latest}"
    if [ "$TAG" = "latest" ]; then
        base="https://github.com/${REPO}/releases/latest/download"
    else
        base="https://github.com/${REPO}/releases/download/${TAG}"
    fi
    url="${base}/${asset}"
    sha_url="${url}.sha256"

    echo "Detected: ${os}/${arch}"
    echo "Downloading ${asset}..."

    tmpdir=$(mktemp -d)
    trap 'rm -rf "$tmpdir"' EXIT

    curl -fsSL -o "${tmpdir}/${asset}" "$url"
    curl -fsSL -o "${tmpdir}/${asset}.sha256" "$sha_url"

    echo "Verifying checksum..."
    (cd "$tmpdir" && verify_checksum "${asset}")

    echo "Extracting..."
    tar -xzf "${tmpdir}/${asset}" -C "$tmpdir"

    echo "Installing to ${INSTALL_DIR}..."

    # Use sudo only when we can't write the target ourselves. On a fresh
    # Apple Silicon Mac /usr/local/bin frequently doesn't exist yet (Homebrew
    # lives in /opt/homebrew), so we must create it before moving the binary —
    # otherwise mv fails with "No such file or directory" on the destination.
    if can_write "$INSTALL_DIR" && can_write "$SHARE_DIR"; then
        SUDO=""
    else
        SUDO="sudo"
        echo "Elevated permissions required to write ${INSTALL_DIR}."
    fi

    $SUDO mkdir -p "${INSTALL_DIR}" "${SHARE_DIR}"
    $SUDO mv "${tmpdir}/${BINARY}" "${INSTALL_DIR}/${BINARY}"
    $SUDO chmod +x "${INSTALL_DIR}/${BINARY}"
    if [ -d "${tmpdir}/auth-helper" ]; then
        # Atomic swap: the old helper stays in place until the new one lands.
        if [ -d "${SHARE_DIR}/auth-helper" ]; then
            $SUDO mv "${SHARE_DIR}/auth-helper" "${SHARE_DIR}/auth-helper.bak" || exit 1
            if $SUDO mv "${tmpdir}/auth-helper" "${SHARE_DIR}/auth-helper"; then
                $SUDO rm -rf "${SHARE_DIR}/auth-helper.bak"
            else
                $SUDO mv "${SHARE_DIR}/auth-helper.bak" "${SHARE_DIR}/auth-helper"
                echo "Error: failed to install auth-helper, restored previous version" >&2
                exit 1
            fi
        else
            $SUDO mv "${tmpdir}/auth-helper" "${SHARE_DIR}/auth-helper"
        fi
    fi

    echo ""
    echo "Installing Python dependencies for auth-helper..."
    # Pinned to the range auth-helper declares (pyproject.toml): an unpinned
    # `pip install playwright` would float the browser-automation dependency
    # of a tool that handles live-trading credentials.
    PLAYWRIGHT_SPEC="playwright>=1.52,<2"
    if command -v python3 >/dev/null 2>&1; then
        if ! python3 -m pip install --quiet "${PLAYWRIGHT_SPEC}"; then
            echo "Warning: failed to install ${PLAYWRIGHT_SPEC}. Run 'python3 -m pip install \"${PLAYWRIGHT_SPEC}\"' manually." >&2
        fi
    else
        echo "Warning: python3 not found. Install Python 3.11+ and run 'python3 -m pip install \"${PLAYWRIGHT_SPEC}\"'." >&2
    fi

    echo ""
    echo "Installed $(${INSTALL_DIR}/${BINARY} version 2>/dev/null || echo "${BINARY}") to ${INSTALL_DIR}/${BINARY}"
    echo "Auth helper installed to ${SHARE_DIR}/auth-helper"
    echo ""
    echo "Next steps:"
    echo "  tossctl doctor"
    echo "  tossctl auth login"
}

# True if we can create/write inside dir. When dir doesn't exist yet, walk up
# to the first existing ancestor and test that (mkdir -p will create the rest).
can_write() {
    d="$1"
    while [ ! -e "$d" ]; do
        d=$(dirname "$d")
    done
    [ -w "$d" ]
}

detect_os() {
    case "$(uname -s)" in
        Darwin*)  echo "darwin" ;;
        Linux*)   echo "linux" ;;
        MINGW*|MSYS*|CYGWIN*) echo "windows" ;;
        *)        echo "Error: unsupported OS: $(uname -s)" >&2; exit 1 ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   echo "amd64" ;;
        arm64|aarch64)  echo "arm64" ;;
        *)              echo "Error: unsupported architecture: $(uname -m)" >&2; exit 1 ;;
    esac
}

verify_checksum() {
    file="$1"
    expected=$(awk '{print $1}' "${file}.sha256")
    case "$expected" in
        *[!0-9a-fA-F]*|"")
            echo "Error: checksum file is malformed, refusing to install" >&2
            exit 1
            ;;
    esac
    if [ "${#expected}" -ne 64 ]; then
        echo "Error: checksum file is malformed, refusing to install" >&2
        exit 1
    fi
    if command -v sha256sum >/dev/null 2>&1; then
        actual=$(sha256sum "$file" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        actual=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        # Fail closed: installing an unverified binary that manages live
        # trading credentials is worse than refusing to install.
        echo "Error: no sha256 tool found (sha256sum/shasum), refusing to install without checksum verification" >&2
        exit 1
    fi
    if [ "$expected" != "$actual" ]; then
        echo "Error: checksum mismatch" >&2
        echo "  expected: $expected" >&2
        echo "  actual:   $actual" >&2
        exit 1
    fi
}

main
