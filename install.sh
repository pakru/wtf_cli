#!/bin/bash
#
# WTF CLI Install Script
# Usage: curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash
#
# Environment variables:
#   WTF_INSTALL_DIR - Custom installation directory (default: ~/.local/bin on Linux, /usr/local/bin on macOS)
#

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

REPO="pakru/wtf_cli"
BINARY_NAME="wtf_cli"

# Print colored messages
info() { echo -e "${BLUE}[INFO]${NC} $1"; }
success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
warn() { echo -e "${YELLOW}[WARN]${NC} $1"; }
error() { echo -e "${RED}[ERROR]${NC} $1" >&2; }

# Show help
show_help() {
    cat << EOF
WTF CLI Installer

Usage: install.sh [OPTIONS]

Options:
    -h, --help      Show this help message
    -d, --dir DIR   Install to custom directory (default: ~/.local/bin on Linux, /usr/local/bin on macOS)

Environment Variables:
    WTF_INSTALL_DIR     Custom installation directory

Examples:
    # Install to default location
    curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash

    # Install to custom directory
    curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash -s -- -d ~/bin

EOF
    exit 0
}

# Detect OS
detect_os() {
    local os
    os="$(uname -s)"
    case "$os" in
        Linux*)  echo "linux" ;;
        Darwin*) echo "darwin" ;;
        *)       error "Unsupported operating system: $os"; exit 1 ;;
    esac
}

# Detect architecture
detect_arch() {
    local arch
    arch="$(uname -m)"
    case "$arch" in
        x86_64)  echo "amd64" ;;
        amd64)   echo "amd64" ;;
        aarch64) echo "arm64" ;;
        arm64)   echo "arm64" ;;
        *)       error "Unsupported architecture: $arch"; exit 1 ;;
    esac
}

# Get default install directory based on OS
get_default_install_dir() {
    local os="$1"
    if [ "$os" = "darwin" ]; then
        echo "/usr/local/bin"
    else
        echo "$HOME/.local/bin"
    fi
}

# Check if command exists
command_exists() {
    command -v "$1" >/dev/null 2>&1
}

# Download file using curl or wget
download() {
    local url="$1"
    local output="$2"
    
    if command_exists curl; then
        curl -fsSL "$url" -o "$output"
    elif command_exists wget; then
        wget -q "$url" -O "$output"
    else
        error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
}

