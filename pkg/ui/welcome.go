package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// WelcomeMessage returns the welcome box string to print to PTY
func WelcomeMessage() string {
	lines := []string{
		"",
		"╭─────────────────────────────────────────────────────────╮",
		"│              Welcome to WTF CLI Terminal                │",
		"│                                                         │",
		"│  Shortcuts:                                             │",
		"│    Ctrl+D     Exit terminal (press twice)               │",
		"│    Ctrl+C     Cancel current command                    │",
		"│    Ctrl+Z     Suspend process                           │",
		"│    /          Command palette (coming soon)             │",
		"│                                                         │",
		"│  Type any command to get started!                       │",
		"╰─────────────────────────────────────────────────────────╯",
		"",
	}

	// Style with purple color
	boxStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")) // Purple

	var result string
	for _, line := range lines {
		result += boxStyle.Render(line) + "\n"
	}

	return result
}
