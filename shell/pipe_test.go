package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"wtf_cli/config"
	"wtf_cli/logger"
)

func TestIsReadingFromPipe(t *testing.T) {
	// Test when stdin is a terminal (should return false)
	result := IsReadingFromPipe()
	// In a test environment, this might be true or false depending on how tests are run
	// So we just verify the function doesn't panic
	_ = result
}

func TestPipeHandler_HandlePipeInput(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	cfg := config.Config{
		DryRun: true,
	}
	handler := NewPipeHandler(cfg)

	// Test when not reading from pipe
	_, err := handler.HandlePipeInput()
	if err != nil {
		t.Errorf("HandlePipeInput failed: %v", err)
	}
	// In test environment, we might not have pipe input, so just verify no error
}

func TestPipeHandler_ProcessPipeMode(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	cfg := config.Config{
		DryRun: true,
	}
	handler := NewPipeHandler(cfg)

	tests := []struct {
		name        string
		input       string
		expectError bool
	}{
		{
			name:        "empty input",
			input:       "",
			expectError: false,
		},
		{
			name:        "error message",
			input:       "ls: cannot access '/nonexistent': No such file or directory",
			expectError: false,
		},
		{
			name:        "multiline input",
			input:       "error: failed to connect\ndetails: timeout\nretry: attempt 2/5",
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.ProcessPipeMode(tt.input)
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}


func TestNewPipeHandler(t *testing.T) {
	cfg := config.Config{
		DryRun: true,
	}
	handler := NewPipeHandler(cfg)

	if handler == nil {
		t.Error("NewPipeHandler returned nil")
	}

	if handler.config.DryRun != cfg.DryRun {
		t.Error("Handler config not set correctly")
	}
}

// TestPipeCommandDetection tests the new pipe command detection functionality
func TestPipeCommandDetection(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	cfg := config.Config{DryRun: true}
	handler := NewPipeHandler(cfg)

	tests := []struct {
		name           string
		input          string
		setupShellData func(t *testing.T) func() // setup function that returns cleanup function
		expectedCmd    string
		expectPipeInfo bool
		description    string
	}{
		{
			name:  "pipe command tracking disabled - ls command",
			input: "ls: cannot access '/nonexistent': No such file or directory",
			setupShellData: func(t *testing.T) func() {
				return createMockShellIntegration(t, ShellIntegrationData{
					Command:  "ls /nonexistent | wtf",
					ExitCode: 2,
				})
			},
			expectedCmd:    "[N/A]",
			expectPipeInfo: false,
			description:    "Should show [N/A] since pipe tracking is disabled",
		},
		{
			name:  "pipe command tracking disabled - git command",
			input: "fatal: not a git repository",
			setupShellData: func(t *testing.T) func() {
				shellData := ShellIntegrationData{
					Command:  "echo test | wtf",
					ExitCode: 0,
				}
				return createMockShellIntegration(t, shellData)
			},
			expectedCmd:    "[N/A]",
			expectPipeInfo: false,
			description:    "Should show [N/A] since pipe tracking is disabled",
		},
		{
			name:  "no pipe command info",
			input: "some error message",
			setupShellData: func(t *testing.T) func() {
				return createMockShellIntegration(t, ShellIntegrationData{
					Command:  "some other command",
					ExitCode: 1,
				})
			},
			expectedCmd:    "[N/A]",
			expectPipeInfo: false,
			description:    "Should show [N/A] when no pipe info available",
		},
		{
			name:  "no shell integration file",
			input: "error without shell integration",
			setupShellData: func(t *testing.T) func() {
				// Return cleanup function that does nothing
				return func() {}
			},
			expectedCmd:    "[N/A]",
			expectPipeInfo: false,
			description:    "Should show [N/A] when no shell integration file exists",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup test environment
			cleanup := tt.setupShellData(t)
			defer cleanup()

			t.Logf("Test case: %s", tt.description)
			t.Logf("Input: %q", tt.input)
			t.Logf("Expected command: %s", tt.expectedCmd)

			// Test pipe mode processing
			err := handler.ProcessPipeMode(tt.input)
			if err != nil {
				t.Errorf("ProcessPipeMode failed: %v", err)
			}

			// Test GetPipeCommandInfo directly
			pipeCmd, err := GetPipeCommandInfo()
			if tt.expectPipeInfo {
				if err != nil {
					t.Errorf("Expected pipe command info but got error: %v", err)
				} else if pipeCmd.Command != tt.expectedCmd {
					t.Errorf("Expected command %q, got %q", tt.expectedCmd, pipeCmd.Command)
				}
			} else {
				if err == nil {
					t.Errorf("Expected no pipe command info but got: %+v", pipeCmd)
				}
			}
		})
	}
}

// TestPipeIntegrationEndToEnd tests the complete pipe flow
func TestPipeIntegrationEndToEnd(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		description string
	}{
		{
			name:        "error message analysis",
			input:       "ls: cannot access '/nonexistent': No such file or directory",
			description: "Should analyze file not found error",
		},
		{
			name:        "success message analysis",
			input:       "file1.txt\nfile2.txt\nfile3.txt",
			description: "Should analyze successful directory listing",
		},
		{
			name:        "mixed output analysis",
			input:       "warning: deprecated function\ninfo: continuing...\nerror: failed to connect",
			description: "Should analyze mixed log output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test would simulate the complete pipe flow:
			// 1. Detect pipe input
			// 2. Read pipe input
			// 3. Create CommandInfo
			// 4. Process pipe mode
			// 5. Display results

			t.Logf("Test case: %s", tt.description)
			t.Logf("Input: %q", tt.input)

			// Initialize logger and create handler
			logger.InitLogger("info")
			cfg := config.Config{DryRun: true}
			handler := NewPipeHandler(cfg)

			// Test pipe mode processing
			err := handler.ProcessPipeMode(tt.input)
			if err != nil {
				t.Errorf("ProcessPipeMode failed: %v", err)
			}
		})
	}
}

// createMockShellIntegration creates a mock shell integration file for testing
// Returns a cleanup function that should be called to remove the test file
func createMockShellIntegration(t *testing.T, data ShellIntegrationData) func() {
	// Create a temporary directory for the test
	tempDir := t.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")
	
	// Create the .wtf directory
	err := os.MkdirAll(wtfDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create test .wtf directory: %v", err)
	}
	
	// Fill in default values if not provided
	if data.StartTime == "" {
		data.StartTime = "1640995200.123"
	}
	if data.EndTime == "" {
		data.EndTime = "1640995200.456"
	}
	if data.Duration == 0 {
		data.Duration = 0.333
	}
	if data.PWD == "" {
		data.PWD = "/home/user/test"
	}
	if data.Timestamp == "" {
		data.Timestamp = "2024-01-01T12:00:00+00:00"
	}
	
	// Create the command file
	commandFile := filepath.Join(wtfDir, "last_command.json")
	jsonData, err := json.MarshalIndent(data, "", "    ")
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}
	
	err = os.WriteFile(commandFile, jsonData, 0644)
	if err != nil {
		t.Fatalf("Failed to write test command file: %v", err)
	}
	
	// Store original HOME and set it to our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	
	// Return cleanup function
	return func() {
		// Restore original HOME
		os.Setenv("HOME", originalHome)
		// Temp directory will be cleaned up automatically by t.TempDir()
	}
}
