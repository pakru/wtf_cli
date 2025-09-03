#!/bin/bash
# Test script for shell integration functionality
# This script can be run independently or called from Go tests

set -e

# Configuration
TEST_DIR="${TEST_DIR:-/tmp/wtf_test_$$}"
WTF_DATA_DIR="$TEST_DIR/.wtf"
INTEGRATION_SCRIPT="$(dirname "$0")/../scripts/integration.sh"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Test counter
TESTS_RUN=0
TESTS_PASSED=0

# Logging functions
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Test helper functions
setup_test_env() {
    log_info "Setting up test environment in $TEST_DIR"
    
    # Clean up any existing test directory
    rm -rf "$TEST_DIR"
    mkdir -p "$WTF_DATA_DIR"
    
    # Copy integration script to test directory
    if [[ ! -f "$INTEGRATION_SCRIPT" ]]; then
        log_error "Integration script not found at $INTEGRATION_SCRIPT"
        exit 1
    fi
    
    # Create modified integration script for testing
    sed "s|WTF_DATA_DIR=\"\$HOME/.wtf\"|WTF_DATA_DIR=\"$WTF_DATA_DIR\"|g" \
        "$INTEGRATION_SCRIPT" > "$TEST_DIR/integration.sh"
    chmod +x "$TEST_DIR/integration.sh"
}

cleanup_test_env() {
    log_info "Cleaning up test environment"
    rm -rf "$TEST_DIR"
}

