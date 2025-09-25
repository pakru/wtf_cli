package shell

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Note: ShellIntegrationData, getCommandFromShellIntegration, and IsShellIntegrationActive
// are now implemented in history.go

// TestShellIntegrationScript tests the shell integration script functionality
func TestShellIntegrationScript(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")
	
	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)
	
	tests := []struct {
		name        string
		command     string
		expectError bool
		exitCode    int
	}{
		{
			name:        "successful command",
			command:     "echo 'hello world'",
			expectError: false,
			exitCode:    0,
		},
		{
			name:        "failing command",
			command:     "ls /nonexistent/directory",
			expectError: false, // Script should handle this gracefully
			exitCode:    2,
		},
		{
			name:        "wtf command (should be skipped)",
			command:     "wtf --help",
			expectError: false,
			exitCode:    0,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up any existing files
			os.RemoveAll(wtfDir)
			
			// Test the shell integration by simulating command execution
			err := testShellIntegrationCommand(tt.command, tt.exitCode, wtfDir)
			
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			
			// For wtf commands, verify they were skipped
			if strings.HasPrefix(tt.command, "wtf") {
				commandFile := filepath.Join(wtfDir, "last_command.json")
				if _, err := os.Stat(commandFile); !os.IsNotExist(err) {
					t.Errorf("wtf command should have been skipped but file exists")
				}
				return
			}
			
			// Verify the JSON file was created correctly
			commandFile := filepath.Join(wtfDir, "last_command.json")
			if _, err := os.Stat(commandFile); os.IsNotExist(err) {
				t.Errorf("Command file was not created")
				return
			}
			
			// Read and verify the JSON content
			data, err := os.ReadFile(commandFile)
			if err != nil {
				t.Errorf("Failed to read command file: %v", err)
				return
			}
			
			var cmdData ShellIntegrationData
			if err := json.Unmarshal(data, &cmdData); err != nil {
				t.Errorf("Failed to parse JSON: %v", err)
				return
			}
			
			// Verify command data
			if cmdData.Command != tt.command {
				t.Errorf("Expected command %q, got %q", tt.command, cmdData.Command)
			}
			if cmdData.ExitCode != tt.exitCode {
				t.Errorf("Expected exit code %d, got %d", tt.exitCode, cmdData.ExitCode)
			}
			if cmdData.PWD == "" {
				t.Errorf("PWD should not be empty")
			}
			if cmdData.Duration < 0 {
				t.Errorf("Duration should be non-negative, got %f", cmdData.Duration)
			}
		})
	}
}

// testShellIntegrationCommand simulates the shell integration script behavior
func testShellIntegrationCommand(command string, exitCode int, wtfDir string) error {
	// Create the .wtf directory
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		return fmt.Errorf("failed to create wtf directory: %w", err)
	}
	
	// Simulate the shell integration script logic
	script := fmt.Sprintf(`
#!/bin/bash
WTF_DATA_DIR="%s"

# Function to capture command info (from integration.sh)
wtf_capture_command() {
    local exit_code=$1
    local command="$2"
    local start_time="$3"
    local end_time=$(date +%%s.%%N)
    
    # Skip if command is empty or starts with wtf
    [[ -z "$command" || "$command" =~ ^wtf.* ]] && return
    
    # Calculate duration (fallback if bc is not available)
    local duration
    if command -v bc >/dev/null 2>&1; then
        duration=$(echo "$end_time - $start_time" | bc -l 2>/dev/null || echo "0.0")
        # Ensure duration has leading zero for JSON validity
        [[ "$duration" =~ ^\. ]] && duration="0$duration"
    else
        duration="0.0"
    fi
    
    # Create command info file
    mkdir -p "$WTF_DATA_DIR" 2>/dev/null || true
    cat > "$WTF_DATA_DIR/last_command.json" << EOF
{
    "command": "$command",
    "exit_code": $exit_code,
    "start_time": "$start_time",
    "end_time": "$end_time",
    "duration": $duration,
    "pwd": "$PWD",
    "timestamp": "$(date -Iseconds)"
}
EOF
}

# Simulate command execution
start_time=$(date +%%s.%%N)
wtf_capture_command %d "%s" "$start_time"
`, wtfDir, exitCode, command)
	
	// Execute the test script
	cmd := exec.Command("bash", "-c", script)
	return cmd.Run()
}