# Get latest release version from GitHub API
get_latest_version() {
    local api_url="https://api.github.com/repos/${REPO}/releases/latest"
    local version
    
    if command_exists curl; then
        version=$(curl -fsSL "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    elif command_exists wget; then
        version=$(wget -qO- "$api_url" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    else
        error "Neither curl nor wget found. Please install one of them."
        exit 1
    fi
    
    if [ -z "$version" ]; then
        error "Could not determine latest version"
        exit 1
    fi
    
    echo "$version"
}

# Get installed version if exists
get_installed_version() {
    local install_path="$1"
    if [ -x "$install_path" ]; then
        # Extract version number and add 'v' prefix to match GitHub tag format
        local ver
        ver=$("$install_path" --version 2>/dev/null | head -n1 | grep -oE '[0-9]+\.[0-9]+\.[0-9]+' || echo "")
        if [ -n "$ver" ]; then
            echo "v${ver}"
        fi
    else
        echo ""
    fi
}

# Verify checksum
verify_checksum() {
    local file="$1"
    local checksums_file="$2"
    local filename
    filename=$(basename "$file")
    
    local expected_checksum
    expected_checksum=$(grep "$filename" "$checksums_file" | awk '{print $1}')
    
    if [ -z "$expected_checksum" ]; then
        error "Could not find checksum for $filename"
        return 1
    fi
    
    local actual_checksum
    if command_exists sha256sum; then
        actual_checksum=$(sha256sum "$file" | awk '{print $1}')
    elif command_exists shasum; then
        actual_checksum=$(shasum -a 256 "$file" | awk '{print $1}')
    else
        warn "No sha256sum or shasum found, skipping checksum verification"
        return 0
    fi
    
    if [ "$expected_checksum" != "$actual_checksum" ]; then
        error "Checksum verification failed!"
        error "Expected: $expected_checksum"
        error "Actual:   $actual_checksum"
        return 1
    fi
    
    return 0
}

# Main installation function
main() {
    # Parse arguments
    local custom_dir=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            -h|--help)
                show_help
                ;;
            -d|--dir)
                custom_dir="$2"
                shift 2
                ;;
            *)
                error "Unknown option: $1"
                show_help
                ;;
        esac
    done

    info "Installing WTF CLI..."
    echo

    # Detect platform
    local os arch
    os=$(detect_os)
    arch=$(detect_arch)
    info "Detected platform: ${os}/${arch}"

    # Validate supported platform
    if [ "$os" = "darwin" ] && [ "$arch" = "amd64" ]; then
        error "macOS on Intel (amd64) is not supported. Only Apple Silicon (arm64) is supported."
        exit 1
    fi

    # Determine install directory
    local install_dir
    install_dir="${custom_dir:-${WTF_INSTALL_DIR:-$(get_default_install_dir "$os")}}"
    local install_path="${install_dir}/${BINARY_NAME}"
    info "Install location: ${install_path}"

    # Check for existing installation
    local installed_version
    installed_version=$(get_installed_version "$install_path")
    if [ -n "$installed_version" ]; then
        info "Existing installation found: ${installed_version}"
    fi

    # Get latest version
    info "Fetching latest release..."
    local latest_version
    latest_version=$(get_latest_version)
    info "Latest version: ${latest_version}"

    # Check if already up to date
    if [ -n "$installed_version" ] && [ "$installed_version" = "$latest_version" ]; then
        success "Already up to date (${latest_version})"
        exit 0
    fi

    # Create temp directory
    local tmp_dir
    tmp_dir=$(mktemp -d)
    trap 'rm -rf "$tmp_dir"' EXIT

    # Build download URLs
    local version_number="${latest_version#v}"  # Remove 'v' prefix
    local archive_name="${BINARY_NAME}_${version_number}_${os}_${arch}.tar.gz"
    local download_url="https://github.com/${REPO}/releases/download/${latest_version}/${archive_name}"
    local checksums_url="https://github.com/${REPO}/releases/download/${latest_version}/checksums.txt"

    # Download archive
    info "Downloading ${archive_name}..."
    download "$download_url" "${tmp_dir}/${archive_name}"

    # Download and verify checksum
    info "Verifying checksum..."
    download "$checksums_url" "${tmp_dir}/checksums.txt"
    if ! verify_checksum "${tmp_dir}/${archive_name}" "${tmp_dir}/checksums.txt"; then
        exit 1
    fi
    success "Checksum verified"

    # Extract archive
    info "Extracting..."
    tar -xzf "${tmp_dir}/${archive_name}" -C "$tmp_dir"

    # Create install directory if needed
    if [ ! -d "$install_dir" ]; then
        info "Creating directory: ${install_dir}"
        mkdir -p "$install_dir" 2>/dev/null || sudo mkdir -p "$install_dir"
    fi

    # Install binary
    info "Installing binary..."
    if [ -w "$install_dir" ]; then
        cp "${tmp_dir}/${BINARY_NAME}" "$install_path"
        chmod +x "$install_path"
    else
        info "Elevated permissions required for ${install_dir}"
        sudo cp "${tmp_dir}/${BINARY_NAME}" "$install_path"
        sudo chmod +x "$install_path"
    fi

    echo
    if [ -n "$installed_version" ]; then
        success "Upgraded ${installed_version} → ${latest_version}"
    else
        success "WTF CLI ${latest_version} installed successfully!"
    fi

    # Check if install directory is in PATH
    if ! echo "$PATH" | tr ':' '\n' | grep -qx "$install_dir"; then
        echo
        warn "NOTE: ${install_dir} is not in your PATH"
        echo
        echo "Add it to your shell profile:"
        echo
        echo "  # For bash (~/.bashrc or ~/.bash_profile)"
        echo "  export PATH=\"\$PATH:${install_dir}\""
        echo
        echo "  # For zsh (~/.zshrc)"
        echo "  export PATH=\"\$PATH:${install_dir}\""
        echo
        echo "Then restart your terminal or run: source ~/.bashrc (or ~/.zshrc)"
    fi

    echo
    info "Run 'wtf_cli --version' to verify the installation"
}

main "$@"
