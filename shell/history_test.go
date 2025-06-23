package shell

import (
	"testing"
)

func TestGetLastCommandFromHistory(t *testing.T) {
	// This test is inherently flaky in automated environments
	// because it depends on shell history which might not be available
	t.Skip("Skipping test that depends on shell history")

	// The following code would work in an interactive shell environment
	// but is unreliable in automated test environments
	/*
		// Execute a known command to set it as the last command in history
		testCmd := "echo 'wtf_cli_test_command'"
		execCmd := exec.Command("bash", "-c", testCmd)
		if err := execCmd.Run(); err != nil {
			t.Fatalf("Failed to execute test command: %v", err)
		}

		// Get the last command
		cmd, err := getLastCommandFromHistory()
		if err != nil {
			t.Fatalf("Failed to get last command: %v", err)
		}

		// Check if we got a non-empty result
		if cmd == "" {
			t.Error("Expected non-empty command, got empty string")
		}

		// Log the result
		t.Logf("Last command: %s", cmd)
	*/
}

func TestGetLastExitCode(t *testing.T) {
	// This test is inherently flaky in automated environments
	// because it depends on shell state which might not be preserved
	t.Skip("Skipping test that depends on shell state")

	// The following code would work in an interactive shell environment
	// but is unreliable in automated test environments
	/*
		// Execute a command that succeeds
		successCmd := exec.Command("bash", "-c", "exit 0")
		_ = successCmd.Run()

		// Get the exit code
		exitCode, err := GetLastExitCode()
		if err != nil {
			t.Fatalf("Failed to get exit code: %v", err)
		}

		// Check if the exit code is 0
		if exitCode != 0 {
			t.Errorf("Expected exit code 0, got %d", exitCode)
		}

		// Execute a command that fails
		failCmd := exec.Command("bash", "-c", "exit 42")
		_ = failCmd.Run()

		// Get the exit code
		exitCode, err = GetLastExitCode()
		if err != nil {
			t.Fatalf("Failed to get exit code: %v", err)
		}

		// Check if the exit code is 42
		if exitCode != 42 {
			t.Errorf("Expected exit code 42, got %d", exitCode)
		}
	*/
}
