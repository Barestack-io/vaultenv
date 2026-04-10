#!/bin/sh
#
# Install the latest vaultenv release binary for this OS/arch.
# One-liner: curl -fsSL …/install.sh | sh
#
# When this file is read from stdin (pipe from curl), $0 is the shell name. We
# copy the rest of the script off the pipe, then re-exec from disk with stdin
# on /dev/tty so: (1) curl exits and the pipeline cannot hang, (2) prompts work.
# When you run sh /path/install.sh, $0 is the path — we never cat stdin, so CI
# and non-tty stdin do not break.
#
_ve_base=$(basename "$0")
case "$_ve_base" in
sh | dash | bash | ksh | zsh | ash | -) _ve_from_stdin=true ;;
*) _ve_from_stdin=false ;;
esac
unset _ve_base

if [ "$_ve_from_stdin" = true ] && [ ! -t 0 ]; then
	_ve_tmp=$(mktemp "${TMPDIR:-/tmp}/vaultenv-install.XXXXXX")
	export VAULTENV_INSTALLER_TMP="$_ve_tmp"
	cat > "$_ve_tmp"
	# Prefer a real TTY for prompts; probe quietly (some environments have no tty).
	if sh -c 'exec 0</dev/tty' 2>/dev/null; then
		exec sh "$_ve_tmp" "$@" 0</dev/tty
	fi
	exec sh "$_ve_tmp" "$@"
fi
unset _ve_from_stdin

set -e

REPO="Barestack-io/vaultenv"
BINARY="vaultenv"
GLOBAL_DIR="/usr/local/bin"
USER_DIR="${HOME}/.local/bin"

trap 'rm -f "${VAULTENV_INSTALLER_TMP:-}"' EXIT

usage() {
	echo "Usage: $0 [--global | --user]"
	echo "  --global   Install to $GLOBAL_DIR (sudo if needed)"
	echo "  --user     Install to $USER_DIR (no sudo)"
	echo "  (no flag)  Prompt for 1) global or 2) user"
	exit "${1:-0}"
}

INSTALL_DIR=""
while [ "$#" -gt 0 ]; do
	case "$1" in
	--global) INSTALL_DIR=$GLOBAL_DIR ;;
	--user) INSTALL_DIR=$USER_DIR ;;
	--help | -h) usage 0 ;;
	*)
		echo "Unknown option: $1" >&2
		usage 1
		;;
	esac
	shift
done

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$OS" in
linux) ;;
darwin) ;;
*)
	echo "Unsupported OS: $OS — get a binary from https://github.com/$REPO/releases/latest" >&2
	exit 1
	;;
esac

case "$ARCH" in
x86_64 | amd64) ARCH=amd64 ;;
aarch64 | arm64) ARCH=arm64 ;;
*)
	echo "Unsupported CPU: $ARCH — get a binary from https://github.com/$REPO/releases/latest" >&2
	exit 1
	;;
esac

echo "Detected platform: $OS/$ARCH"

if [ -z "$INSTALL_DIR" ]; then
	echo ""
	echo "Install vaultenv:"
	echo "  1) System-wide  ($GLOBAL_DIR) — may ask for sudo"
	echo "  2) Your user only ($USER_DIR) — no sudo"
	printf "Choose [1/2] (default 1): "
	read -r _ve_choice || true
	case "$_ve_choice" in
	2) INSTALL_DIR=$USER_DIR ;;
	*) INSTALL_DIR=$GLOBAL_DIR ;;
	esac
fi

URL="https://github.com/$REPO/releases/latest/download/$BINARY-$OS-$ARCH"
TMPBIN=$(mktemp "${TMPDIR:-/tmp}/vaultenv-bin.XXXXXX")

echo "Downloading $URL ..."
if command -v curl >/dev/null 2>&1; then
	curl -fsSL -o "$TMPBIN" "$URL"
elif command -v wget >/dev/null 2>&1; then
	wget -q -O "$TMPBIN" "$URL"
else
	echo "Need curl or wget." >&2
	rm -f "$TMPBIN"
	exit 1
fi

chmod +x "$TMPBIN"

if [ "$INSTALL_DIR" = "$GLOBAL_DIR" ]; then
	if [ -w "$GLOBAL_DIR" ] 2>/dev/null; then
		mkdir -p "$GLOBAL_DIR"
		mv "$TMPBIN" "$GLOBAL_DIR/$BINARY"
	else
		echo "Installing to $GLOBAL_DIR (sudo) ..."
		sudo mkdir -p "$GLOBAL_DIR"
		sudo mv "$TMPBIN" "$GLOBAL_DIR/$BINARY"
		sudo chmod +x "$GLOBAL_DIR/$BINARY"
	fi
else
	mkdir -p "$USER_DIR"
	mv "$TMPBIN" "$USER_DIR/$BINARY"
fi

echo "Installed $INSTALL_DIR/$BINARY"

case ":$PATH:" in
*":$INSTALL_DIR:"*) ;;
*)
	echo ""
	echo "NOTE: $INSTALL_DIR is not on PATH."
	_rc=~/.profile
	case "$(basename "${SHELL:-sh}")" in
	zsh) _rc=~/.zshrc ;;
	bash) _rc=~/.bashrc ;;
	esac
	echo "  echo 'export PATH=\"$INSTALL_DIR:\$PATH\"' >> $_rc"
	;;
esac

if command -v "$BINARY" >/dev/null 2>&1; then
	_ver=$(VAULTENV_NO_UPDATE_CHECK=1 "$BINARY" version </dev/null 2>&1 | head -n1)
elif [ -x "$INSTALL_DIR/$BINARY" ]; then
	_ver=$(VAULTENV_NO_UPDATE_CHECK=1 "$INSTALL_DIR/$BINARY" version </dev/null 2>&1 | head -n1)
else
	_ver=""
fi

echo ""
echo "vaultenv installed successfully."
[ -n "$_ver" ] && echo "  $_ver"
echo ""
echo "Get started: vaultenv login"
echo ""
