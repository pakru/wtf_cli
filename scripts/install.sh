#!/bin/bash
# WTF CLI Installation Script
# This script installs the WTF CLI tool and sets up shell integration

set -e  # Exit on any error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
WTF_DIR="$HOME/.wtf"
BASHRC="$HOME/.bashrc"
INTEGRATION_LINE="source ~/.wtf/integration.sh"

# Helper functions
print_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo -e "${BLUE}"
    echo "=================================================="
    echo "           WTF CLI Installation Script"
    echo "=================================================="
    echo -e "${NC}"
}

# Check if running in the correct directory
check_directory() {
    if [[ ! -f "main.go" ]] || [[ ! -f "scripts/integration.sh" ]]; then
        print_error "Please run this script from the wtf_cli project root directory"
        print_info "Expected files: main.go, scripts/integration.sh"
        exit 1
    fi
}

# Create WTF directory
setup_wtf_directory() {
    print_info "Setting up WTF directory at $WTF_DIR"
    mkdir -p "$WTF_DIR"
    print_success "Created directory: $WTF_DIR"
}

# Copy integration script
install_integration_script() {
    print_info "Installing shell integration script"
    cp "scripts/integration.sh" "$WTF_DIR/integration.sh"
    chmod +x "$WTF_DIR/integration.sh"
    print_success "Installed: $WTF_DIR/integration.sh"
}

# Install the pre-built WTF binary
install_wtf_binary() {
    print_info "Installing WTF CLI binary"
    
    # Check if pre-built binary exists
    if [[ ! -f "build/wtf" ]]; then
        print_error "Pre-built wtf binary not found in build/ directory."
        print_info "Please build the binary first with: make build"
        exit 1
    fi
    
    # Install to user's local bin (create if doesn't exist)
    mkdir -p "$HOME/.local/bin"
    cp build/wtf "$HOME/.local/bin/"
    chmod +x "$HOME/.local/bin/wtf"
    
    print_success "Installed WTF CLI binary to ~/.local/bin/wtf"
    
    # Check if ~/.local/bin is in PATH
    if [[ ":$PATH:" != *":$HOME/.local/bin:"* ]]; then
        print_warning "~/.local/bin is not in your PATH"
        print_info "Adding ~/.local/bin to PATH in ~/.bashrc"
        echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$BASHRC"
    fi
}

# Setup shell integration
setup_shell_integration() {
    print_info "Setting up shell integration"
    
    # Check if integration is already set up
    if grep -q "$INTEGRATION_LINE" "$BASHRC" 2>/dev/null; then
        print_warning "Shell integration already exists in ~/.bashrc"
        return 0
    fi
    
    # Backup bashrc
    if [[ -f "$BASHRC" ]]; then
        cp "$BASHRC" "$BASHRC.wtf-backup.$(date +%Y%m%d-%H%M%S)"
        print_info "Created backup: $BASHRC.wtf-backup.*"
    fi
    
    # Add integration to bashrc
    echo "" >> "$BASHRC"
    echo "# WTF CLI Shell Integration" >> "$BASHRC"
    echo "if [[ -f ~/.wtf/integration.sh ]]; then" >> "$BASHRC"
    echo "    $INTEGRATION_LINE" >> "$BASHRC"
    echo "fi" >> "$BASHRC"
    
    print_success "Added shell integration to ~/.bashrc"
}

# Create default configuration
create_default_config() {
    local config_file="$WTF_DIR/config.json"
    
    if [[ -f "$config_file" ]]; then
        print_warning "Configuration file already exists: $config_file"
        return 0
    fi
    
    print_info "Creating default configuration"
    
    cat > "$config_file" << 'EOF'
{
  "llm_provider": "openrouter",
  "openrouter": {
    "api_key": "your_openrouter_api_key_here",
    "model": "openai/gpt-4o"
  },
  "debug": false,
  "dry_run": false,
  "log_level": "info"
}
EOF
    
    print_success "Created default configuration: $config_file"
    print_warning "Please edit $config_file and add your OpenRouter.ai API key"
}

