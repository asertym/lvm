#!/bin/sh
# lvm installer — macOS and Linux
# Usage: curl -sSL https://github.com/YOURNAME/lvm/releases/latest/download/install.sh | sh

set -e

REPO="YOURNAME/lvm"
BINARY="lvm"
INSTALL_DIR="/usr/local/bin"

# ── colors ────────────────────────────────────────────────────────────────────
red()   { printf '\033[31m%s\033[0m\n' "$1"; }
green() { printf '\033[32m%s\033[0m\n' "$1"; }
yellow(){ printf '\033[33m%s\033[0m\n' "$1"; }
bold()  { printf '\033[1m%s\033[0m\n'  "$1"; }

# ── detect platform ───────────────────────────────────────────────────────────
detect_platform() {
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')
  ARCH=$(uname -m)

  case "$OS" in
    linux)  OS="linux"  ;;
    darwin) OS="macos"  ;;
    *)
      red "Unsupported OS: $OS"
      exit 1
      ;;
  esac

  case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    arm64|aarch64) ARCH="arm64" ;;
    *)
      red "Unsupported architecture: $ARCH"
      exit 1
      ;;
  esac

  ASSET="${BINARY}-${OS}-${ARCH}"
}

# ── fetch latest release tag from GitHub API ──────────────────────────────────
fetch_latest_version() {
  if command -v curl >/dev/null 2>&1; then
    VERSION=$(curl -sSfL "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  elif command -v wget >/dev/null 2>&1; then
    VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
      | grep '"tag_name"' \
      | sed -E 's/.*"tag_name": *"([^"]+)".*/\1/')
  else
    red "curl or wget is required"
    exit 1
  fi

  if [ -z "$VERSION" ]; then
    red "Could not fetch latest release version"
    exit 1
  fi
}

# ── download binary ───────────────────────────────────────────────────────────
download_binary() {
  URL="https://github.com/${REPO}/releases/download/${VERSION}/${ASSET}"
  TMP=$(mktemp)

  printf "Downloading %s %s (%s/%s)...\n" "$BINARY" "$VERSION" "$OS" "$ARCH"

  if command -v curl >/dev/null 2>&1; then
    curl -sSfL "$URL" -o "$TMP" || {
      red "Download failed: $URL"
      rm -f "$TMP"
      exit 1
    }
  else
    wget -qO "$TMP" "$URL" || {
      red "Download failed: $URL"
      rm -f "$TMP"
      exit 1
    }
  fi

  chmod +x "$TMP"
  echo "$TMP"
}

# ── install binary ────────────────────────────────────────────────────────────
install_binary() {
  TMP_BIN="$1"

  # Try /usr/local/bin first; fall back to ~/bin if no write permission.
  if [ -w "$INSTALL_DIR" ]; then
    DEST="${INSTALL_DIR}/${BINARY}"
    mv "$TMP_BIN" "$DEST"
  elif [ "$(id -u)" -eq 0 ]; then
    DEST="${INSTALL_DIR}/${BINARY}"
    mv "$TMP_BIN" "$DEST"
  else
    printf "\n"
    yellow "No write permission to $INSTALL_DIR."
    printf "Trying with sudo...\n\n"
    DEST="${INSTALL_DIR}/${BINARY}"
    sudo mv "$TMP_BIN" "$DEST"
    sudo chmod +x "$DEST"
  fi

  green "✓ Installed $BINARY to $DEST"
}

# ── add to PATH ───────────────────────────────────────────────────────────────
add_to_path() {
  # Detect shell rc file.
  SHELL_NAME=$(basename "${SHELL:-bash}")
  case "$SHELL_NAME" in
    zsh)  RC="$HOME/.zshrc" ;;
    fish) RC="$HOME/.config/fish/config.fish" ;;
    *)    RC="$HOME/.bashrc" ;;
  esac

  # /usr/local/bin is usually already on PATH — only add if not.
  case ":$PATH:" in
    *":$INSTALL_DIR:"*)
      green "✓ $INSTALL_DIR already in PATH"
      return
      ;;
  esac

  LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

  if [ -f "$RC" ] && grep -q "$INSTALL_DIR" "$RC" 2>/dev/null; then
    green "✓ PATH already configured in $RC"
  else
    printf "\n# lvm\n%s\n" "$LINE" >> "$RC"
    green "✓ Added $INSTALL_DIR to PATH in $RC"
    yellow "  Reload with: source $RC"
  fi
}

# ── main ──────────────────────────────────────────────────────────────────────
main() {
  printf "\n"
  bold "lvm — llama.cpp version manager"
  printf "\n"

  detect_platform
  fetch_latest_version
  TMP_BIN=$(download_binary)
  install_binary "$TMP_BIN"
  add_to_path

  printf "\n"
  bold "Done. Run:"
  printf "  lvm init\n"
  printf "  lvm install latest\n\n"
}

main
