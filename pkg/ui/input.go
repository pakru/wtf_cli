package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
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
func (ih *InputHandler) HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
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

	// Check for special keys first
	switch msg.Type {
	case tea.KeyCtrlC:
		// Ctrl+C - send interrupt to PTY
		ih.ptyWriter.Write([]byte{3}) // ASCII ETX (Ctrl+C)
		ih.atLineStart = true         // After interrupt, usually at new prompt
		return true, nil

	case tea.KeyCtrlD:
		return true, func() tea.Msg {
			return ctrlDPressedMsg{}
		}

	case tea.KeyCtrlZ:
		// Ctrl+Z - suspend (send to PTY)
		ih.ptyWriter.Write([]byte{26}) // ASCII SUB (Ctrl+Z)
		return true, nil

	case tea.KeyTab:
		// Tab - send to PTY
		ih.ptyWriter.Write([]byte{9}) // ASCII TAB
		ih.atLineStart = false
		return true, nil

	case tea.KeyEnter:
		// Enter - send newline to PTY
		ih.ptyWriter.Write([]byte{13}) // CR (some shells need this)
		ih.atLineStart = true          // After enter, we're at new line start
		return true, nil

	case tea.KeyBackspace:
		// Backspace - send to PTY
		ih.ptyWriter.Write([]byte{127}) // ASCII DEL
		// We can't perfectly track if we're at line start after backspace
		// but it's conservative to keep atLineStart false
		return true, nil

	case tea.KeySpace:
		// Space - send to PTY
		ih.ptyWriter.Write([]byte{32}) // ASCII SPACE
		ih.atLineStart = false
		return true, nil

	case tea.KeyEsc:
		// Escape - send to PTY
		ih.ptyWriter.Write([]byte{27}) // ASCII ESC
		return true, nil
	}

	if b, ok := ctrlKeyByte(msg.Type); ok {
		ih.ptyWriter.Write([]byte{b})
		if b == '\r' || b == '\n' {
			ih.atLineStart = true
		} else {
			ih.atLineStart = false
		}
		return true, nil
	}

	// Check for slash at line start - trigger command palette
	keyStr := msg.String()
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

	// Normal typing - send to PTY
	if msg.Type == tea.KeyRunes {
		ih.ptyWriter.Write([]byte(keyStr))
		ih.atLineStart = false
		return true, nil
	}

	// Arrow keys and other special keys - generate ANSI escape sequences
	switch msg.Type {
	case tea.KeyUp:
		ih.ptyWriter.Write([]byte("\x1b[A"))
		return true, nil
	case tea.KeyDown:
		ih.ptyWriter.Write([]byte("\x1b[B"))
		return true, nil
	case tea.KeyRight:
		ih.ptyWriter.Write([]byte("\x1b[C"))
		return true, nil
	case tea.KeyLeft:
		ih.ptyWriter.Write([]byte("\x1b[D"))
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
func (ih *InputHandler) sendKeyToPTY(msg tea.KeyMsg) {
	cursorSeq := func(normal, app string) []byte {
		if ih.cursorKeysAppMode {
			return []byte(app)
		}
		return []byte(normal)
	}

	switch msg.Type {
	case tea.KeyCtrlC:
		ih.ptyWriter.Write([]byte{3}) // ASCII ETX (Ctrl+C)
	case tea.KeyCtrlD:
		ih.ptyWriter.Write([]byte{4}) // ASCII EOT (Ctrl+D)
	case tea.KeyCtrlZ:
		ih.ptyWriter.Write([]byte{26}) // ASCII SUB (Ctrl+Z)
	case tea.KeyTab:
		ih.ptyWriter.Write([]byte{9}) // ASCII TAB
	case tea.KeyEnter:
		ih.ptyWriter.Write([]byte{13}) // CR
	case tea.KeyBackspace:
		ih.ptyWriter.Write([]byte{127}) // ASCII DEL
	case tea.KeySpace:
		ih.ptyWriter.Write([]byte{32}) // ASCII SPACE
	case tea.KeyEsc:
		ih.ptyWriter.Write([]byte{27}) // ASCII ESC
	case tea.KeyUp:
		ih.ptyWriter.Write(cursorSeq("\x1b[A", "\x1bOA"))
	case tea.KeyDown:
		ih.ptyWriter.Write(cursorSeq("\x1b[B", "\x1bOB"))
	case tea.KeyRight:
		ih.ptyWriter.Write(cursorSeq("\x1b[C", "\x1bOC"))
	case tea.KeyLeft:
		ih.ptyWriter.Write(cursorSeq("\x1b[D", "\x1bOD"))
	case tea.KeyHome:
		ih.ptyWriter.Write(cursorSeq("\x1b[H", "\x1bOH"))
	case tea.KeyEnd:
		ih.ptyWriter.Write(cursorSeq("\x1b[F", "\x1bOF"))
	case tea.KeyPgUp:
		ih.ptyWriter.Write([]byte("\x1b[5~"))
	case tea.KeyPgDown:
		ih.ptyWriter.Write([]byte("\x1b[6~"))
	case tea.KeyDelete:
		ih.ptyWriter.Write([]byte("\x1b[3~"))
	case tea.KeyInsert:
		ih.ptyWriter.Write([]byte("\x1b[2~"))
	case tea.KeyF1:
		ih.ptyWriter.Write([]byte("\x1bOP"))
	case tea.KeyF2:
		ih.ptyWriter.Write([]byte("\x1bOQ"))
	case tea.KeyF3:
		ih.ptyWriter.Write([]byte("\x1bOR"))
	case tea.KeyF4:
		ih.ptyWriter.Write([]byte("\x1bOS"))
	case tea.KeyF5:
		ih.ptyWriter.Write([]byte("\x1b[15~"))
	case tea.KeyF6:
		ih.ptyWriter.Write([]byte("\x1b[17~"))
	case tea.KeyF7:
		ih.ptyWriter.Write([]byte("\x1b[18~"))
	case tea.KeyF8:
		ih.ptyWriter.Write([]byte("\x1b[19~"))
	case tea.KeyF9:
		ih.ptyWriter.Write([]byte("\x1b[20~"))
	case tea.KeyF10:
		ih.ptyWriter.Write([]byte("\x1b[21~"))
	case tea.KeyF11:
		ih.ptyWriter.Write([]byte("\x1b[23~"))
	case tea.KeyF12:
		ih.ptyWriter.Write([]byte("\x1b[24~"))
	case tea.KeyRunes:
		ih.ptyWriter.Write([]byte(msg.String()))
	default:
		if b, ok := ctrlKeyByte(msg.Type); ok {
			ih.ptyWriter.Write([]byte{b})
			return
		}
		// For other keys, try to send the string representation
		if s := msg.String(); len(s) > 0 {
			ih.ptyWriter.Write([]byte(s))
		}
	}
}

func ctrlKeyByte(key tea.KeyType) (byte, bool) {
	if key >= tea.KeyCtrlAt && key <= tea.KeyCtrlUnderscore {
		return byte(key), true
	}
	if key == tea.KeyCtrlQuestionMark {
		return 127, true
	}
	return 0, false
}
