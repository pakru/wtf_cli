#!/bin/bash

# WTF CLI Test Environment Setup Script
# This script creates a complete isolated test environment

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

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Get the directory where this script is located
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(dirname "$SCRIPT_DIR")"
TEST_ENV_DIR="$SCRIPT_DIR"

print_info "Setting up WTF CLI test environment..."
print_info "Test environment directory: $TEST_ENV_DIR"
print_info "Project root: $PROJECT_ROOT"

# Create test home directory
TEST_HOME="$TEST_ENV_DIR/home"
mkdir -p "$TEST_HOME"
mkdir -p "$TEST_HOME/.wtf"
mkdir -p "$TEST_HOME/.local/bin"

print_success "Created test home directory: $TEST_HOME"

# Copy integration script
cp "$PROJECT_ROOT/scripts/integration.sh" "$TEST_HOME/.wtf/"
chmod +x "$TEST_HOME/.wtf/integration.sh"
print_success "Copied integration script to test environment"

# Build WTF CLI binary for testing
print_info "Building WTF CLI binary..."
cd "$PROJECT_ROOT"
if go build -o "$TEST_HOME/.local/bin/wtf" .; then
    print_success "Built WTF CLI binary"
else
    print_error "Failed to build WTF CLI binary"
    exit 1
fi

# Create test .bashrc
cat > "$TEST_HOME/.bashrc" << 'EOF'
# WTF CLI Test Environment .bashrc

# Set a distinctive prompt
export PS1='\[\033[1;32m\][WTF-TEST]\[\033[0m\] \u@\h:\w$ '

# Add local bin to PATH
export PATH="$HOME/.local/bin:$PATH"

# Source WTF integration
if [[ -f ~/.wtf/integration.sh ]]; then
    source ~/.wtf/integration.sh
    echo "✅ WTF CLI shell integration loaded"
else
    echo "⚠️  WTF CLI integration script not found"
fi

# Enable debug mode for testing
export WTF_DEBUG=true
export WTF_DRY_RUN=true

# Test aliases for convenience
alias ll='ls -la'
alias wtf-status='echo "WTF CLI Test Environment Status:"; echo "- Binary: $(which wtf 2>/dev/null || echo "not found")"; echo "- Integration: $(test -f ~/.wtf/integration.sh && echo "loaded" || echo "not loaded")"; echo "- Last command file: $(test -f ~/.wtf/last_command.json && echo "exists" || echo "not found")"'
alias wtf-test='echo "Testing WTF CLI..."; echo "hello world"; wtf'
alias wtf-clean='rm -f ~/.wtf/last_command.json ~/.wtf/last_output.txt ~/.wtf/command_output.tmp; echo "Cleaned WTF CLI data files"'

# Welcome message
echo ""
echo "🧪 WTF CLI Test Environment Ready!"
echo ""
echo "Available commands:"
echo "  wtf-status  - Show test environment status"
echo "  wtf-test    - Run a quick test"
echo "  wtf-clean   - Clean data files"
echo "  wtf --help  - Show WTF CLI help"
echo ""
echo "Try running some commands to test the integration:"
echo "  ls -la"
echo "  echo 'hello world'"
echo "  wtf"
echo ""
EOF

print_success "Created test .bashrc"

# Create default config for testing
cat > "$TEST_HOME/.wtf/config.json" << 'EOF'
{
    "api_key": "test-key-placeholder",
    "model": "anthropic/claude-3.5-sonnet",
    "base_url": "https://openrouter.ai/api/v1",
    "max_tokens": 1000,
    "temperature": 0.7
}
EOF

print_success "Created test configuration"

# Create start script
cat > "$TEST_ENV_DIR/start_test_env.sh" << EOF
#!/bin/bash

# Start WTF CLI Test Environment
TEST_HOME="$TEST_HOME"

echo "🧪 Starting WTF CLI Test Environment..."
echo "Test HOME: \$TEST_HOME"
echo ""
echo "To exit the test environment, type 'exit'"
echo ""

# Start bash with test environment
HOME="\$TEST_HOME" bash --rcfile "\$TEST_HOME/.bashrc"
EOF

chmod +x "$TEST_ENV_DIR/start_test_env.sh"
print_success "Created start script"

# Create cleanup script
cat > "$TEST_ENV_DIR/cleanup_test_env.sh" << EOF
#!/bin/bash

# Cleanup WTF CLI Test Environment
TEST_ENV_DIR="$TEST_ENV_DIR"

echo "🧹 Cleaning up WTF CLI test environment..."
rm -rf "\$TEST_ENV_DIR/home"
echo "✅ Test environment cleaned up"
EOF

chmod +x "$TEST_ENV_DIR/cleanup_test_env.sh"
print_success "Created cleanup script"

# Create README for test environment
cat > "$TEST_ENV_DIR/README.md" << 'EOF'
# WTF CLI Test Environment

This directory contains a complete isolated test environment for the WTF CLI tool.

## Files

- `setup_test_env.sh` - Sets up the test environment
- `start_test_env.sh` - Starts the test environment
- `cleanup_test_env.sh` - Cleans up the test environment
- `home/` - Isolated home directory for testing

## Usage

1. **Setup** (run once):
   ```bash
   ./setup_test_env.sh
   ```

2. **Start test environment**:
   ```bash
   ./start_test_env.sh
   ```

3. **Test the integration**:
   ```bash
   # Inside the test environment
   wtf-status          # Check status
   ls -la              # Run a command
   wtf                 # Test WTF CLI
   wtf-clean           # Clean data files
   exit                # Exit test environment
   ```

4. **Cleanup** (when done):
   ```bash
   ./cleanup_test_env.sh
   ```

## Features

- Isolated home directory
- WTF CLI binary built and installed
- Shell integration script loaded
- Debug mode enabled
- Distinctive prompt `[WTF-TEST]`
- Helpful aliases and commands
- Test configuration file

## Environment Variables

- `WTF_DEBUG=true` - Enable debug logging
- `WTF_DRY_RUN=true` - Enable dry-run mode
- `PATH` includes `~/.local/bin`
EOF

print_success "Created README"

echo ""
print_success "🎉 WTF CLI test environment setup complete!"
echo ""
echo "Next steps:"
echo "1. Start the test environment: ./start_test_env.sh"
echo "2. Test the integration by running commands"
echo "3. When done, cleanup: ./cleanup_test_env.sh"
echo ""
