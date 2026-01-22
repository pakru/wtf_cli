package utils

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

// TruncateToWidth truncates string to width with ellipsis
func TruncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) <= width {
		return text
	}
	if width <= 3 {
		return TrimToWidth(text, width)
	}
	return ansi.Truncate(text, width, "...")
}

// TrimToWidth trims string to width without ellipsis
func TrimToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	return ansi.Truncate(text, width, "")
}

// PadPlain pads text with spaces to width
func PadPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := ansi.StringWidth(text)
	if textWidth >= width {
		return text
	}
	return text + strings.Repeat(" ", width-textWidth)
}

// PadStyled pads text with spaces to width, accounting for style
func PadStyled(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return text
	}
	return text + strings.Repeat(" ", width-textWidth)
}
