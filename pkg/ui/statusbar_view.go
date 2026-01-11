package ui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// StatusBarView handles the status bar rendering with Lipgloss
type StatusBarView struct {
	currentDir string
	message    string
	width      int
}

// NewStatusBarView creates a new status bar view
func NewStatusBarView() *StatusBarView {
	return &StatusBarView{
		currentDir: getCurrentWorkingDir(),
		width:      80,
	}
}

// SetDirectory updates the current directory
func (s *StatusBarView) SetDirectory(dir string) {
	s.currentDir = dir
}

// SetMessage sets a temporary message
func (s *StatusBarView) SetMessage(msg string) {
	s.message = msg
}

// SetWidth updates the width for rendering
func (s *StatusBarView) SetWidth(width int) {
	s.width = width
}

// Render returns the styled status bar string
func (s *StatusBarView) Render() string {
	// Build content
	var content string
	if s.message != "" {
		content = fmt.Sprintf("[wtf_cli] %s", s.message)
	} else {
		content = fmt.Sprintf("[wtf_cli] %s | Press / for commands", s.currentDir)
	}

	// Truncate if too long
	maxWidth := s.width - 4
	if len(content) > maxWidth {
		content = content[:maxWidth-3] + "..."
	}

	// Pad to full width
	padding := s.width - lipgloss.Width(content)
	if padding > 0 {
		content = content + strings.Repeat(" ", padding)
	}

	// Apply beautiful styling with Lipgloss
	return statusStyle.Render(content)
}

// getCurrentWorkingDir gets the current directory with ~ substitution
func getCurrentWorkingDir() string {
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

// Lipgloss Styles

var (
	// Status bar with gradient background
	statusStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		Bold(true)

	// Alternative color schemes for different themes
	
	// Cyan/purple gradient style
	statusStyleCyan = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#00B8D4")).
		Padding(0, 1).
		Bold(true)
	
	// Dark subtle style
	statusStyleDark = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#D0D0D0")).
		Background(lipgloss.Color("#3C3C3C")).
		Padding(0, 1)
	
	// Highlight style for wtf_cli prefix
	prefixStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFD700")).
		Bold(true)
)

// SetTheme allows changing the status bar theme
func (s *StatusBarView) SetTheme(theme string) {
	switch theme {
	case "cyan":
		statusStyle = statusStyleCyan
	case "dark":
		statusStyle = statusStyleDark
	default:
		// Keep default purple
	}
}
