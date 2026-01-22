package historypicker

import (
	"strings"

	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
)

// ShowHistoryPickerMsg is sent to trigger the history picker
type ShowHistoryPickerMsg struct {
	InitialFilter string
}

// HistoryPickerSelectMsg is sent when a command is selected
type HistoryPickerSelectMsg struct {
	Command string
}

// HistoryPickerCancelMsg is sent when picker is cancelled
type HistoryPickerCancelMsg struct{}

// HistoryPickerPanel provides a searchable TUI for command history
type HistoryPickerPanel struct {
	commands []string // All history commands
	filtered []string // Filtered commands based on search
	filter   string   // Current search filter
	selected int      // Currently selected index in filtered list
	scroll   int      // Scroll offset
	visible  bool     // Panel visibility
	width    int      // Panel dimensions
	height   int      // Panel dimensions
}

// NewHistoryPickerPanel creates a new history picker panel
func NewHistoryPickerPanel() *HistoryPickerPanel {
	return &HistoryPickerPanel{}
}

// Show displays the picker with commands and optional initial filter
func (hp *HistoryPickerPanel) Show(initialFilter string, commands []string) {
	hp.visible = true
	hp.commands = append([]string(nil), commands...) // Copy to avoid mutations
	hp.filter = initialFilter
	hp.selected = 0
	hp.scroll = 0
	hp.updateFiltered()
	hp.ensureVisible()
}

// Hide hides the picker
func (hp *HistoryPickerPanel) Hide() {
	hp.visible = false
}

// IsVisible returns whether the picker is visible
func (hp *HistoryPickerPanel) IsVisible() bool {
	return hp.visible
}

// SetSize updates the picker dimensions
func (hp *HistoryPickerPanel) SetSize(width, height int) {
	hp.width = width
	hp.height = height
}

// updateFiltered applies the current filter to commands
func (hp *HistoryPickerPanel) updateFiltered() {
	if hp.filter == "" {
		hp.filtered = hp.commands
		return
	}

	// Case-insensitive substring matching
	filterLower := strings.ToLower(hp.filter)
	hp.filtered = make([]string, 0)

	for _, cmd := range hp.commands {
		if strings.Contains(strings.ToLower(cmd), filterLower) {
			hp.filtered = append(hp.filtered, cmd)
		}
	}
}

// Update handles keyboard input for the picker
func (hp *HistoryPickerPanel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if !hp.visible {
		return nil
	}

	listHeight := hp.listHeight()
	keyStr := msg.String()

	switch keyStr {
	case "up":
		if hp.selected > 0 {
			hp.selected--
		}
		hp.ensureVisible()
		return nil

	case "down":
		if hp.selected < len(hp.filtered)-1 {
			hp.selected++
		}
		hp.ensureVisible()
		return nil

	case "pgup":
		if len(hp.filtered) > 0 {
			hp.selected -= listHeight
			if hp.selected < 0 {
				hp.selected = 0
			}
			hp.ensureVisible()
		}
		return nil

	case "pgdown":
		if len(hp.filtered) > 0 {
			hp.selected += listHeight
			if hp.selected > len(hp.filtered)-1 {
				hp.selected = len(hp.filtered) - 1
			}
			hp.ensureVisible()
		}
		return nil

	case "home":
		if len(hp.filtered) > 0 {
			hp.selected = 0
			hp.ensureVisible()
		}
		return nil

	case "end":
		if len(hp.filtered) > 0 {
			hp.selected = len(hp.filtered) - 1
			hp.ensureVisible()
		}
		return nil

	case "enter", "tab":
		if len(hp.filtered) > 0 && hp.selected >= 0 && hp.selected < len(hp.filtered) {
			cmd := hp.filtered[hp.selected]
			hp.Hide()
			return func() tea.Msg {
				return HistoryPickerSelectMsg{Command: cmd}
			}
		}
		return nil

	case "esc":
		hp.Hide()
		return func() tea.Msg {
			return HistoryPickerCancelMsg{}
		}

	case "backspace":
		// Delete filter character
		if len(hp.filter) > 0 {
			hp.filter = hp.filter[:len(hp.filter)-1]
			hp.updateFiltered()
			hp.selected = 0
			hp.ensureVisible()
		}
		return nil

	case "ctrl+u":
		// Clear entire filter
		if hp.filter != "" {
			hp.filter = ""
			hp.updateFiltered()
			hp.selected = 0
			hp.ensureVisible()
		}
		return nil

	default:
		// Add to filter if it's printable text
		key := msg.Key()
		if key.Text != "" {
			hp.filter += key.Text
			hp.updateFiltered()
			hp.selected = 0
			hp.ensureVisible()
		}
		return nil
	}
}

