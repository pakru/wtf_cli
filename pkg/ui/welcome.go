package ui

import (
	"fmt"
	"strings"

	"wtf_cli/pkg/version"

	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

// WelcomeMessage returns the welcome box string to print to PTY
func WelcomeMessage() string {
	const boxWidth = 53 // Total inner width

	// Styles
	borderStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("99"))
	titleStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("219")).Bold(true)
	keyStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("222")).Bold(true)
	descStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	headerStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("248"))
	versionStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("244"))

	// Helper: create a line with content padded to boxWidth
	makeLine := func(content string, visualWidth int) string {
		pad := boxWidth - visualWidth
		if pad < 0 {
			pad = 0
		}
		return borderStyle.Render("│") + content + strings.Repeat(" ", pad) + borderStyle.Render("│")
	}

	// Borders
	top := borderStyle.Render("╭" + strings.Repeat("─", boxWidth) + "╮")
	bottom := borderStyle.Render("╰" + strings.Repeat("─", boxWidth) + "╯")
	empty := makeLine("", 0)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, top)

	// Title: ✨ Welcome to WTF CLI ✨
	titleText := "✨ Welcome to WTF CLI ✨"
	rawTitleWidth := runewidth.StringWidth(titleText)
	titleLeftPad := (boxWidth - rawTitleWidth) / 2
	titleLine := strings.Repeat(" ", titleLeftPad) + titleStyle.Render(titleText)
	lines = append(lines, makeLine(titleLine, titleLeftPad+rawTitleWidth))

	lines = append(lines, empty)

	// Shortcuts header
	shortcutsHeader := "  Shortcuts:"
	lines = append(lines, makeLine(headerStyle.Render(shortcutsHeader), runewidth.StringWidth(shortcutsHeader)))

	// Shortcuts
	shortcuts := []struct{ key, desc string }{
		{"Ctrl+D", "Exit terminal (press twice)"},
		{"Ctrl+C", "Cancel current command"},
		{"Ctrl+Z", "Suspend process"},
		{"/", "Open command palette"},
	}
	for _, s := range shortcuts {
		keyFormatted := fmt.Sprintf("    %-10s", s.key)
		line := keyStyle.Render(keyFormatted) + descStyle.Render(s.desc)
		lineWidth := runewidth.StringWidth(keyFormatted) + runewidth.StringWidth(s.desc)
		lines = append(lines, makeLine(line, lineWidth))
	}

	lines = append(lines, empty)

	// Version at bottom (centered, dimmed)
	versionText := version.Summary()
	maxVersionLen := boxWidth - 4
	if runewidth.StringWidth(versionText) > maxVersionLen {
		versionText = truncateToWidth(versionText, maxVersionLen)
	}
	versionLeftPad := (boxWidth - runewidth.StringWidth(versionText)) / 2
	versionLine := strings.Repeat(" ", versionLeftPad) + versionStyle.Render(versionText)
	lines = append(lines, makeLine(versionLine, versionLeftPad+runewidth.StringWidth(versionText)))

	lines = append(lines, bottom)
	lines = append(lines, "")

	return strings.Join(lines, "\n") + "\n"
}
