#!/bin/bash
# Test script for version functionality

set -e

echo "=== Testing WTF CLI Versioning ==="
echo

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
BLUE='\033[0;34m'
NC='\033[0m'

print_test() {
    echo -e "${BLUE}[TEST]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

print_error() {
    echo -e "${RED}[FAIL]${NC} $1"
}

# Test 1: Build with version info
print_test "Building with version information..."
make build
print_success "Build completed"
echo

# Test 2: Check version output
print_test "Testing --version flag..."
VERSION_OUTPUT=$(./build/wtf --version)
echo "$VERSION_OUTPUT"
echo

if echo "$VERSION_OUTPUT" | grep -q "wtf version"; then
    print_success "Version output contains 'wtf version'"
else
    print_error "Version output missing 'wtf version'"
    exit 1
fi

if echo "$VERSION_OUTPUT" | grep -q "commit:"; then
    print_success "Version output contains commit info"
else
    print_error "Version output missing commit info"
    exit 1
fi

if echo "$VERSION_OUTPUT" | grep -q "built:"; then
    print_success "Version output contains build date"
else
    print_error "Version output missing build date"
    exit 1
fi

if echo "$VERSION_OUTPUT" | grep -q "go:"; then
    print_success "Version output contains Go version"
else
    print_error "Version output missing Go version"
    exit 1
fi

# Test 3: Test -v flag
print_test "Testing -v flag..."
SHORT_VERSION=$(./build/wtf -v)
if echo "$SHORT_VERSION" | grep -q "wtf version"; then
    print_success "-v flag works"
else
    print_error "-v flag failed"
    exit 1
fi

# Test 4: Test version subcommand
print_test "Testing 'version' subcommand..."
VERSION_CMD=$(./build/wtf version)
if echo "$VERSION_CMD" | grep -q "wtf version"; then
    print_success "version subcommand works"
else
    print_error "version subcommand failed"
    exit 1
fi

# Test 5: Run unit tests
print_test "Running version unit tests..."
go test -v -run TestVersion
print_success "Unit tests passed"
echo

echo "=== All Version Tests Passed ==="
