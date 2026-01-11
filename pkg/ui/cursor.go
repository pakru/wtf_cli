package ui

import (
	"fmt"
	"regexp"
	"strings"
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

// RenderCursorOverlay adds a visual cursor to content
func (ct *CursorTracker) RenderCursorOverlay(content string, cursorChar string) string {
	lines := strings.Split(content, "\n")

	if ct.row < 0 || ct.row >= len(lines) {
		// Cursor out of bounds, show at end
		return content + cursorChar
	}

	line := lines[ct.row]

	if ct.col < 0 {
		return content + cursorChar
	}

	// Insert cursor character at position
	if ct.col >= len(line) {
		// At end of line - append cursor
		lines[ct.row] = line + cursorChar
	} else {
		// In middle of line - insert cursor before the character
		runes := []rune(line)
		if ct.col < len(runes) {
			// Insert visible cursor character at position
			lines[ct.row] = string(runes[:ct.col]) + cursorChar + string(runes[ct.col:])
		}
	}

	return strings.Join(lines, "\n")
}