// View renders the picker
func (hp *HistoryPickerPanel) View() string {
	if !hp.visible {
		return ""
	}

	boxWidth, contentWidth, listHeight := hp.dimensions()

	boxStyle := styles.BoxStyle.Width(boxWidth)
	titleStyle := styles.TitleStyle
	normalStyle := styles.TextStyle
	selectedStyle := styles.SelectedStyle
	descStyle := styles.TextMutedStyle
	filterStyle := styles.FilterStyle
	footerStyle := styles.FooterStyle

	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("🔍 Command History Search"))
	content.WriteString("\n")

	// Filter input
	if hp.filter != "" {
		content.WriteString(filterStyle.Render("Filter: " + hp.filter))
	} else {
		content.WriteString(descStyle.Render("Type to search..."))
	}
	content.WriteString("\n\n")

	// Command list
	if len(hp.filtered) == 0 {
		if hp.filter != "" {
			content.WriteString(descStyle.Render("No matching commands"))
		} else {
			content.WriteString(descStyle.Render("No commands in history"))
		}
		for i := 1; i < listHeight; i++ {
			content.WriteString("\n")
		}
	} else {
		for i := 0; i < listHeight; i++ {
			index := hp.scroll + i
			if index >= len(hp.filtered) {
				content.WriteString("\n")
				continue
			}
			cmd := hp.filtered[index]

			// Truncate long commands
			if len(cmd) > contentWidth-4 {
				cmd = cmd[:contentWidth-7] + "..."
			}

			line := "  " + cmd
			if index == hp.selected {
				line = utils.PadPlain(line, contentWidth)
				content.WriteString(selectedStyle.Render(line))
			} else {
				content.WriteString(normalStyle.Render(line))
			}
			content.WriteString("\n")
		}
	}

	// Footer with controls
	content.WriteString("\n")
	footerText := "↑↓ Navigate | Tab/Enter Select | Esc Cancel | Ctrl+U Clear"
	if len(hp.filtered) > listHeight {
		footerText = "↑↓ Navigate | PgUp/PgDn Scroll | Tab/Enter Select | Esc Cancel"
	}
	content.WriteString(footerStyle.Render(footerText))

	return boxStyle.Render(content.String())
}

// ensureVisible adjusts scroll to keep selected item visible
func (hp *HistoryPickerPanel) ensureVisible() {
	listHeight := hp.listHeight()

	if len(hp.filtered) == 0 {
		hp.selected = 0
		hp.scroll = 0
		return
	}

	if hp.selected < 0 {
		hp.selected = 0
	}
	if hp.selected >= len(hp.filtered) {
		hp.selected = len(hp.filtered) - 1
	}

	maxScroll := len(hp.filtered) - listHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if hp.scroll > maxScroll {
		hp.scroll = maxScroll
	}

	if hp.selected < hp.scroll {
		hp.scroll = hp.selected
	}
	if hp.selected >= hp.scroll+listHeight {
		hp.scroll = hp.selected - listHeight + 1
	}
	if hp.scroll < 0 {
		hp.scroll = 0
	}
}

// dimensions calculates box width, content width, and list height
func (hp *HistoryPickerPanel) dimensions() (int, int, int) {
	width := hp.width
	height := hp.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	available := width - 2
	if available < 1 {
		available = 1
	}

	boxWidth := available
	if boxWidth > 100 {
		boxWidth = 100
	}
	minWidth := 50
	if minWidth > available {
		minWidth = available
	}
	if boxWidth < minWidth {
		boxWidth = minWidth
	}

	contentWidth := boxWidth - 4
	if contentWidth < 1 {
		contentWidth = 1
	}

	maxContentHeight := height - 4
	if maxContentHeight < 1 {
		maxContentHeight = 1
	}

	// Fixed lines: title + filter + blank + footer
	const fixedLines = 5
	listHeight := maxContentHeight - fixedLines
	if listHeight < 1 {
		listHeight = 1
	}

	// Cap height to show max ~12 items for a more compact dialog
	const maxListHeight = 12
	if listHeight > maxListHeight {
		listHeight = maxListHeight
	}

	return boxWidth, contentWidth, listHeight
}

// listHeight returns the calculated list height
func (hp *HistoryPickerPanel) listHeight() int {
	_, _, listHeight := hp.dimensions()
	return listHeight
}
