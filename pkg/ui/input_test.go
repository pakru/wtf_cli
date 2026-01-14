package ui

import (
	"bytes"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewInputHandler(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	if ih == nil {
		t.Fatal("NewInputHandler() returned nil")
	}
	
	if ih.ptyWriter == nil {
		t.Error("Expected ptyWriter to be set")
	}
}

func TestInputHandler_HandleKey_CtrlC(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	handled, _ := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected Ctrl+C to be handled")
	}
	
	// Should send ASCII 3 (ETX)
	if buf.Bytes()[0] != 3 {
		t.Errorf("Expected byte 3, got %d", buf.Bytes()[0])
	}
}

func TestInputHandler_HandleKey_CtrlD(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	msg := tea.KeyMsg{Type: tea.KeyCtrlD}
	handled, cmd := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected Ctrl+D to be handled")
	}
	
	// Should send ASCII 4 (EOT)
	if buf.Bytes()[0] != 4 {
		t.Errorf("Expected byte 4, got %d", buf.Bytes()[0])
	}
	
	// Should return Quit command
	if cmd == nil {
		t.Error("Expected Quit command for Ctrl+D")
	}
}

func TestInputHandler_HandleKey_Enter(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	handled, _ := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected Enter to be handled")
	}
	
	// Should send CR (13)
	if buf.Bytes()[0] != 13 {
		t.Errorf("Expected byte 13, got %d", buf.Bytes()[0])
	}
}

func TestInputHandler_HandleKey_Tab(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	msg := tea.KeyMsg{Type: tea.KeyTab}
	handled, _ := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected Tab to be handled")
	}
	
	// Should send ASCII 9
	if buf.Bytes()[0] != 9 {
		t.Errorf("Expected byte 9, got %d", buf.Bytes()[0])
	}
}

func TestInputHandler_HandleKey_Backspace(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	msg := tea.KeyMsg{Type: tea.KeyBackspace}
	handled, _ := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected Backspace to be handled")
	}
	
	// Should send ASCII 127 (DEL)
	if buf.Bytes()[0] != 127 {
		t.Errorf("Expected byte 127, got %d", buf.Bytes()[0])
	}
}

func TestInputHandler_HandleKey_ArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		keyType  tea.KeyType
		expected string
	}{
		{"Up", tea.KeyUp, "\x1b[A"},
		{"Down", tea.KeyDown, "\x1b[B"},
		{"Right", tea.KeyRight, "\x1b[C"},
		{"Left", tea.KeyLeft, "\x1b[D"},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ih := NewInputHandler(buf)
			
			msg := tea.KeyMsg{Type: tt.keyType}
			handled, _ := ih.HandleKey(msg)
			
			if !handled {
				t.Errorf("Expected %s to be handled", tt.name)
			}
			
			if buf.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, buf.String())
			}
		})
	}
}

func TestInputHandler_HandleKey_NormalTyping(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	msg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'a'},
	}
	handled, _ := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected normal key to be handled")
	}
	
	// msg.String() for this should be "a"
	if buf.Len() == 0 {
		t.Error("Expected output in buffer")
	}
}

func TestInputHandler_SendToPTY(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	data := []byte("test data")
	err := ih.SendToPTY(data)
	
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	
	if buf.String() != "test data" {
		t.Errorf("Expected 'test data', got %q", buf.String())
	}
}

func TestInputHandler_HandleKey_SlashAtLineStart(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	// At line start (initial state), / should trigger palette
	msg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'/'},
	}
	handled, cmd := ih.HandleKey(msg)
	
	if !handled {
		t.Error("Expected / at line start to be handled")
	}
	
	// Should NOT send to PTY (triggers palette instead)
	if buf.Len() != 0 {
		t.Errorf("Expected empty buffer (palette triggered), got %q", buf.String())
	}

	// Should return a command (to show palette)
	if cmd == nil {
		t.Error("Expected command to show palette")
	}
}

func TestInputHandler_HandleKey_SlashMidLine(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)
	
	// Type something first to not be at line start
	typingMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'e', 'c', 'h', 'o', ' '},
	}
	ih.HandleKey(typingMsg)
	buf.Reset() // Clear the typed chars
	
	// Now / should be sent to PTY (not at line start)
	slashMsg := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune{'/'},
	}
	handled, cmd := ih.HandleKey(slashMsg)
	
	if !handled {
		t.Error("Expected / mid-line to be handled")
	}
	
	// Should send to PTY
	if buf.String() != "/" {
		t.Errorf("Expected '/', got %q", buf.String())
	}

	// Should NOT return palette command
	if cmd != nil {
		t.Error("Should not trigger palette when / is mid-line")
	}
}

