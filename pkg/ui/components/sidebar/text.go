package sidebar

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// splitByWidth hard-wraps text into chunks no wider than width display cells.
func splitByWidth(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	return strings.Split(ansi.Hardwrap(text, width, true), "\n")
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) <= width {
		return text
	}
	if width <= 3 {
		return ansi.Truncate(text, width, "")
	}
	return ansi.Truncate(text, width, "...")
}

func trimToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(text, width, "")
}

func padPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := ansi.StringWidth(text)
	if textWidth >= width {
		return text
	}
	return text + strings.Repeat(" ", width-textWidth)
}

func padStyled(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := lipgloss.Width(text)
	if textWidth > width {
		// Clamp over-wide lines (e.g. width miscalculations on VS16 emoji) so the
		// surrounding border box can never wrap them onto an extra row and grow
		// vertically. ansi.Truncate is ANSI-aware and uses the renderer's width.
		return ansi.Truncate(text, width, "")
	}
	if textWidth == width {
		return text
	}
	return text + strings.Repeat(" ", width-textWidth)
}

func sanitizeContent(content string) string {
	if content == "" {
		return content
	}
	var sb strings.Builder
	sb.Grow(len(content))
	for _, r := range content {
		switch r {
		case '\n', '\t':
			sb.WriteRune(r)
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

// stripANSICodes removes ANSI escape sequences, leaving plain display text.
func stripANSICodes(s string) string {
	return ansi.Strip(s)
}

// MessagePrefix returns the markdown prefix for a chat message role.
func MessagePrefix(role string) string {
	switch role {
	case "user":
		return "**You:** "
	case "tool":
		return "**Tool:** "
	case "error":
		return "Error: "
	default:
		return "**Assistant:** "
	}
}
