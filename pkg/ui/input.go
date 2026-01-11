package ui

import (
	"io"

	tea "github.com/charmbracelet/bubbletea"
)

// InputHandler manages keyboard input routing
type InputHandler struct {
	ptyWriter io.Writer
}

// NewInputHandler creates a new input handler
func NewInputHandler(ptyWriter io.Writer) *InputHandler {
	return &InputHandler{
		ptyWriter: ptyWriter,
	}
}

// HandleKey processes a key message and returns whether it was handled
func (ih *InputHandler) HandleKey(msg tea.KeyMsg) (handled bool, cmd tea.Cmd) {
	// Check for special keys first
	switch msg.Type {
	case tea.KeyCtrlC:
		// Ctrl+C - send interrupt to PTY
		ih.ptyWriter.Write([]byte{3}) // ASCII ETX (Ctrl+C)
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
		return true, nil
		
	case tea.KeyEnter:
		// Enter - send newline to PTY
		ih.ptyWriter.Write([]byte{13}) // CR (some shells need this)
		return true, nil
		
	case tea.KeyBackspace:
		// Backspace - send to PTY
		ih.ptyWriter.Write([]byte{127}) // ASCII DEL
		return true, nil
		
	case tea.KeyEsc:
		// Escape - send to PTY
		ih.ptyWriter.Write([]byte{27}) // ASCII ESC
		return true, nil
	}
	
	// Check for special intercepted keys
	keyStr := msg.String()
	
	// Intercept "/" for command palette (future implementation)
	if keyStr == "/" {
		// For now, just send to PTY
		// In Phase 5, we'll intercept this to show command palette
		ih.ptyWriter.Write([]byte("/"))
		return true, nil
	}
	
	// Normal typing - send to PTY
	if msg.Type == tea.KeyRunes {
		ih.ptyWriter.Write([]byte(keyStr))
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

// SendToP PTY sends raw bytes to the PTY
func (ih *InputHandler) SendToPTY(data []byte) error {
	_, err := ih.ptyWriter.Write(data)
	return err
}
