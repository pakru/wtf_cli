package ui

import (
	"io"

	tea "charm.land/bubbletea/v2"
)

// InputHandler manages keyboard input routing
type InputHandler struct {
	ptyWriter      io.Writer
	atLineStart    bool // Track if cursor is at start of line (for / detection)
	paletteMode    bool // True when command palette is active
	fullScreenMode bool // True when full-screen app (vim, nano) is active

	cursorKeysAppMode bool
	keypadAppMode     bool
	modePending       []byte
}

// NewInputHandler creates a new input handler
func NewInputHandler(ptyWriter io.Writer) *InputHandler {
	return &InputHandler{
		ptyWriter:   ptyWriter,
		atLineStart: true, // Start at line start (fresh prompt)
		modePending: make([]byte, 0, 8),
	}
}

// SetPaletteMode sets whether the command palette is active
func (ih *InputHandler) SetPaletteMode(active bool) {
	ih.paletteMode = active
}

// IsPaletteMode returns whether the command palette is active
func (ih *InputHandler) IsPaletteMode() bool {
	return ih.paletteMode
}

// showPaletteMsg is sent when / is pressed at line start
type showPaletteMsg struct{}

type ctrlDPressedMsg struct{}

// HandleKey processes a key message and returns whether it was handled
func (ih *InputHandler) HandleKey(msg tea.KeyPressMsg) (handled bool, cmd tea.Cmd) {
	// FULL-SCREEN MODE: bypass all special handling, send directly to PTY
	if ih.fullScreenMode {
		ih.sendKeyToPTY(msg)
		return true, nil
	}

	// If palette mode is active, don't process keys here
	// (they should be handled by the palette)
	if ih.paletteMode {
		return false, nil
	}

	keyStr := msg.String()

	// Check for special keys first using string matching (v2 API)
	switch keyStr {
	case "ctrl+c":
		// Ctrl+C - send interrupt to PTY
		ih.ptyWriter.Write([]byte{3}) // ASCII ETX (Ctrl+C)
		ih.atLineStart = true         // After interrupt, usually at new prompt
		return true, nil

	case "ctrl+d":
		return true, func() tea.Msg {
			return ctrlDPressedMsg{}
		}

	case "ctrl+z":
		// Ctrl+Z - suspend (send to PTY)
		ih.ptyWriter.Write([]byte{26}) // ASCII SUB (Ctrl+Z)
		return true, nil

	case "tab":
		// Tab - send to PTY
		ih.ptyWriter.Write([]byte{9}) // ASCII TAB
		ih.atLineStart = false
		return true, nil

	case "enter":
		// Enter - send newline to PTY
		ih.ptyWriter.Write([]byte{13}) // CR (some shells need this)
		ih.atLineStart = true          // After enter, we're at new line start
		return true, nil

	case "backspace":
		// Backspace - send to PTY
		ih.ptyWriter.Write([]byte{127}) // ASCII DEL
		// We can't perfectly track if we're at line start after backspace
		// but it's conservative to keep atLineStart false
		return true, nil

	case " ":
		// Space - send to PTY
		ih.ptyWriter.Write([]byte{32}) // ASCII SPACE
		ih.atLineStart = false
		return true, nil

	case "esc":
		// Escape - send to PTY
		ih.ptyWriter.Write([]byte{27}) // ASCII ESC
		return true, nil

	case "up":
		ih.ptyWriter.Write([]byte("\x1b[A"))
		return true, nil
	case "down":
		ih.ptyWriter.Write([]byte("\x1b[B"))
		return true, nil
	case "right":
		ih.ptyWriter.Write([]byte("\x1b[C"))
		return true, nil
	case "left":
		ih.ptyWriter.Write([]byte("\x1b[D"))
		return true, nil
	}

	// Handle ctrl key combinations
	if len(keyStr) > 5 && keyStr[:5] == "ctrl+" {
		if b, ok := ctrlKeyByteFromString(keyStr); ok {
			ih.ptyWriter.Write([]byte{b})
			if b == '\r' || b == '\n' {
				ih.atLineStart = true
			} else {
				ih.atLineStart = false
			}
			return true, nil
		}
	}

	// Check for slash at line start - trigger command palette
	if keyStr == "/" && ih.atLineStart {
		// Trigger command palette
		return true, func() tea.Msg {
			return showPaletteMsg{}
		}
	}

	// Normal "/" in the middle of typing - send to PTY
	if keyStr == "/" {
		ih.ptyWriter.Write([]byte("/"))
		ih.atLineStart = false
		return true, nil
	}

	// Normal typing - send to PTY if it's printable text
	key := msg.Key()
	if key.Text != "" {
		ih.ptyWriter.Write([]byte(key.Text))
		ih.atLineStart = false
		return true, nil
	}

	// Key not handled
	return false, nil
}

