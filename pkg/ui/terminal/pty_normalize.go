package terminal

import (
	"bytes"
	"strings"
)

const tabWidth = 4

var TabSpaces = []byte(strings.Repeat(" ", tabWidth))

func AppendPTYContent(content string, data []byte, pendingCR *bool) string {
	if len(data) == 0 {
		return content
	}

	buf := []byte(content)

	// State for escape sequence parsing
	inEscape := false
	inCSI := false // CSI = Control Sequence Introducer (ESC[)
	csiStart := 0  // Start index of current CSI sequence
	escStart := 0  // Start index of escape sequence

	for i := 0; i < len(data); i++ {
		b := data[i]

		// Handle escape sequences
		if inEscape {
			if b == '[' {
				inCSI = true
				inEscape = false
				csiStart = escStart
				continue
			}
			// Other escape sequences (ESC followed by single char) - preserve them
			buf = append(buf, data[escStart:i+1]...)
			inEscape = false
			continue
		}

		if inCSI {
			// CSI sequences end with a letter (0x40-0x7E)
			if b >= 0x40 && b <= 0x7E {
				// Check what type of CSI sequence this is
				switch b {
				case 'm':
					// SGR (Select Graphic Rendition) - color/style codes - PRESERVE
					buf = append(buf, data[csiStart:i+1]...)
				case 'D':
					// Cursor left - treat as backspace
					lineStart := bytes.LastIndexByte(buf, '\n')
					if lineStart == -1 {
						lineStart = -1
					}
					if len(buf) > lineStart+1 {
						buf = buf[:len(buf)-1]
					}
				case 'K':
					// Clear to end of line - nothing to do since we're at end
				case 'C', 'A', 'B', 'H', 'J', 'f':
					// Cursor movement/screen clear - skip for our simple terminal
				default:
					// Other CSI sequences - preserve them
					buf = append(buf, data[csiStart:i+1]...)
				}
				inCSI = false
				continue
			}
			// Still in CSI sequence (parameters, intermediate bytes)
			continue
		}

		// Handle pending CR FIRST (before escape check)
		// This ensures CR followed by escape sequence doesn't leave pendingCR true
		if pendingCR != nil && *pendingCR {
			if b == '\n' {
				buf = append(buf, '\n')
				*pendingCR = false
				continue
			}
			if b == '\r' {
				continue
			}
			// Don't clear line if followed by escape (might be color codes)
			if b == 0x1b {
				*pendingCR = false
				// Fall through to escape handling below
			} else {
				buf = trimToLineStart(buf)
				*pendingCR = false
			}
		}

		// Start of escape sequence
		if b == 0x1b {
			inEscape = true
			escStart = i
			continue
		}

		switch b {
		case '\r':
			if pendingCR != nil {
				*pendingCR = true
			}
		case '\n':
			buf = append(buf, '\n')
		case '\t':
			buf = append(buf, TabSpaces...)
		case 0x08: // Backspace - remove last character from current line
			lineStart := bytes.LastIndexByte(buf, '\n')
			if lineStart == -1 {
				lineStart = -1
			}
			if len(buf) > lineStart+1 {
				buf = buf[:len(buf)-1]
			}
		case 0x7f: // DEL - also treat as backspace for display
			lineStart := bytes.LastIndexByte(buf, '\n')
			if lineStart == -1 {
				lineStart = -1
			}
			if len(buf) > lineStart+1 {
				buf = buf[:len(buf)-1]
			}
		default:
			// Only append printable characters (0x20 and above)
			if b >= 0x20 {
				buf = append(buf, b)
			}
		}
	}

	return string(buf)
}

func trimToLineStart(buf []byte) []byte {
	idx := bytes.LastIndexByte(buf, '\n')
	if idx == -1 {
		return buf[:0]
	}
	return buf[:idx+1]
}
