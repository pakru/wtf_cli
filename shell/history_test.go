package shell

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"wtf_cli/logger"
)

func TestShellIntegrationFunctions(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")
	
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "wtf-shell-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Set WTF_DATA_DIR to our temp directory
	originalDataDir := os.Getenv("WTF_DATA_DIR")
	os.Setenv("WTF_DATA_DIR", tempDir)
	defer func() {
		if originalDataDir != "" {
			os.Setenv("WTF_DATA_DIR", originalDataDir)
		} else {
			os.Unsetenv("WTF_DATA_DIR")
		}
	}()

	// Test IsShellIntegrationActive when no file exists
	// Mock home directory to use our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	if IsShellIntegrationActive() {
		t.Error("Expected shell integration to be inactive when no file exists")
	}

	// Create .wtf directory and mock shell integration file
	wtfDir := filepath.Join(tempDir, ".wtf")
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create .wtf directory: %v", err)
	}

	integrationFile := filepath.Join(wtfDir, "last_command.json")
	testData := map[string]interface{}{
		"command":   "git status",
		"exit_code": 0,
		"timestamp": "2024-01-01T12:00:00Z",
	}

	jsonData, err := json.MarshalIndent(testData, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(integrationFile, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Create output file
	outputFile := filepath.Join(wtfDir, "last_output.txt")
	if err := os.WriteFile(outputFile, []byte("On branch main\nnothing to commit, working tree clean"), 0644); err != nil {
		t.Fatalf("Failed to write output file: %v", err)
	}

	// Test IsShellIntegrationActive when file exists
	if !IsShellIntegrationActive() {
		t.Error("Expected shell integration to be active when file exists")
	}

	// Test getCommandFromShellIntegration
	cmdInfo, err := getCommandFromShellIntegration()
	if err != nil {
		t.Fatalf("Failed to get command from shell integration: %v", err)
	}

	if cmdInfo.Command != "git status" {
		t.Errorf("Expected command 'git status', got '%s'", cmdInfo.Command)
	}
	if cmdInfo.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", cmdInfo.ExitCode)
	}
	// Note: Output is no longer read from separate file since shell integration
	// now only captures command and exit code information

	// Note: GetShellIntegrationSetupInstructions was removed as it was unused
}

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "wtf-env-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock home directory to ensure no shell integration file exists
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Test with environment variables set
	os.Setenv("WTF_LAST_COMMAND", "env-command")
	os.Setenv("WTF_LAST_EXIT_CODE", "42")
	os.Setenv("WTF_LAST_OUTPUT", "env-output")
	defer func() {
		os.Unsetenv("WTF_LAST_COMMAND")
		os.Unsetenv("WTF_LAST_EXIT_CODE")
		os.Unsetenv("WTF_LAST_OUTPUT")
	}()

	cmdInfo, err := GetLastCommand()
	if err != nil {
		t.Fatalf("Failed to get last command: %v", err)
	}

	if cmdInfo.Command != "env-command" {
		t.Errorf("Expected command from env var, got '%s'", cmdInfo.Command)
	}
	if cmdInfo.ExitCode != 42 {
		t.Errorf("Expected exit code from env var, got %d", cmdInfo.ExitCode)
	}
	if cmdInfo.Output != "env-output" {
		t.Errorf("Expected output from env var, got '%s'", cmdInfo.Output)
	}
}

func TestGetLastCommandPriority(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")
	
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "wtf-shell-priority-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock home directory to use our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .wtf directory
	wtfDir := filepath.Join(tempDir, ".wtf")
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create .wtf directory: %v", err)
	}

	// Set environment variables
	os.Setenv("WTF_LAST_COMMAND", "env-command")
	os.Setenv("WTF_LAST_EXIT_CODE", "1")
	os.Setenv("WTF_LAST_OUTPUT", "env-output")
	defer func() {
		os.Unsetenv("WTF_LAST_COMMAND")
		os.Unsetenv("WTF_LAST_EXIT_CODE")
		os.Unsetenv("WTF_LAST_OUTPUT")
	}()

	// Create shell integration file (should have priority over env vars)
	integrationFile := filepath.Join(wtfDir, "last_command.json")
	testData := map[string]interface{}{
		"command":   "shell-integration-command",
		"exit_code": 0,
		"timestamp": "2024-01-01T12:00:00Z",
	}

	jsonData, err := json.MarshalIndent(testData, "", "  ")
	if err != nil {
		t.Fatalf("Failed to marshal test data: %v", err)
	}

	if err := os.WriteFile(integrationFile, jsonData, 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// GetLastCommand should prioritize shell integration over env vars
	cmdInfo, err := GetLastCommand()
	if err != nil {
		t.Fatalf("Failed to get last command: %v", err)
	}

	if cmdInfo.Command != "shell-integration-command" {
		t.Errorf("Expected shell integration command to have priority, got '%s'", cmdInfo.Command)
	}
	if cmdInfo.ExitCode != 0 {
		t.Errorf("Expected shell integration exit code, got %d", cmdInfo.ExitCode)
	}
	// Note: Output is no longer read from separate file since shell integration
	// now only captures command and exit code information
}

func TestInvalidShellIntegrationData(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")
	
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "wtf-shell-invalid-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Mock home directory to use our temp directory
	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tempDir)
	defer os.Setenv("HOME", originalHome)

	// Create .wtf directory
	wtfDir := filepath.Join(tempDir, ".wtf")
	if err := os.MkdirAll(wtfDir, 0755); err != nil {
		t.Fatalf("Failed to create .wtf directory: %v", err)
	}

	// Create invalid JSON file
	integrationFile := filepath.Join(wtfDir, "last_command.json")
	invalidJSON := `{"command": "test", "exit_code": "invalid"}`
	
	if err := os.WriteFile(integrationFile, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to write invalid test file: %v", err)
	}

	// Should return error for invalid JSON
	_, err = getCommandFromShellIntegration()
	if err == nil {
		t.Error("Expected error for invalid JSON, got nil")
	}
}