// UpdateTerminalModes updates input behavior based on terminal mode sequences.
func (ih *InputHandler) UpdateTerminalModes(data []byte) {
	if len(data) == 0 {
		return
	}

	combined := data
	if len(ih.modePending) > 0 {
		combined = append(ih.modePending, data...)
		ih.modePending = ih.modePending[:0]
	}

	for i := 0; i < len(combined); i++ {
		if combined[i] != 0x1b {
			continue
		}

		if i+1 >= len(combined) {
			ih.modePending = append(ih.modePending, combined[i:]...)
			break
		}

		switch combined[i+1] {
		case '[':
			if i+4 >= len(combined) {
				ih.modePending = append(ih.modePending, combined[i:]...)
				i = len(combined)
				break
			}
			if combined[i+2] == '?' && combined[i+3] == '1' {
				switch combined[i+4] {
				case 'h':
					ih.cursorKeysAppMode = true
					i += 4
				case 'l':
					ih.cursorKeysAppMode = false
					i += 4
				}
			}
		case '=':
			ih.keypadAppMode = true
			i++
		case '>':
			ih.keypadAppMode = false
			i++
		}
	}

	if len(ih.modePending) > 8 {
		ih.modePending = ih.modePending[len(ih.modePending)-8:]
	}
}

// SendToPTY sends raw bytes to the PTY
func (ih *InputHandler) SendToPTY(data []byte) error {
	_, err := ih.ptyWriter.Write(data)
	return err
}

// ResetLineStart resets the line start tracker (called after PTY output)
func (ih *InputHandler) ResetLineStart() {
	// Don't auto-reset - we track based on user input
}

// SetFullScreenMode enables or disables full-screen mode input bypass
func (ih *InputHandler) SetFullScreenMode(active bool) {
	ih.fullScreenMode = active
}

// IsFullScreenMode returns whether full-screen mode is active
func (ih *InputHandler) IsFullScreenMode() bool {
	return ih.fullScreenMode
}