# Test installation
test_installation() {
    print_info "Testing installation"
    
    # Test if wtf binary is accessible
    if command -v wtf >/dev/null 2>&1; then
        print_success "WTF CLI binary is accessible"
    else
        print_warning "WTF CLI binary not found in PATH. You may need to restart your shell."
    fi
    
    # Test shell integration
    if [[ -f "$WTF_DIR/integration.sh" ]]; then
        print_success "Shell integration script is installed"
    else
        print_error "Shell integration script not found"
        return 1
    fi
    
    # Test configuration
    if [[ -f "$WTF_DIR/config.json" ]]; then
        print_success "Configuration file is created"
    else
        print_error "Configuration file not found"
        return 1
    fi
}

# Print post-installation instructions
print_instructions() {
    echo ""
    echo -e "${GREEN}=================================================="
    echo "           Installation Complete!"
    echo -e "==================================================${NC}"
    echo ""
    echo -e "${YELLOW}Next Steps:${NC}"
    echo ""
    echo "1. ${BLUE}Get an OpenRouter.ai API key:${NC}"
    echo "   Visit: https://openrouter.ai"
    echo ""
    echo "2. ${BLUE}Configure your API key:${NC}"
    echo "   Edit: $WTF_DIR/config.json"
    echo "   Replace 'your_openrouter_api_key_here' with your actual API key"
    echo ""
    echo "3. ${BLUE}Restart your shell or run:${NC}"
    echo "   source ~/.bashrc"
    echo ""
    echo "4. ${BLUE}Test the installation:${NC}"
    echo "   ls -la"
    echo "   wtf"
    echo ""
    echo -e "${YELLOW}Alternative (for testing without API key):${NC}"
    echo "   WTF_DRY_RUN=true wtf"
    echo ""
    echo -e "${GREEN}Enjoy using WTF CLI! 🚀${NC}"
}

# Uninstall function
uninstall() {
    print_info "Uninstalling WTF CLI"
    
    # Remove binary
    if [[ -f "$HOME/.local/bin/wtf" ]]; then
        rm "$HOME/.local/bin/wtf"
        print_success "Removed binary: ~/.local/bin/wtf"
    fi
    
    # Remove WTF directory
    if [[ -d "$WTF_DIR" ]]; then
        read -p "Remove WTF directory and all data? (y/N): " -n 1 -r
        echo
        if [[ $REPLY =~ ^[Yy]$ ]]; then
            rm -rf "$WTF_DIR"
            print_success "Removed directory: $WTF_DIR"
        fi
    fi
    
    # Remove from bashrc
    if [[ -f "$BASHRC" ]] && grep -q "WTF CLI Shell Integration" "$BASHRC"; then
        print_info "Removing shell integration from ~/.bashrc"
        # Create a temporary file without WTF CLI lines
        # Remove the WTF CLI block (from comment to fi)
        sed '/# WTF CLI Shell Integration/,/^fi$/d' "$BASHRC" > "$BASHRC.tmp"
        # Also remove any standalone source line
        sed '/source ~\/.wtf\/integration.sh/d' "$BASHRC.tmp" > "$BASHRC.tmp2"
        mv "$BASHRC.tmp2" "$BASHRC"
        rm -f "$BASHRC.tmp"
        print_success "Removed shell integration from ~/.bashrc"
    fi
    
    print_success "Uninstallation complete"
}

# Main installation function
main() {
    print_header
    
    # Handle command line arguments
    case "${1:-}" in
        "uninstall"|"--uninstall"|"-u")
            uninstall
            exit 0
            ;;
        "help"|"--help"|"-h")
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  (no args)    Install WTF CLI"
            echo "  uninstall    Remove WTF CLI"
            echo "  help         Show this help"
            echo ""
            exit 0
            ;;
    esac
    
    print_info "Starting WTF CLI installation"
    
    # Run installation steps
    check_directory
    setup_wtf_directory
    install_integration_script
    install_wtf_binary
    setup_shell_integration
    create_default_config
    test_installation
    
    print_instructions
}

# Run main function
main "$@"