// TestShellIntegrationWithRealBash tests with actual bash execution
func TestShellIntegrationWithRealBash(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping bash integration test in short mode")
	}
	
	// Create temporary directory
	tempDir := t.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")
	integrationScript := filepath.Join(wtfDir, "integration.sh")
	
	// Copy the integration script to temp directory
	sourceScript := "../scripts/integration.sh"
	if _, err := os.Stat(sourceScript); os.IsNotExist(err) {
		t.Skip("integration.sh not found, skipping real bash test")
	}
	
	// Create .wtf directory
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create wtf directory: %v", err)
	}
	
	// Copy integration script
	sourceData, err := os.ReadFile(sourceScript)
	if err != nil {
		t.Fatalf("Failed to read source script: %v", err)
	}
	
	// Modify script to use temp directory
	modifiedScript := strings.ReplaceAll(string(sourceData), 
		`WTF_DATA_DIR="$HOME/.wtf"`, 
		fmt.Sprintf(`WTF_DATA_DIR="%s"`, wtfDir))
	
	if err := os.WriteFile(integrationScript, []byte(modifiedScript), 0755); err != nil {
		t.Fatalf("Failed to write integration script: %v", err)
	}
	
	// Test script with real interactive bash
	testScript := fmt.Sprintf(`
#!/bin/bash
source "%s"

# Simulate an interactive session by manually calling the functions
# This mimics what would happen in a real interactive bash session
WTF_COMMAND_START=$(date +%%s.%%N)
WTF_LAST_COMMAND="test_command_that_fails"
wtf_prompt_command

exit 0
`, integrationScript)
	
	cmd := exec.Command("bash", "-c", testScript)
	if err := cmd.Run(); err != nil {
		t.Errorf("Failed to execute test script: %v", err)
	}
	
	// Verify the command was captured
	commandFile := filepath.Join(wtfDir, "last_command.json")
	if _, err := os.Stat(commandFile); os.IsNotExist(err) {
		t.Errorf("Command file was not created by real bash execution")
		return
	}
	
	// Read and verify content
	data, err := os.ReadFile(commandFile)
	if err != nil {
		t.Errorf("Failed to read command file: %v", err)
		return
	}
	
	var cmdData ShellIntegrationData
	if err := json.Unmarshal(data, &cmdData); err != nil {
		t.Errorf("Failed to parse JSON from real bash: %v", err)
		return
	}
	
	// Verify basic structure
	if cmdData.Command == "" {
		t.Errorf("Command should not be empty")
	}
	if cmdData.PWD == "" {
		t.Errorf("PWD should not be empty")
	}
}

