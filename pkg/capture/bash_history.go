package capture

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// ReadBashHistory reads commands from the bash history file.
// It uses $HISTFILE environment variable, falling back to ~/.bash_history.
// Returns up to maxLines commands in reverse chronological order (most recent first).
func ReadBashHistory(maxLines int) ([]string, error) {
	histFile := os.Getenv("HISTFILE")
	if histFile == "" {
		// Fallback to default bash history location
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		histFile = filepath.Join(homeDir, ".bash_history")
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
