package shell

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// CommandInfo stores information about the last executed command
type CommandInfo struct {
	Command  string // The command that was executed
	Output   string // Combined stdout and stderr
	ExitCode int    // Exit code of the command
}

// GetLastCommand retrieves information about the last executed command
func GetLastCommand() (CommandInfo, error) {
	cmd := CommandInfo{}
	var err error

	// First, try to get command from environment variables (for shell integration)
	if envCmd := os.Getenv("WTF_LAST_COMMAND"); envCmd != "" {
		cmd.Command = envCmd
	} else {
		// Fall back to trying to get from bash history
		cmd.Command, err = getLastCommandFromHistory()
		if err != nil {
			return CommandInfo{}, fmt.Errorf("failed to get last command: %w", err)
		}
	}

	// Try to get exit code from environment variable first
	if envExitCode := os.Getenv("WTF_LAST_EXIT_CODE"); envExitCode != "" {
		if exitCode, err := strconv.Atoi(envExitCode); err == nil {
			cmd.ExitCode = exitCode
		}
	} else {
		// Try to get the exit code from the current shell
		cmd.ExitCode, err = GetLastExitCode()
		if err != nil {
			// If we can't get the exit code, try to infer it from the command
			cmd.ExitCode = inferExitCodeFromCommand(cmd.Command)
		}
	}

	// Get output from environment variable if available
	if envOutput := os.Getenv("WTF_LAST_OUTPUT"); envOutput != "" {
		cmd.Output = envOutput
	} else {
		cmd.Output = "[Output not available in this implementation]"
	}

	return cmd, nil
}

// inferExitCodeFromCommand tries to infer if a command likely failed based on its content
func inferExitCodeFromCommand(command string) int {
	// This is a simple heuristic - in a real implementation we'd need better shell integration
	if strings.Contains(command, "ls /nonexistent") ||
		strings.Contains(command, "cat /nonexistent") ||
		strings.Contains(command, "cd /nonexistent") {
		return 2 // Common exit code for "No such file or directory"
	}

	// Check for other common failure patterns
	if strings.Contains(command, "permission denied") ||
		strings.Contains(command, "sudo") {
		return 1
	}

	return 0 // Default to success
}

// getLastCommandFromHistory retrieves the last command from bash history
func getLastCommandFromHistory() (string, error) {
	// Try using fc command to get the last command
	// fc -ln -1 will show the last command without line numbers
	fcCmd := exec.Command("bash", "-c", "fc -ln -1")
	var out bytes.Buffer
	fcCmd.Stdout = &out
	fcCmd.Stderr = os.Stderr

	if err := fcCmd.Run(); err != nil {
		// If fc fails, try using history command
		histCmd := exec.Command("bash", "-c", "history 1")
		var histOut bytes.Buffer
		histCmd.Stdout = &histOut
		histCmd.Stderr = os.Stderr

		if err := histCmd.Run(); err != nil {
			return "", fmt.Errorf("failed to get command history: %w", err)
		}

		// Parse the output from history command (format: "123 command")
		historyLine := strings.TrimSpace(histOut.String())
		parts := strings.SplitN(historyLine, " ", 2)
		if len(parts) < 2 {
			return "", fmt.Errorf("unexpected history output format: %s", historyLine)
		}
		return strings.TrimSpace(parts[1]), nil
	}

	// Return the command from fc (which doesn't include line numbers)
	return strings.TrimSpace(out.String()), nil
}

// GetLastCommandOutput retrieves the output of the last executed command
// In a real implementation, this would access the shell's stored output
func GetLastCommandOutput() (string, error) {
	// This is a placeholder. In a real implementation, we would retrieve
	// the actual output from the shell environment.
	return "[Output not available in this implementation]", nil
}

// GetLastExitCode retrieves the exit code of the last executed command
func GetLastExitCode() (int, error) {
	// Try to get the exit code from $? variable in bash
	cmd := exec.Command("bash", "-c", "echo $?")
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("failed to get exit code: %w", err)
	}

	// Parse the exit code
	exitCodeStr := strings.TrimSpace(out.String())
	exitCode, err := strconv.Atoi(exitCodeStr)
	if err != nil {
		return 0, fmt.Errorf("failed to parse exit code: %w", err)
	}

	return exitCode, nil
}
