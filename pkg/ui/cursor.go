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
	
	// Parse cursor position codes
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
		}
	}
	
	// Track cursor movements
	// CSI n A - Cursor Up
	// CSI n B - Cursor Down
	// CSI n C - Cursor Forward
	// CSI n D - Cursor Back
	
	// Carriage return
	if strings.Contains(content, "\r") {
		ct.col = 0
	}
	
	// Newline
	if strings.Contains(content, "\n") {
		ct.row++
		// Don't reset col on \n without \r
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
		// Cursor out of bounds, return as-is
		return content
	}
	
	line := lines[ct.row]
	
	if ct.col < 0 || ct.col > len(line) {
		// Cursor out of bounds, return as-is
		return content
	}
	
	// Insert cursor character at position
	if ct.col == len(line) {
		// At end of line
		lines[ct.row] = line + cursorChar
	} else {
		// In middle of line - replace character with highlighted version
		runes := []rune(line)
		if ct.col < len(runes) {
			// Highlight the character under cursor
			lines[ct.row] = string(runes[:ct.col]) + cursorChar + string(runes[ct.col+1:])
		}
	}
	
	return strings.Join(lines, "\n")
}