// sendKeyToPTY sends a key directly to PTY with proper encoding
func (ih *InputHandler) sendKeyToPTY(msg tea.KeyPressMsg) {
	cursorSeq := func(normal, app string) []byte {
		if ih.cursorKeysAppMode {
			return []byte(app)
		}
		return []byte(normal)
	}

	keyStr := msg.String()
	key := msg.Key()

	switch keyStr {
	case "ctrl+c":
		ih.ptyWriter.Write([]byte{3}) // ASCII ETX (Ctrl+C)
	case "ctrl+d":
		ih.ptyWriter.Write([]byte{4}) // ASCII EOT (Ctrl+D)
	case "ctrl+z":
		ih.ptyWriter.Write([]byte{26}) // ASCII SUB (Ctrl+Z)
	case "tab":
		ih.ptyWriter.Write([]byte{9}) // ASCII TAB
	case "enter":
		ih.ptyWriter.Write([]byte{13}) // CR
	case "backspace":
		ih.ptyWriter.Write([]byte{127}) // ASCII DEL
	case " ":
		ih.ptyWriter.Write([]byte{32}) // ASCII SPACE
	case "esc":
		ih.ptyWriter.Write([]byte{27}) // ASCII ESC
	case "up":
		ih.ptyWriter.Write(cursorSeq("\x1b[A", "\x1bOA"))
	case "down":
		ih.ptyWriter.Write(cursorSeq("\x1b[B", "\x1bOB"))
	case "right":
		ih.ptyWriter.Write(cursorSeq("\x1b[C", "\x1bOC"))
	case "left":
		ih.ptyWriter.Write(cursorSeq("\x1b[D", "\x1bOD"))
	case "home":
		ih.ptyWriter.Write(cursorSeq("\x1b[H", "\x1bOH"))
	case "end":
		ih.ptyWriter.Write(cursorSeq("\x1b[F", "\x1bOF"))
	case "pgup":
		ih.ptyWriter.Write([]byte("\x1b[5~"))
	case "pgdown":
		ih.ptyWriter.Write([]byte("\x1b[6~"))
	case "delete":
		ih.ptyWriter.Write([]byte("\x1b[3~"))
	case "insert":
		ih.ptyWriter.Write([]byte("\x1b[2~"))
	case "f1":
		ih.ptyWriter.Write([]byte("\x1bOP"))
	case "f2":
		ih.ptyWriter.Write([]byte("\x1bOQ"))
	case "f3":
		ih.ptyWriter.Write([]byte("\x1bOR"))
	case "f4":
		ih.ptyWriter.Write([]byte("\x1bOS"))
	case "f5":
		ih.ptyWriter.Write([]byte("\x1b[15~"))
	case "f6":
		ih.ptyWriter.Write([]byte("\x1b[17~"))
	case "f7":
		ih.ptyWriter.Write([]byte("\x1b[18~"))
	case "f8":
		ih.ptyWriter.Write([]byte("\x1b[19~"))
	case "f9":
		ih.ptyWriter.Write([]byte("\x1b[20~"))
	case "f10":
		ih.ptyWriter.Write([]byte("\x1b[21~"))
	case "f11":
		ih.ptyWriter.Write([]byte("\x1b[23~"))
	case "f12":
		ih.ptyWriter.Write([]byte("\x1b[24~"))
	default:
		// Handle ctrl key combinations
		if len(keyStr) > 5 && keyStr[:5] == "ctrl+" {
			if b, ok := ctrlKeyByteFromString(keyStr); ok {
				ih.ptyWriter.Write([]byte{b})
				return
			}
		}
		// For text input, send the text
		if key.Text != "" {
			ih.ptyWriter.Write([]byte(key.Text))
		} else if len(keyStr) > 0 {
			// Fallback to string representation
			ih.ptyWriter.Write([]byte(keyStr))
		}
	}
}

// ctrlKeyByteFromString converts a ctrl+X string to the corresponding byte
func ctrlKeyByteFromString(keyStr string) (byte, bool) {
	if len(keyStr) < 6 {
		return 0, false
	}
	char := keyStr[5:]
	switch char {
	case "@":
		return 0, true // ctrl+@
	case "a":
		return 1, true
	case "b":
		return 2, true
	case "c":
		return 3, true
	case "d":
		return 4, true
	case "e":
		return 5, true
	case "f":
		return 6, true
	case "g":
		return 7, true
	case "h":
		return 8, true
	case "i":
		return 9, true // tab
	case "j":
		return 10, true
	case "k":
		return 11, true
	case "l":
		return 12, true
	case "m":
		return 13, true // enter
	case "n":
		return 14, true
	case "o":
		return 15, true
	case "p":
		return 16, true
	case "q":
		return 17, true
	case "r":
		return 18, true
	case "s":
		return 19, true
	case "t":
		return 20, true
	case "u":
		return 21, true
	case "v":
		return 22, true
	case "w":
		return 23, true
	case "x":
		return 24, true
	case "y":
		return 25, true
	case "z":
		return 26, true
	case "[":
		return 27, true // esc
	case "\\":
		return 28, true
	case "]":
		return 29, true
	case "^":
		return 30, true
	case "_":
		return 31, true
	case "?":
		return 127, true
	}
	return 0, false
}
