package ui

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"golang.org/x/term"
)

// StatusBar renders a status bar at the bottom of the terminal
type StatusBar struct {
	mu         sync.RWMutex
	currentDir string
	message    string
	model      string
	termWidth  int
	termHeight int
}

// NewStatusBar creates a new status bar
func NewStatusBar() *StatusBar {
	sb := &StatusBar{
		currentDir: getWorkingDir(),
	}
	sb.updateTerminalSize()
	return sb
}

// SetDirectory updates the current directory displayed
func (sb *StatusBar) SetDirectory(dir string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.currentDir = dir
}

// SetMessage sets a temporary message to display
func (sb *StatusBar) SetMessage(msg string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.message = msg
}

// SetModel updates the active model displayed.
func (sb *StatusBar) SetModel(model string) {
	sb.mu.Lock()
	defer sb.mu.Unlock()
	sb.model = strings.TrimSpace(model)
}

// Render draws the status bar at the bottom of the terminal
func (sb *StatusBar) Render() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	sb.updateTerminalSize()

	// Build status bar content
	var content string
	modelLabel := sb.model
	if modelLabel == "" {
		modelLabel = "unknown"
	}
	if sb.message != "" {
		content = fmt.Sprintf("[wtf_cli] %s | [llm]: %s", sb.message, modelLabel)
	} else {
		content = fmt.Sprintf("[wtf_cli] %s | [llm]: %s | Press / for commands", sb.currentDir, modelLabel)
	}

	// Truncate if too long
	maxWidth := sb.termWidth - 2
	if maxWidth < 10 {
		maxWidth = 10 // Minimum width
	}

	if len(content) > maxWidth {
		content = content[:maxWidth-3] + "..."
	}

	// Pad to full width
	padding := strings.Repeat(" ", sb.termWidth-len(content))

	// Build ANSI escape sequence for bottom bar
	// Save cursor, move to bottom, print with inverse colors, restore cursor
	return fmt.Sprintf(
		"\033[s\033[%d;1H\033[7m%s%s\033[0m\033[u",
		sb.termHeight,
		content,
		padding,
	)
}

// Clear removes the status bar from the display
func (sb *StatusBar) Clear() string {
	sb.mu.RLock()
	defer sb.mu.RUnlock()

	// Clear bottom line
	clearLine := strings.Repeat(" ", sb.termWidth)
	return fmt.Sprintf("\033[s\033[%d;1H%s\033[u", sb.termHeight, clearLine)
}

// updateTerminalSize refreshes the cached terminal dimensions
func (sb *StatusBar) updateTerminalSize() {
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fallback to defaults
		sb.termWidth = 80
		sb.termHeight = 24
		return
	}
	sb.termWidth = width
	sb.termHeight = height
}

// getWorkingDir returns the current working directory
func getWorkingDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "~"
	}

	// Replace home directory with ~
	home, err := os.UserHomeDir()
	if err == nil && strings.HasPrefix(dir, home) {
		dir = "~" + dir[len(home):]
	}

	return dir
}
