package ui

import (
	"regexp"
	"strings"
)

// DirectoryParser extracts the current directory from shell prompts
type DirectoryParser struct {
	lastDir string
}

// NewDirectoryParser creates a new directory parser
func NewDirectoryParser() *DirectoryParser {
	return &DirectoryParser{
		lastDir: "~",
	}
}

// ParseFromOutput extracts directory from shell prompt patterns
func (dp *DirectoryParser) ParseFromOutput(data []byte) {
	content := string(data)
	
	// Common bash prompt patterns:
	// user@host:~/path$
	// user@host:/full/path$
	
	// Pattern 1: user@host:path$ or user@host:path#
	promptRegex := regexp.MustCompile(`\w+@[\w-]+:(~?[/\w.-]+)[$#]`)
	matches := promptRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		dp.lastDir = matches[1]
		return
	}
	
	// Pattern 2: Look for PWD= in output
	pwdRegex := regexp.MustCompile(`PWD=([^\s;]+)`)
	matches = pwdRegex.FindStringSubmatch(content)
	if len(matches) > 1 {
		dp.lastDir = matches[1]
		// Replace home with ~
		if strings.HasPrefix(dp.lastDir, "/home/"+getUsername()) {
			dp.lastDir = strings.Replace(dp.lastDir, "/home/"+getUsername(), "~", 1)
		}
		return
	}
}

// GetDirectory returns the last parsed directory
func (dp *DirectoryParser) GetDirectory() string {
	if dp.lastDir == "" {
		return "~"
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
