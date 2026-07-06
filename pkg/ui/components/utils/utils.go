package utils

import (
	"strconv"
	"strings"
	"unicode/utf8"

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

// EscapeControl makes s safe to render in a single-line UI field. Any
// control character (including newline/tab/ESC) or invalid UTF-8 byte
// causes the whole string to be Go-quoted, so a hostile value (e.g. a
// model-supplied path) cannot break layout or inject terminal control
// sequences into the rendered popup. Ordinary strings pass through
// unchanged.
func EscapeControl(s string) string {
	if isSafeDisplayString(s) {
		return s
	}
	return strconv.Quote(s)
}

func isSafeDisplayString(s string) bool {
	if !utf8.ValidString(s) {
		return false
	}
	for _, r := range s {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// TailPreservingTruncate shortens text to at most width display cells,
// keeping the END of the string and eliding the front with "…". Use this
// instead of TruncateToWidth for values where the distinguishing suffix
// matters more than the prefix — e.g. "/safe/looking/prefix/secret" must not
// be allowed to display as "/safe/looking/prefix/…", which would hide
// exactly the part that reveals what is really being accessed.
func TailPreservingTruncate(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if ansi.StringWidth(text) <= width {
		return text
	}
	if width <= 1 {
		return TrimToWidth(text, width)
	}
	runes := []rune(text)
	for start := 1; start < len(runes); start++ {
		candidate := "…" + string(runes[start:])
		if ansi.StringWidth(candidate) <= width {
			return candidate
		}
	}
	return "…"
}