// TestGetCommandFromShellIntegration tests the Go function that reads shell integration data
func TestGetCommandFromShellIntegration(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")
	
	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)
	
	tests := []struct {
		name        string
		setupData   ShellIntegrationData
		expectError bool
	}{
		{
			name: "valid command data",
			setupData: ShellIntegrationData{
				Command:   "git status",
				ExitCode:  0,
				StartTime: "1703123456.789",
				EndTime:   "1703123456.891",
				Duration:  0.102,
				PWD:       "/home/user/project",
				Timestamp: "2023-12-20T15:30:56-08:00",
			},
			expectError: false,
		},
		{
			name: "failed command data",
			setupData: ShellIntegrationData{
				Command:   "make build",
				ExitCode:  2,
				StartTime: "1703123456.789",
				EndTime:   "1703123456.891",
				Duration:  0.102,
				PWD:       "/home/user/project",
				Timestamp: "2023-12-20T15:30:56-08:00",
			},
			expectError: false,
		},
		{
			name:        "no data",
			setupData:   ShellIntegrationData{},
			expectError: true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up
			os.RemoveAll(wtfDir)
			
			if tt.setupData.Command != "" {
				// Create .wtf directory and command file
				if err := os.MkdirAll(wtfDir, 0755); err != nil {
					t.Fatalf("Failed to create wtf directory: %v", err)
				}
				
				// Write test data
				data, err := json.Marshal(tt.setupData)
				if err != nil {
					t.Fatalf("Failed to marshal test data: %v", err)
				}
				
				commandFile := filepath.Join(wtfDir, "last_command.json")
				if err := os.WriteFile(commandFile, data, 0644); err != nil {
					t.Fatalf("Failed to write command file: %v", err)
				}
				
				// Optionally create output file
				outputFile := filepath.Join(wtfDir, "last_output.txt")
				testOutput := "test command output\nline 2"
				if err := os.WriteFile(outputFile, []byte(testOutput), 0644); err != nil {
					t.Fatalf("Failed to write output file: %v", err)
				}
			}
			
			// Test the Go function
			cmd, err := getCommandFromShellIntegration()
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			// Verify the parsed data
			if cmd.Command != tt.setupData.Command {
				t.Errorf("Expected command %q, got %q", tt.setupData.Command, cmd.Command)
			}
			if cmd.ExitCode != tt.setupData.ExitCode {
				t.Errorf("Expected exit code %d, got %d", tt.setupData.ExitCode, cmd.ExitCode)
			}
			// Note: CommandInfo doesn't have PWD and Duration fields
			// These would be part of extended shell integration data
			
			// Note: Output is no longer read from separate file since shell integration
			// now only captures command and exit code information
		})
	}
}

// TestIsShellIntegrationActive tests the shell integration detection
func TestIsShellIntegrationActive(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	
	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)
	
	// Test when integration is not active
	if IsShellIntegrationActive() {
		t.Errorf("Expected shell integration to be inactive")
	}
	
	// Create .wtf directory and command file
	wtfDir := filepath.Join(tempDir, ".wtf")
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create wtf directory: %v", err)
	}
	
	commandFile := filepath.Join(wtfDir, "last_command.json")
	testData := `{"command": "test", "exit_code": 0}`
	if err := os.WriteFile(commandFile, []byte(testData), 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}
	
	// Test when integration is active
	if !IsShellIntegrationActive() {
		t.Errorf("Expected shell integration to be active")
	}
}

// BenchmarkShellIntegrationCapture benchmarks the command capture performance
func BenchmarkShellIntegrationCapture(b *testing.B) {
	tempDir := b.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")
	
	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)
	
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		b.Fatalf("Failed to create wtf directory: %v", err)
	}
	
	b.ResetTimer()
	
	for i := 0; i < b.N; i++ {
		command := fmt.Sprintf("test command %d", i)
		if err := testShellIntegrationCommand(command, 0, wtfDir); err != nil {
			b.Errorf("Failed to capture command: %v", err)
		}
	}
}

// TestShellIntegrationEndToEnd tests the complete flow
func TestShellIntegrationEndToEnd(t *testing.T) {
	// Create temporary directory
	tempDir := t.TempDir()
	
	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)
	
	// Simulate shell integration capturing a command
	wtfDir := filepath.Join(tempDir, ".wtf")
	testCommand := "git push origin main"
	testExitCode := 1
	
	if err := testShellIntegrationCommand(testCommand, testExitCode, wtfDir); err != nil {
		t.Fatalf("Failed to simulate shell integration: %v", err)
	}
	
	// Test that GetLastCommand can read the data
	cmd, err := GetLastCommand()
	if err != nil {
		t.Fatalf("GetLastCommand failed: %v", err)
	}
	
	// Verify the data
	if cmd.Command != testCommand {
		t.Errorf("Expected command %q, got %q", testCommand, cmd.Command)
	}
	if cmd.ExitCode != testExitCode {
		t.Errorf("Expected exit code %d, got %d", testExitCode, cmd.ExitCode)
	}
	
	// Verify shell integration is detected as active
	if !IsShellIntegrationActive() {
		t.Errorf("Shell integration should be detected as active")
	}
}
