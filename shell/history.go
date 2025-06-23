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

	// Get the last command from bash history
	cmd.Command, err = getLastCommandFromHistory()
	if err != nil {
		return CommandInfo{}, fmt.Errorf("failed to get last command: %w", err)
	}

	// In a real implementation, we would retrieve the actual output and exit code
	// from the shell environment. For now, we'll use placeholder values.
	// This will be implemented with proper shell integration.
	cmd.Output = "[Output not available in this implementation]"
	cmd.ExitCode = 0

	return cmd, nil
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
