package statusbar

import (
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/charmbracelet/x/ansi"
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

	modelLabel := sb.model
	if modelLabel == "" {
		modelLabel = "unknown"
	}
	const (
		minGap         = 2
		contentPadding = 2
	)

	rightContent := fmt.Sprintf("[llm]: %s", modelLabel)
	if sb.message == "" {
		rightContent = fmt.Sprintf("[llm]: %s | Press / for commands", modelLabel)
	}
	rightWidth := ansi.StringWidth(rightContent)

	innerWidth := sb.termWidth - contentPadding
	if innerWidth < 0 {
		innerWidth = 0
	}

	innerContent := ""
	if innerWidth == 0 {
		innerContent = ""
	} else if rightWidth > innerWidth {
		innerContent = ansi.Truncate(rightContent, innerWidth, "")
	} else {
		leftText := sb.currentDir
		if sb.message != "" {
			leftText = sb.message
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

		innerContent = leftContent + strings.Repeat(" ", gap) + rightContent
	}

	if w := ansi.StringWidth(innerContent); w < innerWidth {
		innerContent += strings.Repeat(" ", innerWidth-w)
	} else if w > innerWidth && innerWidth > 0 {
		innerContent = ansi.Truncate(innerContent, innerWidth, "")
	}

	fullContent := innerContent
	if contentPadding == 2 && sb.termWidth >= 2 {
		fullContent = " " + innerContent + " "
	}
	if w := ansi.StringWidth(fullContent); w < sb.termWidth {
		fullContent += strings.Repeat(" ", sb.termWidth-w)
	}

	// Build ANSI escape sequence for bottom bar
	// Save cursor, move to bottom, print with inverse colors, restore cursor
	return fmt.Sprintf(
		"\033[s\033[%d;1H\033[7m%s\033[0m\033[u",
		sb.termHeight,
		fullContent,
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
