#!/bin/bash
# Test script for WTF CLI installation
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}"
    echo "=================================================="
    echo "        WTF CLI Installation Test Script"
    echo "=================================================="
    echo -e "${NC}"
}

# Create a temporary test environment
setup_test_env() {
    TEST_DIR=$(mktemp -d)
    export HOME="$TEST_DIR"
    export PATH="$TEST_DIR/.local/bin:$PATH"
    
    print_info "Created test environment: $TEST_DIR"
    print_info "Test HOME: $HOME"
    
    # Create a fake bashrc
    touch "$HOME/.bashrc"
}

# Test the installation script
test_installation() {
    print_info "Testing installation script..."
    
    # Run installation in test environment
    if $INSTALL_SCRIPT >/dev/null 2>&1; then
        print_success "Installation script completed without errors"
    else
        print_error "Installation script failed"
        return 1
    fi
    
    # Check if binary was created
    if [[ -f "$HOME/.local/bin/wtf" ]]; then
        print_success "WTF binary installed correctly"
    else
        print_error "WTF binary not found"
        return 1
    fi
    
    # Check if integration script was installed
    if [[ -f "$HOME/.wtf/integration.sh" ]]; then
        print_success "Shell integration script installed"
    else
        print_error "Shell integration script not found"
        return 1
    fi
    
    # Check if config was created
    if [[ -f "$HOME/.wtf/config.json" ]]; then
        print_success "Configuration file created"
    else
        print_error "Configuration file not found"
        return 1
    fi
    
    # Check if bashrc was modified
    if grep -q "WTF CLI Shell Integration" "$HOME/.bashrc"; then
        print_success "Shell integration added to .bashrc"
    else
        print_error "Shell integration not added to .bashrc"
        return 1
    fi
}

# Test the binary works
test_binary() {
    print_info "Testing WTF CLI binary..."
    
    # Test dry-run mode
    if WTF_DRY_RUN=true "$HOME/.local/bin/wtf" >/dev/null 2>&1; then
        print_success "WTF CLI binary works in dry-run mode"
    else
        print_error "WTF CLI binary failed in dry-run mode"
        return 1
    fi
}

# Test uninstallation
test_uninstallation() {
    print_info "Testing uninstallation..."
    
    # Run uninstallation (auto-confirm directory removal)
    if echo "y" | $INSTALL_SCRIPT uninstall >/dev/null 2>&1; then
        print_success "Uninstallation completed"
    else
        print_error "Uninstallation failed"
        return 1
    fi
    
    # Check if binary was removed
    if [[ ! -f "$HOME/.local/bin/wtf" ]]; then
        print_success "WTF binary removed"
    else
        print_error "WTF binary still exists"
        return 1
    fi
    
    # Check if directory was removed
    if [[ ! -d "$HOME/.wtf" ]]; then
        print_success "WTF directory removed"
    else
        print_error "WTF directory still exists"
        return 1
    fi
    
    # Check if bashrc was cleaned
    if ! grep -q "WTF CLI Shell Integration" "$HOME/.bashrc" 2>/dev/null; then
        print_success "Shell integration removed from .bashrc"
    else
        print_error "Shell integration still in .bashrc"
        return 1
    fi
}

# Cleanup test environment
cleanup() {
    if [[ -n "$TEST_DIR" ]] && [[ -d "$TEST_DIR" ]]; then
        rm -rf "$TEST_DIR"
        print_info "Cleaned up test environment"
    fi
}

# Main test function
main() {
    print_header
    
    # Trap cleanup on exit
    trap cleanup EXIT
    
    print_info "Starting WTF CLI installation tests"
    
    # Check if we're in the right directory
    # Handle being run from either project root or scripts directory
    if [[ -f "install.sh" ]] && [[ -f "main.go" ]]; then
        # Running from project root
        INSTALL_SCRIPT="./install.sh"
    elif [[ -f "scripts/install.sh" ]] && [[ -f "main.go" ]]; then
        # Running from project root with scripts in subdirectory
        INSTALL_SCRIPT="./scripts/install.sh"
    elif [[ -f "install.sh" ]] && [[ -f "../main.go" ]]; then
        # Running from scripts directory
        INSTALL_SCRIPT="./install.sh"
        cd ..
    else
        print_error "Please run this script from the wtf_cli project root or scripts directory"
        exit 1
    fi
    
    # Run tests
    setup_test_env
    test_installation
    test_binary
    test_uninstallation
    
    print_success "All installation tests passed! ✅"
    echo ""
    echo -e "${GREEN}The installation script is working correctly.${NC}"
    echo -e "${YELLOW}You can now safely use:${NC}"
    echo "  ./scripts/install.sh       - to install WTF CLI"
    echo "  ./scripts/install.sh uninstall - to remove WTF CLI"
    echo "  make install-full  - to install via Makefile"
    echo "  make uninstall     - to uninstall via Makefile"
}

# Run main function
main "$@"
