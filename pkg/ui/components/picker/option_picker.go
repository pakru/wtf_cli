package picker

import (
	"strings"

	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
)

type OpenOptionPickerMsg struct {
	FieldKey string
	Title    string
	Options  []string
	Current  string
}

type OptionPickerSelectMsg struct {
	FieldKey string
	Value    string
}

// OptionPickerPanel provides a simple list picker for settings options.
type OptionPickerPanel struct {
	title    string
	fieldKey string
	options  []string
	selected int
	scroll   int
	visible  bool
	width    int
	height   int
}

// NewOptionPickerPanel creates a new option picker panel.
func NewOptionPickerPanel() *OptionPickerPanel {
	return &OptionPickerPanel{}
}

// Show displays the picker for a settings field.
func (p *OptionPickerPanel) Show(title, fieldKey string, options []string, current string) {
	p.visible = true
	p.title = title
	p.fieldKey = fieldKey
	p.options = append([]string(nil), options...)
	p.selected = 0
	p.scroll = 0

	if current != "" {
		for i, option := range p.options {
			if option == current {
				p.selected = i
				break
			}
		}
	}

	p.ensureVisible(p.listHeight())
}

// Hide hides the picker.
func (p *OptionPickerPanel) Hide() {
	p.visible = false
}

// IsVisible reports whether the picker is visible.
func (p *OptionPickerPanel) IsVisible() bool {
	return p.visible
}

// SetSize updates the picker dimensions.
func (p *OptionPickerPanel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Update handles keyboard input for the picker.
func (p *OptionPickerPanel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if !p.visible {
		return nil
	}

	listHeight := p.listHeight()

	keyStr := msg.String()
	switch keyStr {
	case "up":
		if p.selected > 0 {
			p.selected--
		}
		p.ensureVisible(listHeight)
		return nil

	case "down":
		if p.selected < len(p.options)-1 {
			p.selected++
		}
		p.ensureVisible(listHeight)
		return nil

	case "pgup":
		if len(p.options) > 0 {
			p.selected -= listHeight
			if p.selected < 0 {
				p.selected = 0
			}
			p.ensureVisible(listHeight)
		}
		return nil

	case "pgdown":
		if len(p.options) > 0 {
			p.selected += listHeight
			if p.selected > len(p.options)-1 {
				p.selected = len(p.options) - 1
			}
			p.ensureVisible(listHeight)
		}
		return nil

	case "home":
		if len(p.options) > 0 {
			p.selected = 0
			p.ensureVisible(listHeight)
		}
		return nil

	case "end":
		if len(p.options) > 0 {
			p.selected = len(p.options) - 1
			p.ensureVisible(listHeight)
		}
		return nil

	case "enter":
		if len(p.options) > 0 && p.selected >= 0 && p.selected < len(p.options) {
			value := p.options[p.selected]
			p.Hide()
			return func() tea.Msg {
				return OptionPickerSelectMsg{FieldKey: p.fieldKey, Value: value}
			}
		}
		return nil

	case "esc":
		p.Hide()
		return nil
	}

	return nil
}

// View renders the picker.
func (p *OptionPickerPanel) View() string {
	if !p.visible {
		return ""
	}

	boxWidth, contentWidth, listHeight := p.dimensions()

	boxStyle := styles.BoxStyle.Width(boxWidth)
	titleStyle := styles.TitleStyle
	normalStyle := styles.TextStyle
	selectedStyle := styles.SelectedStyle
	descStyle := styles.TextMutedStyle
	footerStyle := styles.FooterStyle

	var content strings.Builder
	content.WriteString(titleStyle.Render(p.title))
	content.WriteString("\n\n")

	if len(p.options) == 0 {
		content.WriteString(descStyle.Render("No options available"))
		for i := 1; i < listHeight; i++ {
			content.WriteString("\n")
		}
	} else {
		for i := 0; i < listHeight; i++ {
			index := p.scroll + i
			if index >= len(p.options) {
				content.WriteString("\n")
				continue
			}
			option := p.options[index]
			line := "  " + option
			if index == p.selected {
				line = utils.PadPlain(line, contentWidth)
				content.WriteString(selectedStyle.Render(line))
			} else {
				content.WriteString(normalStyle.Render(line))
			}
			content.WriteString("\n")
		}
	}

	content.WriteString("\n")
	content.WriteString(footerStyle.Render("Up/Down Navigate | Enter Select | Esc Cancel"))

	return boxStyle.Render(content.String())
}

func (p *OptionPickerPanel) ensureVisible(listHeight int) {
	if len(p.options) == 0 {
		p.selected = 0
		p.scroll = 0
		return
	}

	if p.selected < 0 {
		p.selected = 0
	}
	if p.selected >= len(p.options) {
		p.selected = len(p.options) - 1
	}

	maxScroll := len(p.options) - listHeight
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

func (p *OptionPickerPanel) dimensions() (int, int, int) {
	width := p.width
	height := p.height
	if width <= 0 {
		width = 80
	}
	if height <= 0 {
		height = 24
	}

	boxWidth := width - 2
	if boxWidth > 70 {
		boxWidth = 70
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	contentWidth := boxWidth - 4
	if contentWidth < 10 {
		contentWidth = 10
	}

	maxContentHeight := height - 4
	if maxContentHeight < 6 {
		maxContentHeight = 6
	}

	const fixedLines = 4
	listHeight := maxContentHeight - fixedLines
	if listHeight < 1 {
		listHeight = 1
	}

	return boxWidth, contentWidth, listHeight
}

func (p *OptionPickerPanel) listHeight() int {
	_, _, listHeight := p.dimensions()
	return listHeight
}
