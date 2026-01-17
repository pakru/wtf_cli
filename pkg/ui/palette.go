package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Command represents a slash command
type Command struct {
	Name        string
	Description string
}

// CommandPalette displays available slash commands
type CommandPalette struct {
	commands []Command
	selected int
	filter   string
	visible  bool
	width    int
	height   int
}

// NewCommandPalette creates a new command palette
func NewCommandPalette() *CommandPalette {
	return &CommandPalette{
		commands: []Command{
			{Name: "/wtf", Description: "Analyze last output and suggest fixes"},
			{Name: "/explain", Description: "Explain what the last command did"},
			{Name: "/fix", Description: "Suggest fix for last error"},
			{Name: "/history", Description: "Show command history"},
			{Name: "/models", Description: "Show available models"},
			{Name: "/settings", Description: "Open settings panel"},
			{Name: "/close_sidebar", Description: "Close AI sidebar"},
			{Name: "/help", Description: "Show help"},
		},
		selected: 0,
		visible:  false,
	}
}

// Show makes the palette visible
func (p *CommandPalette) Show() {
	p.visible = true
	p.selected = 0
	p.filter = ""
}

// Hide hides the palette
func (p *CommandPalette) Hide() {
	p.visible = false
}

// IsVisible returns whether the palette is visible
func (p *CommandPalette) IsVisible() bool {
	return p.visible
}

// SetSize sets the palette dimensions
func (p *CommandPalette) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// filteredCommands returns commands matching the current filter
func (p *CommandPalette) filteredCommands() []Command {
	if p.filter == "" {
		return p.commands
	}

	var filtered []Command
	filter := strings.ToLower(p.filter)
	for _, cmd := range p.commands {
		if strings.Contains(strings.ToLower(cmd.Name), filter) ||
			strings.Contains(strings.ToLower(cmd.Description), filter) {
			filtered = append(filtered, cmd)
		}
	}
	return filtered
}

// paletteSelectMsg is sent when a command is selected
type paletteSelectMsg struct {
	command string
}

// paletteCancelMsg is sent when palette is cancelled
type paletteCancelMsg struct{}

// Update handles keyboard input for the palette
func (p *CommandPalette) Update(msg tea.KeyMsg) tea.Cmd {
	filtered := p.filteredCommands()

	switch msg.Type {
	case tea.KeyUp:
		if p.selected > 0 {
			p.selected--
		}
		return nil

	case tea.KeyDown:
		if p.selected < len(filtered)-1 {
			p.selected++
		}
		return nil

	case tea.KeyEnter:
		// Select current command
		if len(filtered) > 0 && p.selected < len(filtered) {
			cmd := filtered[p.selected]
			p.Hide()
			return func() tea.Msg {
				return paletteSelectMsg{command: cmd.Name}
			}
		}
		return nil

	case tea.KeyEsc:
		// Cancel palette
		p.Hide()
		return func() tea.Msg {
			return paletteCancelMsg{}
		}

	case tea.KeyBackspace:
		// Delete filter character
		if len(p.filter) > 0 {
			p.filter = p.filter[:len(p.filter)-1]
			p.selected = 0 // Reset selection when filter changes
		}
		return nil

	case tea.KeyRunes:
		// Add to filter
		p.filter += msg.String()
		p.selected = 0 // Reset selection when filter changes
		return nil
	}

	return nil
}

// View renders the command palette
func (p *CommandPalette) View() string {
	if !p.visible {
		return ""
	}

	// Use a capped width for readability
	boxWidth := p.width - 2 // Leave room for border
	if boxWidth > 80 {
		boxWidth = 80
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Styles - full width bar
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("141")).
		Padding(0, 2).
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

	selectedDescStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Bold(true)

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	filterStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	// Build content
	var content strings.Builder

	// Title
	content.WriteString(titleStyle.Render("Command Palette"))
	content.WriteString("\n")

	// Filter input
	if p.filter != "" {
		content.WriteString(filterStyle.Render("Filter: " + p.filter))
		content.WriteString("\n")
	}

	content.WriteString("\n")

	// Commands
	filtered := p.filteredCommands()
	if len(filtered) == 0 {
		content.WriteString(descStyle.Render("No matching commands"))
	} else {
		maxNameWidth := 0
		for _, cmd := range filtered {
			if w := lipgloss.Width(cmd.Name); w > maxNameWidth {
				maxNameWidth = w
			}
		}
		if maxNameWidth < 4 {
			maxNameWidth = 4
		}

		for i, cmd := range filtered {
			namePadding := maxNameWidth - lipgloss.Width(cmd.Name)
			if namePadding < 0 {
				namePadding = 0
			}
			nameLabel := cmd.Name + strings.Repeat(" ", namePadding)

			if i == p.selected {
				line := selectedStyle.Render("  " + nameLabel + " ")
				line += " " + selectedDescStyle.Render(cmd.Description)
				content.WriteString(line + "\n")
			} else {
				line := normalStyle.Render("  " + nameLabel + " ")
				line += " " + descStyle.Render(cmd.Description)
				content.WriteString(line + "\n")
			}
		}
	}

	content.WriteString("\n")
	content.WriteString(descStyle.Render("↑↓ Navigate • Enter Select • Esc Cancel"))

	// Render the box (centering handled by model.overlayCenter)
	return boxStyle.Render(content.String())
}

// GetSelectedCommand returns the currently selected command name
func (p *CommandPalette) GetSelectedCommand() string {
	filtered := p.filteredCommands()
	if len(filtered) > 0 && p.selected < len(filtered) {
		return filtered[p.selected].Name
	}
	return ""
}
