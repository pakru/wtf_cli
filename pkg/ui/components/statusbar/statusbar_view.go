package statusbar

import (
	"fmt"
	"os"
	"strings"

	"wtf_cli/pkg/ui/styles"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// StatusBarView handles the status bar rendering with Lipgloss
type StatusBarView struct {
	currentDir  string
	message     string
	model       string
	width       int
	statusStyle lipgloss.Style
}

// NewStatusBarView creates a new status bar view
func NewStatusBarView() *StatusBarView {
	return &StatusBarView{
		currentDir:  getCurrentWorkingDir(),
		width:       80,
		statusStyle: styles.StatusBarStyle,
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

// GetMessage returns the current message
func (s *StatusBarView) GetMessage() string {
	return s.message
}

// SetModel updates the active model displayed.
func (s *StatusBarView) SetModel(model string) {
	s.model = strings.TrimSpace(model)
}

// SetWidth updates the width for rendering
func (s *StatusBarView) SetWidth(width int) {
	s.width = width
}

// Render returns the styled status bar string
func (s *StatusBarView) Render() string {
	modelLabel := s.model
	if modelLabel == "" {
		modelLabel = "unknown"
	}
	const (
		minGap         = 2
		contentPadding = 2
	)

	rightContent := fmt.Sprintf("[llm]: %s", modelLabel)
	if s.message == "" {
		rightContent = fmt.Sprintf("[llm]: %s | Press / for commands", modelLabel)
	}
	rightWidth := ansi.StringWidth(rightContent)

	innerWidth := s.width - contentPadding
	if innerWidth < 0 {
		innerWidth = 0
	}

	if innerWidth == 0 {
		return s.statusStyle.Width(s.width).Render("")
	}

	leftText := s.currentDir
	if s.message != "" {
		leftText = s.message
	}

	leftPrefix := "[wtf_cli]"
	leftContent := leftPrefix

	leftAvailable := innerWidth - rightWidth - minGap
	if leftAvailable < 0 {
		leftAvailable = 0
	}

	prefixWidth := ansi.StringWidth(leftPrefix)
	if leftAvailable >= prefixWidth+1 {
		bodyWidth := leftAvailable - prefixWidth - 1
		if bodyWidth > 0 && leftText != "" {
			truncated := truncatePath(leftText, bodyWidth)
			if truncated != "" {
				leftContent = leftPrefix + " " + truncated
			}
		}
	} else if leftAvailable < prefixWidth {
		leftContent = ansi.Truncate(leftPrefix, leftAvailable, "")
	}

	leftWidth := ansi.StringWidth(leftContent)
	gap := innerWidth - leftWidth - rightWidth
	if gap < 0 {
		gap = 0
	}
	if gap < minGap && leftWidth > 0 {
		allowedLeft := innerWidth - rightWidth - minGap
		if allowedLeft < 0 {
			allowedLeft = 0
		}
		leftContent = ansi.Truncate(leftContent, allowedLeft, "")
		leftWidth = ansi.StringWidth(leftContent)
		gap = innerWidth - leftWidth - rightWidth
		if gap < 0 {
			gap = 0
		}
	}

	if rightWidth > innerWidth {
		rightContent = ansi.Truncate(rightContent, innerWidth, "")
		rightWidth = ansi.StringWidth(rightContent)
		leftContent = ""
		gap = innerWidth - rightWidth
		if gap < 0 {
			gap = 0
		}
	}

	fullContent := leftContent + strings.Repeat(" ", gap) + rightContent
	return s.statusStyle.Width(s.width).Render(fullContent)
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

// SetTheme allows changing the status bar theme
func (s *StatusBarView) SetTheme(theme string) {
	switch theme {
	case "cyan":
		s.statusStyle = styles.StatusBarStyleCyan
	case "dark":
		s.statusStyle = styles.StatusBarStyleDark
	default:
		s.statusStyle = styles.StatusBarStyle
	}
}
