package ui

import (
	"strings"

	"github.com/mattn/go-runewidth"
)

const tabWidth = 4

func normalizePTYOutput(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	text := string(data)
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "")
	return expandTabs(text, tabWidth)
}

func expandTabs(text string, width int) string {
	if width <= 0 || !strings.Contains(text, "\t") {
		return text
	}

	var sb strings.Builder
	col := 0

	for _, r := range text {
		switch r {
		case '\n':
			sb.WriteRune(r)
			col = 0
		case '\t':
			spaces := width - (col % width)
			if spaces <= 0 {
				spaces = width
			}
			sb.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		default:
			sb.WriteRune(r)
			col += runewidth.RuneWidth(r)
		}
	}

	return sb.String()
}
