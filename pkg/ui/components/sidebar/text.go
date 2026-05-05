package sidebar

import (
	"runtime"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/mattn/go-runewidth"
)

func splitByWidth(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}

	var parts []string
	var sb strings.Builder
	currentWidth := 0

	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width && currentWidth > 0 {
			parts = append(parts, sb.String())
			sb.Reset()
			currentWidth = 0
		}
		sb.WriteRune(r)
		currentWidth += runeWidth
	}

	if sb.Len() > 0 {
		parts = append(parts, sb.String())
	}

	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(text) <= width {
		return text
	}
	if width <= 3 {
		return trimToWidth(text, width)
	}
	return trimToWidth(text, width-3) + "..."
}

func trimToWidth(text string, width int) string {
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

func padPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := runewidth.StringWidth(text)
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
	if textWidth >= width {
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

func stripANSICodes(s string) string {
	if s == "" {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == 0x1b { // ESC
			if i+1 >= len(s) {
				continue
			}
			next := s[i+1]
			switch next {
			case '[': // CSI
				i += 2
				for i < len(s) {
					ch = s[i]
					if ch >= 0x40 && ch <= 0x7E {
						break
					}
					i++
				}
			case ']': // OSC
				i += 2
				for i < len(s) {
					if s[i] == 0x07 {
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i++
						break
					}
					i++
				}
			default:
				i++
			}
			continue
		}

		sb.WriteByte(ch)
	}

	return sb.String()
}

// MessagePrefix returns the markdown prefix for a chat message role.
// Emoji are suppressed on darwin where terminal support is inconsistent.
func MessagePrefix(role string) string {
	useEmoji := runtime.GOOS != "darwin"
	switch role {
	case "user":
		if useEmoji {
			return "👤 **You:** "
		}
		return "**You:** "
	case "tool":
		if useEmoji {
			return "🔧 **Tool:** "
		}
		return "**Tool:** "
	case "error":
		if useEmoji {
			return "❌ Error: "
		}
		return "Error: "
	default:
		if useEmoji {
			return "🖥️ **Assistant:** "
		}
		return "**Assistant:** "
	}
}
