package picker

import (
	"strings"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type OpenModelPickerMsg struct {
	Options []ai.ModelInfo
	Current string
	APIURL  string
}

type ModelPickerSelectMsg struct {
	ModelID string
}

type ModelPickerRefreshMsg struct {
	Cache ai.ModelCache
	Err   error
}

// ModelPickerPanel provides a searchable list of models.
type ModelPickerPanel struct {
	options  []ai.ModelInfo
	filter   string
	selected int
	scroll   int
	visible  bool
	width    int
	height   int
	current  string
}

// NewModelPickerPanel creates a new model picker panel.
func NewModelPickerPanel() *ModelPickerPanel {
	return &ModelPickerPanel{}
}

// Show displays the model picker with available options.
func (p *ModelPickerPanel) Show(options []ai.ModelInfo, current string) {
	p.visible = true
	p.filter = ""
	p.selected = 0
	p.scroll = 0
	p.options = append([]ai.ModelInfo(nil), options...)
	p.current = current

	if current != "" {
		for i, option := range p.options {
			if option.ID == current {
				p.selected = i
				break
			}
		}
	}

	p.ensureVisible(p.filteredOptions(), p.listHeight())
}

// UpdateOptions refreshes the picker list while preserving filter and selection.
func (p *ModelPickerPanel) UpdateOptions(options []ai.ModelInfo) {
	selectedID := ""
	filtered := p.filteredOptions()
	if len(filtered) > 0 && p.selected >= 0 && p.selected < len(filtered) {
		selectedID = filtered[p.selected].ID
	}
	if selectedID == "" {
		selectedID = p.current
	}

	p.options = append([]ai.ModelInfo(nil), options...)

	filtered = p.filteredOptions()
	p.selected = 0
	if selectedID != "" {
		for i, option := range filtered {
			if option.ID == selectedID {
				p.selected = i
				break
			}
		}
	}
	p.ensureVisible(filtered, p.listHeight())
}

// Hide hides the model picker.
func (p *ModelPickerPanel) Hide() {
	p.visible = false
}

// IsVisible reports whether the picker is visible.
func (p *ModelPickerPanel) IsVisible() bool {
	return p.visible
}

// SetSize updates the picker dimensions.
func (p *ModelPickerPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Update handles keyboard input for the picker.
func (p *ModelPickerPanel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if !p.visible {
		return nil
	}

	filtered := p.filteredOptions()
	listHeight := p.listHeight()
	keyStr := msg.String()

	switch keyStr {
	case "up":
		if p.selected > 0 {
			p.selected--
		}
		p.ensureVisible(filtered, listHeight)
		return nil

	case "down":
		if p.selected < len(filtered)-1 {
			p.selected++
		}
		p.ensureVisible(filtered, listHeight)
		return nil

	case "pgup":
		if len(filtered) > 0 {
			p.selected -= listHeight
			if p.selected < 0 {
				p.selected = 0
			}
			p.ensureVisible(filtered, listHeight)
		}
		return nil

	case "pgdown":
		if len(filtered) > 0 {
			p.selected += listHeight
			if p.selected > len(filtered)-1 {
				p.selected = len(filtered) - 1
			}
			p.ensureVisible(filtered, listHeight)
		}
		return nil

	case "home":
		if len(filtered) > 0 {
			p.selected = 0
			p.ensureVisible(filtered, listHeight)
		}
		return nil

	case "end":
		if len(filtered) > 0 {
			p.selected = len(filtered) - 1
			p.ensureVisible(filtered, listHeight)
		}
		return nil

	case "enter":
		if len(filtered) > 0 && p.selected < len(filtered) {
			modelID := filtered[p.selected].ID
			p.Hide()
			return func() tea.Msg {
				return ModelPickerSelectMsg{ModelID: modelID}
			}
		}
		return nil

	case "esc":
		p.Hide()
		return nil

	case "backspace":
		if len(p.filter) > 0 {
			p.filter = p.filter[:len(p.filter)-1]
			p.selected = 0
			p.scroll = 0
		}
		return nil

	default:
		// Add to filter if it's printable text
		key := msg.Key()
		if key.Text != "" {
			p.filter += key.Text
			p.selected = 0
			p.scroll = 0
		}
		return nil
	}
}

// View renders the model picker.
func (p *ModelPickerPanel) View() string {
	if !p.visible {
		return ""
	}

	boxWidth, contentWidth, listHeight := p.dimensions()

	boxStyle := styles.BoxStyle.Width(boxWidth)
	titleStyle := styles.TitleStyle
	normalStyle := styles.TextStyle
	selectedStyle := styles.SelectedStyle
	descStyle := styles.TextMutedStyle
	filterStyle := styles.FilterStyle
	placeholderStyle := styles.PlaceholderStyle
	footerStyle := styles.FooterStyle

	var content strings.Builder

	content.WriteString(titleStyle.Render("Model Picker"))
	content.WriteString("\n")

	if strings.TrimSpace(p.filter) == "" {
		content.WriteString(descStyle.Render("Search: "))
		content.WriteString(placeholderStyle.Render("type to filter"))
	} else {
		content.WriteString(descStyle.Render("Search: "))
		content.WriteString(filterStyle.Render(p.filter))
	}
	content.WriteString("\n\n")

	filtered := p.filteredOptions()
	if len(filtered) == 0 {
		content.WriteString(descStyle.Render("No matching models"))
		for i := 1; i < listHeight; i++ {
			content.WriteString("\n")
		}
	} else {
		maxLabelWidth := p.maxLabelWidth(filtered, contentWidth)
		descWidth := contentWidth - 2 - maxLabelWidth - 1
		if descWidth < 0 {
			descWidth = 0
		}

		for i := 0; i < listHeight; i++ {
			index := p.scroll + i
			if index >= len(filtered) {
				content.WriteString("\n")
				continue
			}
			option := filtered[index]
			label := utils.TruncateToWidth(modelOptionLabel(option), maxLabelWidth)
			labelPadding := maxLabelWidth - lipgloss.Width(label)
			if labelPadding < 0 {
				labelPadding = 0
			}
			labelText := label + strings.Repeat(" ", labelPadding)

			desc := ""
			if descWidth > 0 {
				desc = utils.TruncateToWidth(modelOptionDesc(option), descWidth)
			}

			if index == p.selected {
				line := "  " + labelText
				if desc != "" {
					line += " " + desc
				}
				line = utils.PadPlain(line, contentWidth)
				content.WriteString(selectedStyle.Render(line))
			} else {
				line := normalStyle.Render("  " + labelText)
				if desc != "" {
					line += " " + descStyle.Render(desc)
				}
				content.WriteString(line)
			}
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(footerStyle.Render("Up/Down Navigate | Enter Select | Esc Cancel"))

	return boxStyle.Render(content.String())
}

func (p *ModelPickerPanel) filteredOptions() []ai.ModelInfo {
	if strings.TrimSpace(p.filter) == "" {
		return p.options
	}

	filter := strings.ToLower(strings.TrimSpace(p.filter))
	filtered := make([]ai.ModelInfo, 0, len(p.options))
	for _, option := range p.options {
		name := strings.ToLower(option.Name)
		id := strings.ToLower(option.ID)
		if strings.Contains(name, filter) || strings.Contains(id, filter) {
			filtered = append(filtered, option)
		}
	}
	return filtered
}

func (p *ModelPickerPanel) ensureVisible(filtered []ai.ModelInfo, listHeight int) {
	if len(filtered) == 0 {
		p.selected = 0
		p.scroll = 0
		return
	}

	if p.selected < 0 {
		p.selected = 0
	}
	if p.selected >= len(filtered) {
		p.selected = len(filtered) - 1
	}

	maxScroll := len(filtered) - listHeight
	if maxScroll < 0 {
		maxScroll = 0
	}
	if p.scroll > maxScroll {
		p.scroll = maxScroll
	}

	if p.selected < p.scroll {
		p.scroll = p.selected
	}
	if p.selected >= p.scroll+listHeight {
		p.scroll = p.selected - listHeight + 1
	}
	if p.scroll < 0 {
		p.scroll = 0
	}
}

func (p *ModelPickerPanel) dimensions() (int, int, int) {
	width := p.width
	height := p.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	boxWidth := width - 2
	if boxWidth > 90 {
		boxWidth = 90
	}
	if boxWidth < 60 {
		boxWidth = 60
	}

	contentWidth := boxWidth - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	maxContentHeight := height - 4
	if maxContentHeight < 6 {
		maxContentHeight = 6
	}

	const fixedLines = 5
	listHeight := maxContentHeight - fixedLines
	if listHeight < 1 {
		listHeight = 1
	}

	return boxWidth, contentWidth, listHeight
}

func (p *ModelPickerPanel) listHeight() int {
	_, _, listHeight := p.dimensions()
	return listHeight
}

func (p *ModelPickerPanel) maxLabelWidth(options []ai.ModelInfo, contentWidth int) int {
	const minLabelWidth = 8
	const prefixWidth = 2
	const gapWidth = 1

	maxWidth := 0
	for _, option := range options {
		label := modelOptionLabel(option)
		if width := lipgloss.Width(label); width > maxWidth {
			maxWidth = width
		}
	}

	if maxWidth < minLabelWidth {
		maxWidth = minLabelWidth
	}

	maxAllowed := contentWidth - prefixWidth - gapWidth
	if maxAllowed < 4 {
		maxAllowed = 4
	}
	if maxWidth > maxAllowed {
		maxWidth = maxAllowed
	}

	return maxWidth
}

func modelOptionLabel(option ai.ModelInfo) string {
	label := strings.TrimSpace(option.Name)
	if label == "" {
		return option.ID
	}
	return label
}

func modelOptionDesc(option ai.ModelInfo) string {
	label := strings.TrimSpace(option.Name)
	if label == "" || label == option.ID {
		return ""
	}
	return option.ID
}

// Helpers for truncation/padding would be duplicated or need a shared utils package.
// For now, I'll duplicate them since they are small - wait, they are not small.
// I should use runewidth and strings.
// Let me verify if I included truncateToWidth and friends.
// Oops, I didn't include them in the content above. I need to include them.
// They were in sidebar.go but they are likely duplicated in model_picker.go or I need to find where they are.
// Ah, `truncateToWidth` is used in `model_picker.go` line 252.
// It seems `truncateToWidth` was defined in `model_picker.go` or `sidebar.go`.
// Let me check `model_picker.go` again for helper functions.
// I scrolled down but maybe missed them.
// If they are not in `model_picker.go`, they might be in `util.go`? I haven't seen `util.go`.
// Wait, `sidebar.go` had them. `model_picker.go` might have its own copy or use them if they were exported? No they were unexported.
// So `model_picker.go` must have them if it uses them.
// Let me check `model_picker.go` fully.
