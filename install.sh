#!/bin/bash
# Liza installation script
# Downloads and installs the latest release of liza

set -e

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Configuration
REPO="liza-mas/liza"
BINARY_NAME="liza"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Detect OS and architecture
detect_platform() {
    local os
    local arch

    # Detect OS
    case "$(uname -s)" in
        Linux*)     os="linux";;
        Darwin*)    os="darwin";;
        MINGW*|MSYS*|CYGWIN*)
            echo -e "${RED}Error: Windows installation via this script is not supported.${NC}"
            echo "Please download the binary manually from:"
            echo "https://github.com/${REPO}/releases"
            exit 1
            ;;
        *)
            echo -e "${RED}Error: Unsupported operating system: $(uname -s)${NC}"
            exit 1
            ;;
    esac

    # Detect architecture
    case "$(uname -m)" in
        x86_64|amd64)   arch="amd64";;
        arm64|aarch64)  arch="arm64";;
        *)
            echo -e "${RED}Error: Unsupported architecture: $(uname -m)${NC}"
            exit 1
            ;;
    esac

    echo "${os}-${arch}"
}

# Get the latest release version
get_latest_version() {
    local version
    version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

    if [ -z "$version" ]; then
        echo -e "${RED}Error: Could not determine latest version${NC}" >&2
        return 1
    fi

    echo "$version"
}

# Download and install
install_liza() {
    local platform=$1
    local version=$2
    local tmp_dir
    local version_bare="${version#v}"

    echo -e "${GREEN}Installing liza...${NC}"
    echo "  Version: ${version}"
    echo "  Platform: ${platform}"
    echo "  Install directory: ${INSTALL_DIR}"
    echo ""

    # Create temporary directory
    tmp_dir=$(mktemp -d)
    trap "rm -rf ${tmp_dir}" EXIT

    # Download archive (goreleaser produces tar.gz for linux/darwin)
    local archive_name="${BINARY_NAME}-${version_bare}-${platform}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${version}/${archive_name}"
    echo "Downloading from ${download_url}..."

    if ! curl -fsSL "${download_url}" -o "${tmp_dir}/${archive_name}"; then
        echo -e "${RED}Error: Failed to download archive${NC}"
        echo "URL: ${download_url}"
        exit 1
    fi

    # Extract
    echo "Extracting..."
    tar -xzf "${tmp_dir}/${archive_name}" -C "${tmp_dir}"

    # Make executable
    chmod +x "${tmp_dir}/${BINARY_NAME}"
    [ -f "${tmp_dir}/liza-mcp" ] && chmod +x "${tmp_dir}/liza-mcp"

    # Verify the binary works
    echo "Verifying binary..."
    if ! "${tmp_dir}/${BINARY_NAME}" version >/dev/null 2>&1; then
        echo -e "${YELLOW}Warning: Could not verify binary${NC}"
    fi

    # Install
    echo "Installing to ${INSTALL_DIR}..."

    # Create install directory if it doesn't exist
    if [ ! -d "$INSTALL_DIR" ]; then
        echo "Creating ${INSTALL_DIR}..."
        mkdir -p "$INSTALL_DIR" 2>/dev/null || sudo mkdir -p "$INSTALL_DIR"
    fi

    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        [ -f "${tmp_dir}/liza-mcp" ] && mv "${tmp_dir}/liza-mcp" "${INSTALL_DIR}/liza-mcp"
    else
        echo "Note: Sudo access required to install to ${INSTALL_DIR}"
        sudo mv "${tmp_dir}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
        [ -f "${tmp_dir}/liza-mcp" ] && sudo mv "${tmp_dir}/liza-mcp" "${INSTALL_DIR}/liza-mcp"
    fi

    echo -e "${GREEN}✓ Installation complete!${NC}"
    echo ""
    echo "Run 'liza version' to verify installation"
    echo "Run 'liza help' to get started"
}

# Main
main() {
    echo ""
    echo "Liza Installer"
    echo "=============="
    echo ""

    # Check dependencies
    if ! command -v curl >/dev/null 2>&1; then
        echo -e "${RED}Error: curl is required but not installed${NC}"
        exit 1
    fi

    # Detect platform
    local platform
    platform=$(detect_platform)

    # Get latest version (or use VERSION env var if set)
    local version="${VERSION:-}"
    if [ -z "$version" ]; then
        version=$(get_latest_version) || exit 1
    fi

    # Install
    install_liza "$platform" "$version"
}

# Show help
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    echo "Liza Installation Script"
    echo ""
    echo "Usage: $0 [OPTIONS]"
    echo ""
    echo "Options:"
    echo "  -h, --help          Show this help message"
    echo ""
    echo "Environment variables:"
    echo "  VERSION             Specific version to install (default: latest)"
    echo "  INSTALL_DIR         Installation directory (default: /usr/local/bin)"
    echo ""
    echo "Examples:"
    echo "  # Install latest version"
    echo "  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | bash"
    echo ""
    echo "  # Install specific version"
    echo "  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | VERSION=v1.0.0 bash"
    echo ""
    echo "  # Install to custom directory"
    echo "  curl -fsSL https://raw.githubusercontent.com/${REPO}/main/install.sh | INSTALL_DIR=~/.local/bin bash"
    exit 0
fi

main
