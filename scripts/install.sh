#!/bin/sh
set -e

# Setup colors
if [ -t 1 ]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  BLUE='\033[0;34m'
  NC='\033[0m' # No Color
else
  RED=''
  GREEN=''
  BLUE=''
  NC=''
fi

echo "${BLUE}Installing LLM Launcher (llml)...${NC}"

# Detect OS
OS_RAW=$(uname -s)
case "$OS_RAW" in
  Darwin)
    OS="Darwin"
    ;;
  Linux)
    OS="Linux"
    ;;
  *)
    echo "${RED}Error: Unsupported operating system: $OS_RAW${NC}" >&2
    exit 1
    ;;
esac

# Detect Architecture
ARCH_RAW=$(uname -m)
case "$ARCH_RAW" in
  x86_64|amd64)
    ARCH="x86_64"
    ;;
  arm64|aarch64)
    ARCH="arm64"
    ;;
  *)
    echo "${RED}Error: Unsupported architecture: $ARCH_RAW${NC}" >&2
    exit 1
    ;;
esac

# Resolve latest release version from Github redirects (to avoid API rate limits)
echo "Resolving latest version..."
REDIRECT_URL=$(curl -sS -I https://github.com/flyingnobita/llml/releases/latest | grep -i '^location:')
TAG=$(echo "$REDIRECT_URL" | grep -o 'tag/v[0-9.]*' | cut -d/ -f2)

if [ -z "$TAG" ]; then
  # Fallback to API if redirect lookup fails
  TAG_JSON=$(curl -sS https://api.github.com/repos/flyingnobita/llml/releases/latest)
  TAG=$(echo "$TAG_JSON" | grep -o '"tag_name": *"[^"]*"' | head -n 1 | cut -d'"' -f4)
fi

if [ -z "$TAG" ]; then
  echo "${RED}Error: Could not resolve the latest version of llml.${NC}" >&2
  exit 1
fi

VERSION=$(echo "$TAG" | sed 's/^v//')
echo "Found version: $VERSION ($OS/$ARCH)"

# Build download URL
DOWNLOAD_URL="https://github.com/flyingnobita/llml/releases/download/${TAG}/llml_${VERSION}_${OS}_${ARCH}.tar.gz"
echo "Downloading $DOWNLOAD_URL..."

# Create a secure temp directory for download/extraction
TEMP_DIR=$(mktemp -d 2>/dev/null || mktemp -d -t 'llml-install')
cleanup() {
  rm -rf "$TEMP_DIR"
}
trap cleanup EXIT

# Download and extract the archive
if ! curl -fsSL "$DOWNLOAD_URL" -o "$TEMP_DIR/llml.tar.gz"; then
  echo "${RED}Error: Failed to download the package from $DOWNLOAD_URL${NC}" >&2
  exit 1
fi

if ! tar -xzf "$TEMP_DIR/llml.tar.gz" -C "$TEMP_DIR"; then
  echo "${RED}Error: Failed to extract the package.${NC}" >&2
  exit 1
fi

# Determine destination bin directory
if [ "$(id -u)" -eq 0 ]; then
  BIN_DIR="/usr/local/bin"
else
  BIN_DIR="$HOME/.local/bin"
fi

echo "Installing binary to $BIN_DIR/llml..."
mkdir -p "$BIN_DIR"

if [ "$(id -u)" -eq 0 ]; then
  mv "$TEMP_DIR/llml" "$BIN_DIR/llml"
else
  if [ -w "$BIN_DIR" ] || [ ! -d "$BIN_DIR" ]; then
    mv "$TEMP_DIR/llml" "$BIN_DIR/llml"
  else
    echo "${RED}Error: Directory $BIN_DIR is not writable.${NC}" >&2
    exit 1
  fi
fi

chmod +x "$BIN_DIR/llml"

echo "${GREEN}LLM Launcher (llml) version $VERSION installed successfully!${NC}"

# Help the user verify PATH
if ! command -v llml >/dev/null 2>&1; then
  echo ""
  echo "${BLUE}Note: $BIN_DIR is not on your PATH.${NC}"
  echo "You may need to add it to your shell profile (e.g., ~/.zshrc or ~/.bashrc):"
  echo "  export PATH=\"\$PATH:$BIN_DIR\""
fi
