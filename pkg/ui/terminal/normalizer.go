package terminal

// Normalizer converts raw PTY output into normalized plain-text lines.
// It handles common control sequences such as CR/LF, backspace, CSI cursor
// left, OSC title sequences, and tabs.
type Normalizer struct {
	line        []byte
	pendingCR   bool
	inEscape    bool
	inCSI       bool
	csiParam    int
	csiHasParam bool
	inOSC       bool
	oscEscape   bool
}

// NewNormalizer creates a new PTY normalizer instance.
func NewNormalizer() *Normalizer {
	return &Normalizer{}
}

// Append processes raw PTY data and returns any completed normalized lines.
// Lines are returned without ANSI/OSC sequences and without trailing newlines.
func (n *Normalizer) Append(data []byte) [][]byte {
	if len(data) == 0 {
		return nil
	}

	var lines [][]byte

	for _, b := range data {
		if n.inOSC {
			if n.oscEscape {
				if b == '\\' {
					n.inOSC = false
				}
				n.oscEscape = false
				continue
			}
			if b == 0x07 {
				n.inOSC = false
				continue
			}
			if b == 0x1b {
				n.oscEscape = true
				continue
			}
			continue
		}

		if n.inEscape {
			if b == '[' {
				n.inCSI = true
				n.inEscape = false
				n.csiParam = 0
				n.csiHasParam = false
				continue
			}
			if b == ']' {
				n.inEscape = false
				n.inOSC = true
				continue
			}
			// Ignore other single-char escape sequences.
			n.inEscape = false
			continue
		}

		if n.inCSI {
			if b >= '0' && b <= '9' {
				n.csiParam = n.csiParam*10 + int(b-'0')
				n.csiHasParam = true
				continue
			}
			if b == ';' {
				continue
			}
			if b >= 0x40 && b <= 0x7E {
				switch b {
				case 'D':
					count := 1
					if n.csiHasParam && n.csiParam > 0 {
						count = n.csiParam
					}
					for i := 0; i < count && len(n.line) > 0; i++ {
						n.line = n.line[:len(n.line)-1]
					}
				case 'K':
					// Clear to end of line - nothing to do for buffer output.
				}
				n.inCSI = false
				n.csiParam = 0
				n.csiHasParam = false
				continue
			}
			continue
		}

		if n.pendingCR {
			if b == '\n' {
				n.flushLine(&lines)
				n.pendingCR = false
				continue
			}
			if b == '\r' {
				continue
			}
			if b == 0x1b {
				n.pendingCR = false
			} else {
				n.line = n.line[:0]
				n.pendingCR = false
			}
		}

		if b == 0x1b {
			n.inEscape = true
			continue
		}

		switch b {
		case '\r':
			n.pendingCR = true
		case '\n':
			n.flushLine(&lines)
		case '\t':
			n.line = append(n.line, TabSpaces...)
		case 0x08, 0x7f:
			if len(n.line) > 0 {
				n.line = n.line[:len(n.line)-1]
			}
		default:
			if b >= 0x20 {
				n.line = append(n.line, b)
			}
		}
	}

	return lines
}

func (n *Normalizer) flushLine(lines *[][]byte) {
	if len(n.line) == 0 {
		return
	}
	lineCopy := make([]byte, len(n.line))
	copy(lineCopy, n.line)
	*lines = append(*lines, lineCopy)
	n.line = n.line[:0]
}
