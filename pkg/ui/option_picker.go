package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type openOptionPickerMsg struct {
	fieldKey string
	title    string
	options  []string
	current  string
}

type optionPickerSelectMsg struct {
	fieldKey string
	value    string
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
func (p *OptionPickerPanel) Update(msg tea.KeyMsg) tea.Cmd {
	if !p.visible {
		return nil
	}

	listHeight := p.listHeight()

	switch msg.Type {
	case tea.KeyUp:
		if p.selected > 0 {
			p.selected--
		}
		p.ensureVisible(listHeight)
		return nil

	case tea.KeyDown:
		if p.selected < len(p.options)-1 {
			p.selected++
		}
		p.ensureVisible(listHeight)
		return nil

	case tea.KeyPgUp:
		if len(p.options) > 0 {
			p.selected -= listHeight
			if p.selected < 0 {
				p.selected = 0
			}
			p.ensureVisible(listHeight)
		}
		return nil

	case tea.KeyPgDown:
		if len(p.options) > 0 {
			p.selected += listHeight
			if p.selected > len(p.options)-1 {
				p.selected = len(p.options) - 1
			}
			p.ensureVisible(listHeight)
		}
		return nil

	case tea.KeyHome:
		if len(p.options) > 0 {
			p.selected = 0
			p.ensureVisible(listHeight)
		}
		return nil

	case tea.KeyEnd:
		if len(p.options) > 0 {
			p.selected = len(p.options) - 1
			p.ensureVisible(listHeight)
		}
		return nil

	case tea.KeyEnter:
		if len(p.options) > 0 && p.selected >= 0 && p.selected < len(p.options) {
			value := p.options[p.selected]
			p.Hide()
			return func() tea.Msg {
				return optionPickerSelectMsg{fieldKey: p.fieldKey, value: value}
			}
		}
		return nil

	case tea.KeyEsc:
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

	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("141")).
		Padding(1, 2).
		Width(boxWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")).
		Bold(true)

	normalStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("141")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

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
				line = padPlain(line, contentWidth)
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
