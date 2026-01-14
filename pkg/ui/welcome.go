package ui

import (
	"github.com/charmbracelet/lipgloss"
)

// WelcomeBanner creates the welcome message box
type WelcomeBanner struct {
	width   int
	visible bool
}

// NewWelcomeBanner creates a new welcome banner
func NewWelcomeBanner() *WelcomeBanner {
	return &WelcomeBanner{
		visible: true,
	}
}

// SetWidth sets the banner width
func (wb *WelcomeBanner) SetWidth(width int) {
	wb.width = width
}

// Hide hides the banner
func (wb *WelcomeBanner) Hide() {
	wb.visible = false
}

// IsVisible returns whether the banner is visible
func (wb *WelcomeBanner) IsVisible() bool {
	return wb.visible
}

// View renders the welcome banner
func (wb *WelcomeBanner) View() string {
	if !wb.visible {
		return ""
	}

	// Banner content
	title := "  ▄█     █▄      ▄█      ▄████████     ▄████████  ▄█        ▄█  "
	line2 := " ███     ███    ███     ███    ███    ███    ███ ███       ███  "
	line3 := " ███     ███    ███▌    ███    █▀     ███    █▀  ███       ███▌ "
	line4 := " ███     ███    ███▌   ▄███▄▄▄        ███        ███       ███▌ "
	line5 := " ███     ███    ███▌  ▀▀███▀▀▀        ███        ███       ███▌ "
	line6 := " ███     ███    ███     ███           ███    █▄  ███       ███  "
	line7 := " ███ ▄█▄ ███    ███     ███           ███    ███ ███▌    ▄ ███  "
	line8 := "  ▀███▀███▀     █▀      ███            ▀█████████▀█████▄▄██ █▀   "

	shortcuts := []string{
		"",
		"╭─────────────────────────────────────────────────────────╮",
		"│              Welcome to WTF CLI Terminal                │",
		"│                                                         │",
		"│  Shortcuts:                                             │",
		"│    Ctrl+D     Exit terminal                             │",
		"│    Ctrl+C     Cancel current command                    │",
		"│    Ctrl+Z     Suspend process                           │",
		"│    /          Command palette (coming soon)             │",
		"│                                                         │",
		"│  Type any command to get started!                       │",
		"╰─────────────────────────────────────────────────────────╯",
		"",
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")). // Purple
		Bold(true)

	boxStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")) // Gray

	// Build the banner
	var content string
	
	// Add ASCII art title
	content += titleStyle.Render(title) + "\n"
	content += titleStyle.Render(line2) + "\n"
	content += titleStyle.Render(line3) + "\n"
	content += titleStyle.Render(line4) + "\n"
	content += titleStyle.Render(line5) + "\n"
	content += titleStyle.Render(line6) + "\n"
	content += titleStyle.Render(line7) + "\n"
	content += titleStyle.Render(line8) + "\n"
	
	// Add shortcuts box
	for _, line := range shortcuts {
		content += boxStyle.Render(line) + "\n"
	}

	return content
}

// Height returns the height of the banner in lines
func (wb *WelcomeBanner) Height() int {
	if !wb.visible {
		return 0
	}
	return 21 // 8 lines for ASCII art + 13 lines for shortcuts box
}
