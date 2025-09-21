package shell

import (
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
