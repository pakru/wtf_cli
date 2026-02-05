package capture

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// detectShell determines which shell is being used based on $SHELL and OS platform.
// Returns "zsh", "bash", or "unknown".
func detectShell() string {
	// Check $SHELL environment variable first
	shell := os.Getenv("SHELL")
	if strings.HasSuffix(shell, "/zsh") || strings.HasSuffix(shell, "\\zsh") {
		return "zsh"
	}
	if strings.HasSuffix(shell, "/bash") || strings.HasSuffix(shell, "\\bash") {
		return "bash"
	}

	// Fall back to OS platform detection
	// macOS defaults to zsh (since Catalina), Linux typically uses bash
	if runtime.GOOS == "darwin" {
		return "zsh"
	}

	return "bash" // default fallback
}

// ReadBashHistory reads commands from the shell history file.
// It supports both bash and zsh history formats.
// It uses $HISTFILE environment variable, falling back to shell-specific defaults.
// Returns up to maxLines commands in reverse chronological order (most recent first).
func ReadBashHistory(maxLines int) ([]string, error) {
	histFile := os.Getenv("HISTFILE")
	if histFile == "" {
		// Detect shell and use appropriate default history file
		shell := detectShell()
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}

		if shell == "zsh" {
			histFile = filepath.Join(homeDir, ".zsh_history")
		} else {
			histFile = filepath.Join(homeDir, ".bash_history")
		}
	}

	file, err := os.Open(histFile)
	if err != nil {
		// If history file doesn't exist, return empty list (not an error)
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()

		// Skip bash timestamps (lines starting with #)
		if strings.HasPrefix(line, "#") {
			continue
		}

		// Handle zsh extended history format: ": timestamp:0;command"
		if strings.HasPrefix(line, ": ") {
			// Find the semicolon that separates timestamp from command
			if idx := strings.Index(line, ";"); idx != -1 {
				line = line[idx+1:]
			} else {
				// Malformed zsh history line, skip it
				continue
			}
		}

		// Skip empty lines
		if strings.TrimSpace(line) == "" {
			continue
		}

		lines = append(lines, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	// Reverse the order so most recent is first
	reversed := make([]string, len(lines))
	for i, line := range lines {
		reversed[len(lines)-1-i] = line
	}

	// Limit to maxLines
	if maxLines > 0 && len(reversed) > maxLines {
		reversed = reversed[:maxLines]
	}

	return reversed, nil
}

// MergeHistory combines bash history with session history, deduplicating entries.
// Session history takes precedence (appears first). Most recent items are at the beginning.
func MergeHistory(bashHistory []string, sessionHistory []CommandRecord) []string {
	// Use a map to track seen commands (for deduplication)
	seen := make(map[string]bool)
	var result []string

	// First, add session history (most recent session commands)
	for i := len(sessionHistory) - 1; i >= 0; i-- {
		cmd := strings.TrimSpace(sessionHistory[i].Command)
		if cmd != "" && !seen[cmd] {
			result = append(result, cmd)
			seen[cmd] = true
		}
	}

	// Then add bash history (already in reverse chronological order)
	for _, cmd := range bashHistory {
		cmd = strings.TrimSpace(cmd)
		if cmd != "" && !seen[cmd] {
			result = append(result, cmd)
			seen[cmd] = true
		}
	}

	return result
}
