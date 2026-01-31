package input

import (
	"context"
	"io"
	"log/slog"
	"strings"

	"wtf_cli/pkg/logging"

	tea "charm.land/bubbletea/v2"
)

// InputHandler manages keyboard input routing
type InputHandler struct {
	ptyWriter         io.Writer
	atLineStart       bool   // Track if cursor is at start of line (for / detection)
	lineBuffer        string // Track current line text for Ctrl+R initial filter
	paletteMode       bool   // True when command palette is active
	historyPickerMode bool   // True when history picker is active
	fullScreenMode    bool   // True when full-screen app (vim, nano) is active

	cursorKeysAppMode  bool
	keypadAppMode      bool
	bracketedPasteMode bool
	modePending        []byte
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

// SetHistoryPickerMode sets whether the history picker is active
func (ih *InputHandler) SetHistoryPickerMode(active bool) {
	ih.historyPickerMode = active
}

// IsHistoryPickerMode returns whether the history picker is active
func (ih *InputHandler) IsHistoryPickerMode() bool {
	return ih.historyPickerMode
}

// SetLineBuffer sets the current line buffer and updates line start tracking.
func (ih *InputHandler) SetLineBuffer(text string) {
	ih.lineBuffer = text
	ih.atLineStart = len(text) == 0
}

// ClearLineBuffer clears the internal line buffer.
// Used when echo is disabled (password entry) to prevent capturing secrets.
func (ih *InputHandler) ClearLineBuffer() {
	ih.lineBuffer = ""
	ih.atLineStart = true
}

// ShowPaletteMsg is sent when / is pressed at line start
type ShowPaletteMsg struct{}

// ShowHistoryPickerMsg is sent when Ctrl+R is pressed
type ShowHistoryPickerMsg struct {
	InitialFilter string // Pre-typed text to use as initial filter (empty for now)
}

// CommandSubmittedMsg is sent when the user submits a command line (Enter).
type CommandSubmittedMsg struct {
	Command string
}

// ToggleChatMsg is sent when Ctrl+T is pressed to toggle chat sidebar
type ToggleChatMsg struct{}

type CtrlDPressedMsg struct{}

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

	// If history picker mode is active, don't process keys here
	// (they should be handled by the picker)
	if ih.historyPickerMode {
		return false, nil
	}

	keyStr := msg.String()

	cursorSeq := func(normal, app string) []byte {
		if ih.cursorKeysAppMode {
			return []byte(app)
		}
		return []byte(normal)
	}

	// Check for special keys first using string matching (v2 API)
	switch keyStr {
	case "ctrl+c":
		// Ctrl+C - send interrupt to PTY
		ih.ptyWriter.Write([]byte{3}) // ASCII ETX (Ctrl+C)
		ih.atLineStart = true         // After interrupt, usually at new prompt
		ih.lineBuffer = ""            // Clear line buffer on interrupt
		return true, nil

	case "ctrl+d":
		return true, func() tea.Msg {
			return CtrlDPressedMsg{}
		}

	case "ctrl+t":
		// Ctrl+T - toggle chat sidebar
		return true, func() tea.Msg {
			return ToggleChatMsg{}
		}

	case "ctrl+r":
		// Ctrl+R - trigger history picker with current line as initial filter
		initFilter := ih.lineBuffer
		return true, func() tea.Msg {
			return ShowHistoryPickerMsg{InitialFilter: initFilter}
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
		// Enter - send newline to PTY and emit command submission
		submitted := ih.lineBuffer
		ih.ptyWriter.Write([]byte{13}) // CR (some shells need this)
		ih.atLineStart = true          // After enter, we're at new line start
		ih.lineBuffer = ""             // Clear line buffer on enter
		return true, func() tea.Msg {
			return CommandSubmittedMsg{Command: submitted}
		}

	case "backspace":
		// Backspace - send to PTY
		ih.ptyWriter.Write([]byte{127}) // ASCII DEL
		// Remove last character from line buffer
		if len(ih.lineBuffer) > 0 {
			ih.lineBuffer = ih.lineBuffer[:len(ih.lineBuffer)-1]
			ih.atLineStart = len(ih.lineBuffer) == 0
		}
		return true, nil

	case "delete":
		// Delete - send to PTY
		ih.ptyWriter.Write([]byte("\x1b[3~"))
		return true, nil

	case " ":
		// Space - send to PTY
		ih.ptyWriter.Write([]byte{32}) // ASCII SPACE
		ih.lineBuffer += " "
		ih.atLineStart = false
		return true, nil

	case "esc":
		// Escape - send to PTY
		ih.ptyWriter.Write([]byte{27}) // ASCII ESC
		return true, nil

	case "up":
		ih.ptyWriter.Write(cursorSeq("\x1b[A", "\x1bOA"))
		return true, nil
	case "down":
		ih.ptyWriter.Write(cursorSeq("\x1b[B", "\x1bOB"))
		return true, nil
	case "right":
		ih.ptyWriter.Write(cursorSeq("\x1b[C", "\x1bOC"))
		return true, nil
	case "left":
		ih.ptyWriter.Write(cursorSeq("\x1b[D", "\x1bOD"))
		return true, nil
	case "home":
		ih.ptyWriter.Write(cursorSeq("\x1b[H", "\x1bOH"))
		return true, nil
	case "end":
		ih.ptyWriter.Write(cursorSeq("\x1b[F", "\x1bOF"))
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
			return ShowPaletteMsg{}
		}
	}

	// Normal "/" in the middle of typing - send to PTY
	if keyStr == "/" {
		ih.ptyWriter.Write([]byte("/"))
		ih.lineBuffer += "/"
		ih.atLineStart = false
		return true, nil
	}

	// Normal typing - send to PTY if it's printable text
	key := msg.Key()
	if key.Text != "" {
		ih.ptyWriter.Write([]byte(key.Text))
		ih.lineBuffer += key.Text
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
			if i+2 >= len(combined) {
				ih.modePending = append(ih.modePending, combined[i:]...)
				i = len(combined)
				break
			}
			if combined[i+2] == '?' {
				j := i + 3
				for j < len(combined) && combined[j] >= '0' && combined[j] <= '9' {
					j++
				}
				if j >= len(combined) {
					ih.modePending = append(ih.modePending, combined[i:]...)
					i = len(combined)
					break
				}
				if combined[j] == 'h' || combined[j] == 'l' {
					enable := combined[j] == 'h'
					digits := combined[i+3 : j]
					if len(digits) == 1 && digits[0] == '1' {
						ih.cursorKeysAppMode = enable
					} else if len(digits) == 4 &&
						digits[0] == '2' && digits[1] == '0' && digits[2] == '0' && digits[3] == '4' {
						if ih.bracketedPasteMode != enable {
							ih.bracketedPasteMode = enable
							logger := slog.Default()
							ctx := context.Background()
							if logger.Enabled(ctx, logging.LevelTrace) {
								logger.Log(ctx, logging.LevelTrace, "bracketed_paste_mode", "enabled", enable)
							}
						}
					}
					i = j
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

// HandlePaste sends pasted content to the PTY, wrapping with bracketed paste
// delimiters when enabled, and updates line buffer tracking.
func (ih *InputHandler) HandlePaste(content string) {
	if content == "" {
		return
	}

	logger := slog.Default()
	ctx := context.Background()
	if logger.Enabled(ctx, logging.LevelTrace) {
		logger.Log(ctx, logging.LevelTrace, "paste_to_pty", "len", len(content), "bracketed", ih.bracketedPasteMode)
	}
	if ih.bracketedPasteMode {
		ih.ptyWriter.Write([]byte("\x1b[200~"))
	}
	if len(content) > 0 {
		ih.ptyWriter.Write([]byte(content))
	}
	if ih.bracketedPasteMode {
		ih.ptyWriter.Write([]byte("\x1b[201~"))
	}

	lastNL := strings.LastIndexAny(content, "\n\r")
	if lastNL == -1 {
		ih.lineBuffer += content
		ih.atLineStart = false
		return
	}

	ih.lineBuffer = content[lastNL+1:]
	ih.atLineStart = len(ih.lineBuffer) == 0
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
