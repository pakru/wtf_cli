package result

import (
	"strings"

	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
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

// SetContent updates the panel content without resetting visibility.
func (rp *ResultPanel) SetContent(content string) {
	rp.content = content
	rp.lines = strings.Split(content, "\n")
	if rp.scrollY >= len(rp.lines) {
		if len(rp.lines) > 0 {
			rp.scrollY = len(rp.lines) - 1
		} else {
			rp.scrollY = 0
		}
	}
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

// ResultPanelCloseMsg is sent when the result panel is closed
type ResultPanelCloseMsg struct{}

// Update handles keyboard input for the result panel
func (rp *ResultPanel) Update(msg tea.KeyPressMsg) tea.Cmd {
	maxScroll := len(rp.lines) - (rp.height - 6) // Account for box borders
	if maxScroll < 0 {
		maxScroll = 0
	}

	keyStr := msg.String()
	switch keyStr {
	case "esc", "enter":
		// Close the panel
		rp.Hide()
		return func() tea.Msg {
			return ResultPanelCloseMsg{}
		}

	case "up":
		if rp.scrollY > 0 {
			rp.scrollY--
		}
		return nil

	case "down":
		if rp.scrollY < maxScroll {
			rp.scrollY++
		}
		return nil

	case "pgup":
		rp.scrollY -= 10
		if rp.scrollY < 0 {
			rp.scrollY = 0
		}
		return nil

	case "pgdown":
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
			return ResultPanelCloseMsg{}
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
	boxStyle := styles.BoxStyle.Width(panelWidth)
	titleStyle := styles.TitleStyle
	contentStyle := styles.TextStyle
	footerStyle := styles.FooterStyle

	contentWidth := panelWidth - 6
	if contentWidth < 1 {
		contentWidth = 1
	}

	// Build content
	var sb strings.Builder

	// Title
	sb.WriteString(titleStyle.Render(utils.TruncateToWidth(rp.title, contentWidth)))
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
		line := utils.TruncateToWidth(rp.lines[i], contentWidth)
		sb.WriteString(contentStyle.Render(line))
		sb.WriteString("\n")
	}

	// Scroll indicator
	if len(rp.lines) > visibleLines {
		sb.WriteString(footerStyle.Render("↑↓ Scroll • "))
	}

	sb.WriteString(footerStyle.Render("Esc/q Close"))

	// Render box
	box := boxStyle.Render(sb.String())

	return box
}
