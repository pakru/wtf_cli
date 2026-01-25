package terminal

import (
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

// CursorTracker tracks cursor position from ANSI escape sequences
type CursorTracker struct {
	row int
	col int
}

// NewCursorTracker creates a new cursor tracker
func NewCursorTracker() *CursorTracker {
	return &CursorTracker{
		row: 0,
		col: 0,
	}
}

// UpdateFromOutput parses ANSI codes to track cursor position
func (ct *CursorTracker) UpdateFromOutput(data []byte) {
	content := string(data)

	// Parse absolute cursor position codes first (these override)
	// CSI n ; m H or CSI n ; m f  - Cursor Position
	cursorPosRegex := regexp.MustCompile(`\x1b\[(\d+);(\d+)[Hf]`)
	matches := cursorPosRegex.FindAllStringSubmatch(content, -1)

	for _, match := range matches {
		if len(match) == 3 {
			// Parse row and col (ANSI is 1-indexed)
			var row, col int
			fmt.Sscanf(match[1], "%d", &row)
			fmt.Sscanf(match[2], "%d", &col)
			ct.row = row - 1 // Convert to 0-indexed
			ct.col = col - 1
			return // Absolute position set, don't process further
		}
	}

	// Carriage return - go to start of line
	if strings.Contains(content, "\r") {
		ct.col = 0
	}

	// Newline - move to next line
	if strings.Contains(content, "\n") {
		ct.row++
	}

	// Strip ANSI codes to count actual visible characters
	ansiRegex := regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`)
	strippedContent := ansiRegex.ReplaceAllString(content, "")

	// Parse cursor movement codes (from original content, not stripped)
	// ESC[nC - Cursor Forward
	cursorForwardRegex := regexp.MustCompile(`\x1b\[(\d*)C`)
	if matches := cursorForwardRegex.FindStringSubmatch(content); len(matches) > 0 {
		n := 1
		if len(matches) > 1 && matches[1] != "" {
			fmt.Sscanf(matches[1], "%d", &n)
		}
		ct.col += n
	}

	// ESC[nD - Cursor Back
	cursorBackRegex := regexp.MustCompile(`\x1b\[(\d*)D`)
	if matches := cursorBackRegex.FindStringSubmatch(content); len(matches) > 0 {
		n := 1
		if len(matches) > 1 && matches[1] != "" {
			fmt.Sscanf(matches[1], "%d", &n)
		}
		ct.col -= n
		if ct.col < 0 {
			ct.col = 0
		}
	}

	// Count only visible printable characters (after stripping ANSI)
	for _, ch := range strippedContent {
		if ch >= 32 && ch < 127 && ch != '\r' && ch != '\n' {
			// Printable character - advance cursor
			ct.col++
		}
	}

	// Backspace
	if strings.Contains(content, "\x7f") || strings.Contains(content, "\b") {
		ct.col--
		if ct.col < 0 {
			ct.col = 0
		}
	}
}

// GetPosition returns current cursor position
func (ct *CursorTracker) GetPosition() (row, col int) {
	return ct.row, ct.col
}

// SetPosition overrides the tracked cursor position.
func (ct *CursorTracker) SetPosition(row, col int) {
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	ct.row = row
	ct.col = col
}

// RenderCursorOverlay adds a visual cursor at the end of the last line
func (ct *CursorTracker) RenderCursorOverlay(content string, cursorChar string) string {
	if cursorChar == "" {
		return content
	}

	if len(content) == 0 {
		return cursorChar
	}

	lines := strings.Split(content, "\n")
	row := ct.row
	col := ct.col
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	for len(lines) <= row {
		lines = append(lines, "")
	}

	lines[row] = renderCursorInLine(lines[row], col, cursorChar)
	return strings.Join(lines, "\n")
}

func renderCursorInLine(line string, col int, cursorChar string) string {
	const (
		inverseOn  = "\x1b[7m"
		inverseOff = "\x1b[27m"
	)

	var b strings.Builder
	visible := 0
	applied := false

	for i := 0; i < len(line); {
		if line[i] == 0x1b && i+1 < len(line) && line[i+1] == '[' {
			j := i + 2
			for j < len(line) {
				c := line[j]
				if (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') {
					j++
					break
				}
				j++
			}
			b.WriteString(line[i:j])
			i = j
			continue
		}

		r, size := utf8.DecodeRuneInString(line[i:])
		if r == utf8.RuneError && size == 1 {
			width := 1
			if !applied && col >= visible && col < visible+width {
				b.WriteString(inverseOn)
				b.WriteByte(line[i])
				b.WriteString(inverseOff)
				applied = true
			} else {
				b.WriteByte(line[i])
			}
			i++
			visible += width
			continue
		}

		width := runewidth.RuneWidth(r)
		if width < 1 {
			width = 1
		}
		if !applied && col >= visible && col < visible+width {
			b.WriteString(inverseOn)
			b.WriteRune(r)
			b.WriteString(inverseOff)
			applied = true
		} else {
			b.WriteRune(r)
		}
		i += size
		visible += width
	}

	if !applied {
		if visible < col {
			b.WriteString(strings.Repeat(" ", col-visible))
		}
		b.WriteString(cursorChar)
	}

	return b.String()
}
