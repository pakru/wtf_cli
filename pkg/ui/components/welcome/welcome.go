package welcome

import (
	"fmt"
	"strings"

	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"
	"wtf_cli/pkg/version"

	"github.com/mattn/go-runewidth"
)

// WelcomeMessage returns the welcome box string to print to PTY
func WelcomeMessage() string {
	const boxWidth = 53 // Total inner width

	// Helper: create a line with content padded to boxWidth
	makeLine := func(content string, visualWidth int) string {
		pad := boxWidth - visualWidth
		if pad < 0 {
			pad = 0
		}
		return styles.WelcomeBorderStyle.Render("│") + content + strings.Repeat(" ", pad) + styles.WelcomeBorderStyle.Render("│")
	}

	// Borders
	top := styles.WelcomeBorderStyle.Render("╭" + strings.Repeat("─", boxWidth) + "╮")
	bottom := styles.WelcomeBorderStyle.Render("╰" + strings.Repeat("─", boxWidth) + "╯")
	empty := makeLine("", 0)

	var lines []string
	lines = append(lines, "")
	lines = append(lines, top)

	// Title: ✨ Welcome to WTF CLI ✨
	titleText := "✨ Welcome to WTF CLI ✨"
	rawTitleWidth := runewidth.StringWidth(titleText)
	titleLeftPad := (boxWidth - rawTitleWidth) / 2
	titleLine := strings.Repeat(" ", titleLeftPad) + styles.WelcomeTitleStyle.Render(titleText)
	lines = append(lines, makeLine(titleLine, titleLeftPad+rawTitleWidth))

	lines = append(lines, empty)

	// Shortcuts header
	shortcutsHeader := "  Shortcuts:"
	lines = append(lines, makeLine(styles.WelcomeHeaderStyle.Render(shortcutsHeader), runewidth.StringWidth(shortcutsHeader)))

	// Shortcuts
	shortcuts := []struct{ key, desc string }{
		{"Ctrl+D", "Exit terminal (press twice)"},
		{"Ctrl+T", "Toggle tty analysis sidebar chat"},
		{"Ctrl+R", "Search command history"},
		{"Shift+Tab", "Switch focus to chat panel"},
		{"/", "Open command palette"},
	}
	for _, s := range shortcuts {
		keyFormatted := fmt.Sprintf("    %-10s", s.key)
		line := styles.WelcomeKeyStyle.Render(keyFormatted) + styles.TextStyle.Render(s.desc)
		lineWidth := runewidth.StringWidth(keyFormatted) + runewidth.StringWidth(s.desc)
		lines = append(lines, makeLine(line, lineWidth))
	}

	lines = append(lines, empty)

	// Version at bottom (centered, dimmed)
	versionText := version.Summary()
	maxVersionLen := boxWidth - 4
	if runewidth.StringWidth(versionText) > maxVersionLen {
		versionText = utils.TruncateToWidth(versionText, maxVersionLen)
	}
	versionLeftPad := (boxWidth - runewidth.StringWidth(versionText)) / 2
	versionLine := strings.Repeat(" ", versionLeftPad) + styles.WelcomeVersionStyle.Render(versionText)
	lines = append(lines, makeLine(versionLine, versionLeftPad+runewidth.StringWidth(versionText)))

	lines = append(lines, bottom)
	lines = append(lines, "")

	return strings.Join(lines, "\n") + "\n"
}
