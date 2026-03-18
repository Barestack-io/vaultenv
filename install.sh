#!/bin/sh
set -e

REPO="Barestack-io/vaultenv"
BINARY="vaultenv"
GLOBAL_DIR="/usr/local/bin"
USER_DIR="${HOME}/.local/bin"

main() {
    parse_args "$@"
    detect_platform

    if [ -z "$INSTALL_DIR" ]; then
        prompt_install_type
    fi

    download_binary
    verify_install
}

parse_args() {
    INSTALL_DIR=""
    for arg in "$@"; do
        case "$arg" in
            --global)
                INSTALL_DIR="$GLOBAL_DIR"
                ;;
            --user)
                INSTALL_DIR="$USER_DIR"
                ;;
            --help|-h)
                echo "Usage: install.sh [--global | --user]"
                echo ""
                echo "Install the latest vaultenv binary."
                echo ""
                echo "Options:"
                echo "  --global    Install to ${GLOBAL_DIR} (requires sudo, available system-wide)"
                echo "  --user      Install to ${USER_DIR} (no sudo, current user only)"
                echo ""
                echo "If no option is given, you will be prompted to choose."
                exit 0
                ;;
        esac
    done
}

prompt_install_type() {
    # When piped through sh, stdin is the script itself -- reopen the tty
    if [ ! -t 0 ]; then
        exec < /dev/tty
    fi

    echo ""
    echo "Where would you like to install vaultenv?"
    echo ""
    echo "  1) System-wide  (${GLOBAL_DIR}) — requires sudo, available to all users"
    echo "  2) Current user  (${USER_DIR}) — no sudo required"
    echo ""
    printf "Choose [1/2] (default: 1): "
    read -r choice

    case "$choice" in
        2)
            INSTALL_DIR="$USER_DIR"
            ;;
        *)
            INSTALL_DIR="$GLOBAL_DIR"
            ;;
    esac
}

detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case "$OS" in
        linux)  OS="linux" ;;
        darwin) OS="darwin" ;;
        *)
            echo "Error: Unsupported operating system: $OS"
            echo "Download manually from https://github.com/${REPO}/releases/latest"
            exit 1
            ;;
    esac

    case "$ARCH" in
        x86_64|amd64)   ARCH="amd64" ;;
        aarch64|arm64)  ARCH="arm64" ;;
        *)
            echo "Error: Unsupported architecture: $ARCH"
            echo "Download manually from https://github.com/${REPO}/releases/latest"
            exit 1
            ;;
    esac

    echo "Detected platform: ${OS}/${ARCH}"
}

download_binary() {
    URL="https://github.com/${REPO}/releases/latest/download/${BINARY}-${OS}-${ARCH}"
    TMPFILE=$(mktemp)

    echo "Downloading ${BINARY} from ${URL}..."

    if command -v curl >/dev/null 2>&1; then
        HTTP_CODE=$(curl -fsSL -w "%{http_code}" -o "$TMPFILE" "$URL")
        if [ "$HTTP_CODE" != "200" ] && [ "$HTTP_CODE" != "302" ] && [ "$HTTP_CODE" != "000" ]; then
            echo "Error: Download failed (HTTP ${HTTP_CODE})"
            rm -f "$TMPFILE"
            exit 1
        fi
    elif command -v wget >/dev/null 2>&1; then
        wget -q -O "$TMPFILE" "$URL"
    else
        echo "Error: curl or wget is required"
        rm -f "$TMPFILE"
        exit 1
    fi

    chmod +x "$TMPFILE"

    if [ "$INSTALL_DIR" = "$GLOBAL_DIR" ]; then
        if [ -w "$INSTALL_DIR" ]; then
            mkdir -p "$INSTALL_DIR"
            mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
        else
            echo "Installing to ${INSTALL_DIR} (sudo required)..."
            sudo mkdir -p "$INSTALL_DIR"
            sudo mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
            sudo chmod +x "${INSTALL_DIR}/${BINARY}"
        fi
    else
        mkdir -p "$INSTALL_DIR"
        mv "$TMPFILE" "${INSTALL_DIR}/${BINARY}"
    fi

    echo "Installed ${BINARY} to ${INSTALL_DIR}/${BINARY}"
}

verify_install() {
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*)
            ;;
        *)
            echo ""
            echo "NOTE: ${INSTALL_DIR} is not in your PATH."
            SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
            case "$SHELL_NAME" in
                zsh)  RC_FILE="~/.zshrc" ;;
                bash) RC_FILE="~/.bashrc" ;;
                *)    RC_FILE="~/.profile" ;;
            esac
            echo "Add it by running:"
            echo ""
            echo "  echo 'export PATH=\"${INSTALL_DIR}:\$PATH\"' >> ${RC_FILE} && source ${RC_FILE}"
            echo ""
            ;;
    esac

    if command -v "$BINARY" >/dev/null 2>&1; then
        VERSION=$("$BINARY" --help 2>&1 | head -n1)
        echo ""
        echo "vaultenv installed successfully."
        echo "  ${VERSION}"
    elif [ -x "${INSTALL_DIR}/${BINARY}" ]; then
        VERSION=$("${INSTALL_DIR}/${BINARY}" --help 2>&1 | head -n1)
        echo ""
        echo "vaultenv installed successfully."
        echo "  ${VERSION}"
    fi

    echo ""
    echo "Get started: vaultenv login"
}

main "$@"