run_test() {
    local test_name="$1"
    local test_command="$2"
    local expected_exit_code="${3:-0}"
    local should_capture="${4:-true}"
    
    TESTS_RUN=$((TESTS_RUN + 1))
    log_info "Running test: $test_name"
    
    # Clean up previous test data
    rm -f "$WTF_DATA_DIR/last_command.json"
    rm -f "$WTF_DATA_DIR/last_output.txt"
    
    # Create test script that sources integration and runs command
    cat > "$TEST_DIR/test_script.sh" << EOF
#!/bin/bash
source "$TEST_DIR/integration.sh"

# Simulate command execution with proper hooks
WTF_COMMAND_START=\$(date +%s.%N)
WTF_LAST_COMMAND="$test_command"

# Execute the command and capture exit code immediately
if $test_command; then
    exit_code=0
else
    exit_code=\$?
fi

# Capture the command with the actual exit code
wtf_capture_command "\$WTF_LAST_COMMAND" "\$WTF_COMMAND_START" "\$exit_code"

exit \$exit_code
EOF
    
    chmod +x "$TEST_DIR/test_script.sh"
    
    # Run the test script
    if bash "$TEST_DIR/test_script.sh" >/dev/null 2>&1; then
        actual_exit_code=0
    else
        actual_exit_code=$?
    fi
    
    # Verify exit code if specified
    if [[ "$expected_exit_code" != "any" && "$actual_exit_code" != "$expected_exit_code" ]]; then
        log_error "Test '$test_name' failed: expected exit code $expected_exit_code, got $actual_exit_code"
        return 1
    fi
    
    # Check if command should have been captured
    if [[ "$should_capture" == "true" ]]; then
        if [[ ! -f "$WTF_DATA_DIR/last_command.json" ]]; then
            log_error "Test '$test_name' failed: command file not created"
            return 1
        fi
        
        # Verify JSON structure
        if ! python3 -m json.tool "$WTF_DATA_DIR/last_command.json" >/dev/null 2>&1; then
            log_error "Test '$test_name' failed: invalid JSON in command file"
            return 1
        fi
        
        # Verify command content
        local captured_command
        captured_command=$(python3 -c "
import json
with open('$WTF_DATA_DIR/last_command.json') as f:
    data = json.load(f)
    print(data.get('command', ''))
" 2>/dev/null)
        
        if [[ "$captured_command" != "$test_command" ]]; then
            log_error "Test '$test_name' failed: expected command '$test_command', got '$captured_command'"
            return 1
        fi
        
        log_info "Test '$test_name' passed: command captured correctly"
    else
        if [[ -f "$WTF_DATA_DIR/last_command.json" ]]; then
            log_error "Test '$test_name' failed: command should not have been captured"
            return 1
        fi
        
        log_info "Test '$test_name' passed: command correctly skipped"
    fi
    
    TESTS_PASSED=$((TESTS_PASSED + 1))
    return 0
}

# Test cases
test_successful_command() {
    run_test "successful_command" "echo 'hello world'" 0 true
}

test_failing_command() {
    run_test "failing_command" "ls /nonexistent/directory" 2 true
}

test_wtf_command_skipped() {
    run_test "wtf_command_skipped" "wtf --help" any false
}

test_empty_command_skipped() {
    run_test "empty_command_skipped" "" 0 false
}

test_complex_command() {
    run_test "complex_command" "find /tmp -name '*.tmp' | head -5" 0 true
}

test_command_with_pipes() {
    run_test "command_with_pipes" "echo 'test' | grep 'test' | wc -l" 0 true
}

test_json_structure() {
    log_info "Testing JSON structure completeness"
    
    # Run a simple command
    run_test "json_structure_test" "date" 0 true
    
    # Verify all required fields are present
    local required_fields=("command" "exit_code" "start_time" "end_time" "duration" "pwd" "timestamp")
    
    for field in "${required_fields[@]}"; do
        if ! python3 -c "
import json
with open('$WTF_DATA_DIR/last_command.json') as f:
    data = json.load(f)
    if '$field' not in data:
        exit(1)
" 2>/dev/null; then
            log_error "JSON structure test failed: missing field '$field'"
            return 1
        fi
    done
    
    log_info "JSON structure test passed: all required fields present"
    TESTS_PASSED=$((TESTS_PASSED + 1))
}

test_performance() {
    log_info "Testing performance (100 command captures)"
    
    local start_time
    start_time=$(date +%s.%N)
    
    for i in {1..100}; do
        run_test "perf_test_$i" "echo 'test $i'" 0 true >/dev/null 2>&1
    done
    
    local end_time
    end_time=$(date +%s.%N)
    local duration
    duration=$(echo "$end_time - $start_time" | bc -l)
    
    log_info "Performance test completed: 100 captures in ${duration}s ($(echo "scale=3; $duration * 10" | bc -l)ms average)"
    
    # Performance should be reasonable (less than 1 second for 100 captures)
    if (( $(echo "$duration > 1.0" | bc -l) )); then
        log_warn "Performance test: captures took longer than expected (${duration}s)"
    else
        log_info "Performance test passed"
    fi
}

# Main test execution
main() {
    log_info "Starting shell integration tests"
    
    # Check dependencies
    if ! command -v python3 >/dev/null 2>&1; then
        log_error "python3 is required for JSON validation tests"
        exit 1
    fi
    
    if ! command -v bc >/dev/null 2>&1; then
        log_error "bc is required for duration calculations"
        exit 1
    fi
    
    # Set up test environment
    setup_test_env
    
    # Trap cleanup on exit
    trap cleanup_test_env EXIT
    
    # Run all tests
    test_successful_command
    test_failing_command
    test_wtf_command_skipped
    test_empty_command_skipped
    test_complex_command
    test_command_with_pipes
    test_json_structure
    test_performance
    
    # Report results
    log_info "Test Results: $TESTS_PASSED/$TESTS_RUN tests passed"
    
    if [[ "$TESTS_PASSED" == "$TESTS_RUN" ]]; then
        log_info "All tests passed! ✅"
        exit 0
    else
        log_error "Some tests failed! ❌"
        exit 1
    fi
}

# Run main function if script is executed directly
if [[ "${BASH_SOURCE[0]}" == "${0}" ]]; then
    main "$@"
fi
