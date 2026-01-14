package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// InputHandler manages keyboard input routing
type InputHandler struct {
	ptyWriter   io.Writer
	atLineStart bool // Track if cursor is at start of line (for / detection)
	paletteMode bool // True when command palette is active
}

// NewInputHandler creates a new input handler
func NewInputHandler(ptyWriter io.Writer) *InputHandler {
	return &InputHandler{
		ptyWriter:   ptyWriter,
		atLineStart: true, // Start at line start (fresh prompt)
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

// HandleKey processes a key message and returns whether it was handled
func (ih *InputHandler) HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
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
		// Ctrl+D - send EOF to PTY (will trigger shell exit)
		ih.ptyWriter.Write([]byte{4}) // ASCII EOT (Ctrl+D)
		return true, tea.Quit

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

// SendToPTY sends raw bytes to the PTY
func (ih *InputHandler) SendToPTY(data []byte) error {
	_, err := ih.ptyWriter.Write(data)
	return err
}

// ResetLineStart resets the line start tracker (called after PTY output)
func (ih *InputHandler) ResetLineStart() {
	// Don't auto-reset - we track based on user input
}
