package shell

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// CommandInfo stores information about the last executed command
type CommandInfo struct {
	Command   string      // The command that was executed
	Output    string      // Combined stdout and stderr
	ExitCode  int         // Exit code of the command
	PipeInput string      // NEW: Input received via pipe
	Source    InputSource // NEW: How the data was obtained
}

// InputSource represents the source of command information
type InputSource int

const (
	SourceShellIntegration InputSource = iota
	SourceHistory
	SourceEnvironment
	SourcePipe
)

// GetLastCommand retrieves information about the last executed command
func GetLastCommand() (CommandInfo, error) {
	// First priority: Try shell integration (JSON file)
	if shellCmd, err := getCommandFromShellIntegration(); err == nil {
		return shellCmd, nil
	}

	cmd := CommandInfo{}
	var err error

	// Second priority: Environment variables (for testing)
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
			// If we can't get the exit code, assume this command returned unknown (-1)
			cmd.ExitCode = -1
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

// getLastCommandFromHistory retrieves the last command from bash history
func getLastCommandFromHistory() (string, error) {
	// Method 1: Try to read from bash history file directly
	if cmd, err := getCommandFromHistoryFile(); err == nil && cmd != "" {
		return cmd, nil
	}

	// Method 2: Try using fc command with interactive shell
	if cmd, err := getCommandWithFC(); err == nil && cmd != "" {
		return cmd, nil
	}

	// Method 3: Try using history command
	if cmd, err := getCommandWithHistory(); err == nil && cmd != "" {
		return cmd, nil
	}

	// Method 4: Try reading from shell environment variables
	if cmd := os.Getenv("HISTCMD_LAST"); cmd != "" {
		return cmd, nil
	}

	// If all methods fail, return empty string (not an error)
	// This allows the tool to continue working with environment variable overrides
	return "", nil
}

// getCommandFromHistoryFile tries to read the last command from bash history file
func getCommandFromHistoryFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	historyFile := filepath.Join(homeDir, ".bash_history")
	data, err := os.ReadFile(historyFile)
	if err != nil {
		return "", err
	}

	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(lines) == 0 {
		return "", fmt.Errorf("empty history file")
	}

	// Get the last non-empty line
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "#") {
			return line, nil
		}
	}

	return "", fmt.Errorf("no valid commands found in history")
}

// getCommandWithFC tries to use fc command
func getCommandWithFC() (string, error) {
	// Try with different approaches to access interactive shell history
	commands := []string{
		"fc -ln -1",
		"set -o history; fc -ln -1",
		"bash -i -c 'fc -ln -1'",
	}

	for _, cmdStr := range commands {
		cmd := exec.Command("bash", "-c", cmdStr)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = nil // Suppress errors

		if err := cmd.Run(); err == nil {
			result := strings.TrimSpace(out.String())
			if result != "" {
				return result, nil
			}
		}
	}

	return "", fmt.Errorf("fc command failed")
}

// getCommandWithHistory tries to use history command
func getCommandWithHistory() (string, error) {
	commands := []string{
		"history 1",
		"bash -i -c 'history 1'",
		"set -o history; history 1",
	}

	for _, cmdStr := range commands {
		cmd := exec.Command("bash", "-c", cmdStr)
		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = nil // Suppress errors

		if err := cmd.Run(); err == nil {
			historyLine := strings.TrimSpace(out.String())
			if historyLine != "" {
				// Parse the output from history command (format: "123 command")
				parts := strings.SplitN(historyLine, " ", 2)
				if len(parts) >= 2 {
					return strings.TrimSpace(parts[1]), nil
				}
			}
		}
	}

	return "", fmt.Errorf("history command failed")
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

// ShellIntegrationData represents the JSON structure from shell integration
type ShellIntegrationData struct {
	Command   string  `json:"command"`
	ExitCode  int     `json:"exit_code"`
	StartTime string  `json:"start_time"`
	EndTime   string  `json:"end_time"`
	Duration  float64 `json:"duration"`
	PWD       string  `json:"pwd"`
	Timestamp string  `json:"timestamp"`
}

// getCommandFromShellIntegration reads command info from shell integration JSON file
func getCommandFromShellIntegration() (CommandInfo, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return CommandInfo{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	commandFile := filepath.Join(homeDir, ".wtf", "last_command.json")
	data, err := os.ReadFile(commandFile)
	if err != nil {
		return CommandInfo{}, fmt.Errorf("failed to read command file: %w", err)
	}

	var shellData ShellIntegrationData
	if err := json.Unmarshal(data, &shellData); err != nil {
		return CommandInfo{}, fmt.Errorf("failed to parse command JSON: %w", err)
	}

	cmd := CommandInfo{
		Command:  shellData.Command,
		ExitCode: shellData.ExitCode,
	}

	return cmd, nil
}

// IsShellIntegrationActive checks if shell integration is active by looking for the command file
func IsShellIntegrationActive() bool {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return false
	}

	commandFile := filepath.Join(homeDir, ".wtf", "last_command.json")
	_, err = os.Stat(commandFile)
	return err == nil
}

// GetPipeCommandInfo retrieves information about the original command when WTF is used in a pipe
func GetPipeCommandInfo() (CommandInfo, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return CommandInfo{}, fmt.Errorf("failed to get home directory: %w", err)
	}

	commandFile := filepath.Join(homeDir, ".wtf", "last_command.json")
	data, err := os.ReadFile(commandFile)
	if err != nil {
		return CommandInfo{}, fmt.Errorf("failed to read command file: %w", err)
	}

	var shellData ShellIntegrationData
	if err := json.Unmarshal(data, &shellData); err != nil {
		return CommandInfo{}, fmt.Errorf("failed to parse command JSON: %w", err)
	}

	// Always return error since we no longer track pipe commands
	return CommandInfo{}, fmt.Errorf("pipe command tracking disabled")
}
