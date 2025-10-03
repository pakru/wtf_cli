package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// TestShellIntegrationJSONReadWrite tests the core JSON read/write functionality
func TestShellIntegrationJSONReadWrite(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")

	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create .wtf directory
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create wtf directory: %v", err)
	}

	// Test data
	testData := ShellIntegrationData{
		Command:   "git status",
		ExitCode:  0,
		StartTime: "1703123456.789",
		EndTime:   "1703123457.123",
		Duration:  0.334,
		PWD:       "/home/user/project",
		Timestamp: "2023-12-21T10:30:57-08:00",
	}

	// Write test data to JSON file
	commandFile := filepath.Join(wtfDir, "last_command.json")
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(commandFile, data, 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}

	// Test reading the data back
	cmd, err := getCommandFromShellIntegration()
	if err != nil {
		t.Fatalf("Failed to read shell integration data: %v", err)
	}

	// Verify the data
	if cmd.Command != testData.Command {
		t.Errorf("Expected command %q, got %q", testData.Command, cmd.Command)
	}
	if cmd.ExitCode != testData.ExitCode {
		t.Errorf("Expected exit code %d, got %d", testData.ExitCode, cmd.ExitCode)
	}

	// Test shell integration detection
	if !IsShellIntegrationActive() {
		t.Error("Shell integration should be detected as active")
	}
}

// TestShellIntegrationInactive tests behavior when shell integration is not active
func TestShellIntegrationInactive(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()

	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Test when no .wtf directory exists
	if IsShellIntegrationActive() {
		t.Error("Shell integration should not be detected as active")
	}

	// Test reading when no file exists
	_, err := getCommandFromShellIntegration()
	if err == nil {
		t.Error("Expected error when reading non-existent shell integration data")
	}
}

// TestGetLastCommandWithShellIntegration tests the integration with GetLastCommand
func TestGetLastCommandWithShellIntegration(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	wtfDir := filepath.Join(tempDir, ".wtf")

	// Set up test environment
	originalHome := os.Getenv("HOME")
	defer os.Setenv("HOME", originalHome)
	os.Setenv("HOME", tempDir)

	// Create .wtf directory
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create wtf directory: %v", err)
	}

	// Test data
	testData := ShellIntegrationData{
		Command:   "make build",
		ExitCode:  2,
		StartTime: "1703123456.789",
		EndTime:   "1703123457.123",
		Duration:  0.334,
		PWD:       "/home/user/project",
		Timestamp: "2023-12-21T10:30:57-08:00",
	}

	// Write test data to JSON file
	commandFile := filepath.Join(wtfDir, "last_command.json")
	data, err := json.Marshal(testData)
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(commandFile, data, 0644); err != nil {
		t.Fatalf("Failed to write command file: %v", err)
	}

	// Test that GetLastCommand uses shell integration data
	cmd, err := GetLastCommand()
	if err != nil {
		t.Fatalf("GetLastCommand failed: %v", err)
	}

	// Verify the data comes from shell integration
	if cmd.Command != testData.Command {
		t.Errorf("Expected command %q, got %q", testData.Command, cmd.Command)
	}
	if cmd.ExitCode != testData.ExitCode {
		t.Errorf("Expected exit code %d, got %d", testData.ExitCode, cmd.ExitCode)
	}
}

// Note: TestShellIntegrationSetupInstructions was removed because
// GetShellIntegrationSetupInstructions function was deleted as unused

// Helper function to check if string contains substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr ||
			len(s) > len(substr) &&
				(s[:len(substr)] == substr ||
					s[len(s)-len(substr):] == substr ||
					containsInMiddle(s, substr)))
}

func containsInMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
