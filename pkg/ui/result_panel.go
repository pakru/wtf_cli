package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ResultPanel displays command execution results
type ResultPanel struct {
	title   string
	content string
	visible bool
	width   int
	height  int
	scrollY int
	lines   []string
}

// NewResultPanel creates a new result panel
func NewResultPanel() *ResultPanel {
	return &ResultPanel{}
}

// Show displays the result panel with content
func (rp *ResultPanel) Show(title, content string) {
	rp.title = title
	rp.content = content
	rp.visible = true
	rp.scrollY = 0
	rp.lines = strings.Split(content, "\n")
}

// Hide hides the result panel
func (rp *ResultPanel) Hide() {
	rp.visible = false
}

// IsVisible returns whether the panel is visible
func (rp *ResultPanel) IsVisible() bool {
	return rp.visible
}

// SetSize sets the panel dimensions
func (rp *ResultPanel) SetSize(width, height int) {
	rp.width = width
	rp.height = height
}

// resultPanelCloseMsg is sent when the result panel is closed
type resultPanelCloseMsg struct{}

// Update handles keyboard input for the result panel
func (rp *ResultPanel) Update(msg tea.KeyMsg) tea.Cmd {
	maxScroll := len(rp.lines) - (rp.height - 6) // Account for box borders
	if maxScroll < 0 {
		maxScroll = 0
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyEnter:
		// Close the panel
		rp.Hide()
		return func() tea.Msg {
			return resultPanelCloseMsg{}
		}

	case tea.KeyUp:
		if rp.scrollY > 0 {
			rp.scrollY--
		}
		return nil

	case tea.KeyDown:
		if rp.scrollY < maxScroll {
			rp.scrollY++
		}
		return nil

	case tea.KeyPgUp:
		rp.scrollY -= 10
		if rp.scrollY < 0 {
			rp.scrollY = 0
		}
		return nil

	case tea.KeyPgDown:
		rp.scrollY += 10
		if rp.scrollY > maxScroll {
			rp.scrollY = maxScroll
		}
		return nil
	}

	// 'q' also closes
	if msg.String() == "q" {
		rp.Hide()
		return func() tea.Msg {
			return resultPanelCloseMsg{}
		}
	}

	return nil
}

// View renders the result panel
func (rp *ResultPanel) View() string {
	if !rp.visible {
		return ""
	}

	// Calculate panel size - use most of the screen
	panelWidth := rp.width - 4
	if panelWidth > 80 {
		panelWidth = 80
	}
	panelHeight := rp.height - 4
	if panelHeight > 30 {
		panelHeight = 30
	}

	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("141")).
		Padding(1, 2).
		Width(panelWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")).
		Bold(true)

	contentStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	// Build content
	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(rp.title))
	sb.WriteString("\n\n")

	// Content with scrolling
	visibleLines := panelHeight - 8 // Account for title, footer, borders
	if visibleLines < 5 {
		visibleLines = 5
	}

	endLine := rp.scrollY + visibleLines
	if endLine > len(rp.lines) {
		endLine = len(rp.lines)
	}

	for i := rp.scrollY; i < endLine; i++ {
		sb.WriteString(contentStyle.Render(rp.lines[i]))
		sb.WriteString("\n")
	}

	// Scroll indicator
	if len(rp.lines) > visibleLines {
		sb.WriteString("\n")
		sb.WriteString(footerStyle.Render("↑↓ Scroll • "))
	}

	sb.WriteString(footerStyle.Render("Esc/q Close"))

	// Render box
	box := boxStyle.Render(sb.String())

	// Center the box
	boxWidth := lipgloss.Width(box)
	leftPad := (rp.width - boxWidth) / 2
	if leftPad > 0 {
		lines := strings.Split(box, "\n")
		for i, line := range lines {
			lines[i] = strings.Repeat(" ", leftPad) + line
		}
		box = strings.Join(lines, "\n")
	}

	return box
}
