package utils

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

// TruncateToWidth truncates string to width with ellipsis
func TruncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(text) <= width {
		return text
	}
	if width <= 3 {
		return TrimToWidth(text, width)
	}
	return TrimToWidth(text, width-3) + "..."
}

// TrimToWidth trims string to width without ellipsis
func TrimToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	var sb strings.Builder
	currentWidth := 0
	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width {
			break
		}
		sb.WriteRune(r)
		currentWidth += runeWidth
	}
	return sb.String()
}

// PadPlain pads text with spaces to width
func PadPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := runewidth.StringWidth(text)
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
