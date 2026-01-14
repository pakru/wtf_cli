package ui

import (
	"regexp"
	"strings"
)

// DirectoryParser extracts the current directory from shell prompts
type DirectoryParser struct {
	lastDir string
	parsed  bool // true if we've actually parsed a directory
}

// NewDirectoryParser creates a new directory parser
func NewDirectoryParser() *DirectoryParser {
	return &DirectoryParser{
		lastDir: "",
		parsed:  false,
	}
}

// ParseFromOutput extracts directory from shell prompt patterns
func (dp *DirectoryParser) ParseFromOutput(data []byte) {
	content := string(data)

	// Common bash prompt patterns:
	// user@host:~/path$
	// user@host:/full/path$
	// user@host:~/path (git-branch)$
	// user@host:~/path (git-branch) $

	// Pattern 1: user@host:path with optional (branch) before $ or #
	// Matches: pavel@host:~/projects (main) $
	// Matches: pavel@host:~/projects$
	promptRegex := regexp.MustCompile(`\w+@[\w-]+:(~?[/\w.-]+)(?:\s*\([^)]*\))?\s*[$#]`)
	matches := promptRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		dp.lastDir = matches[1]
		dp.parsed = true
		return
	}

	// Pattern 2: Just the path with $ or # at end (simpler prompts)
	simpleRegex := regexp.MustCompile(`(~?/[\w/.-]+)\s*[$#]\s*$`)
	matches = simpleRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		dp.lastDir = matches[1]
		dp.parsed = true
		return
	}

	// Pattern 3: Look for PWD= in output (from env or echo $PWD)
	pwdRegex := regexp.MustCompile(`PWD=([^\s;]+)`)
	matches = pwdRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		dp.lastDir = matches[1]
		dp.parsed = true
		// Replace home with ~
		if strings.HasPrefix(dp.lastDir, "/home/"+getUsername()) {
			dp.lastDir = strings.Replace(dp.lastDir, "/home/"+getUsername(), "~", 1)
		}
		return
	}
}

// GetDirectory returns the last parsed directory, or empty if not parsed
func (dp *DirectoryParser) GetDirectory() string {
	if !dp.parsed {
		return "" // Return empty if nothing parsed yet
	}
	return dp.lastDir
}

func getUsername() string {
	// Simple helper to get username for home replacement
	// Could use os.Getenv("USER") but keeping it simple
	parts := strings.Split(getCurrentWorkingDir(), "/")
	if len(parts) >= 3 && parts[1] == "home" {
		return parts[2]
	}
	return ""
}
