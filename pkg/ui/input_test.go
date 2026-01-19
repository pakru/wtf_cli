package ui

import (
	"bytes"
	"testing"

	"wtf_cli/pkg/ui/components/testutils"

	tea "charm.land/bubbletea/v2"
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

	msg := testutils.TestKeyCtrlC
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

	msg := testutils.TestKeyCtrlD
	handled, cmd := ih.HandleKey(msg)

	if !handled {
		t.Error("Expected Ctrl+D to be handled")
	}

	if len(buf.Bytes()) != 0 {
		t.Errorf("Expected no PTY output, got %v", buf.Bytes())
	}

	// Should return ctrlDPressedMsg command
	if cmd == nil {
		t.Error("Expected ctrlDPressedMsg command for Ctrl+D")
	}

	if cmd != nil {
		msg := cmd()
		if _, ok := msg.(ctrlDPressedMsg); !ok {
			t.Errorf("Expected ctrlDPressedMsg, got %T", msg)
		}
	}
}

func TestInputHandler_HandleKey_CtrlW(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	msg := testutils.TestKeyCtrlW
	handled, cmd := ih.HandleKey(msg)

	if !handled {
		t.Error("Expected Ctrl+W to be handled")
	}

	if cmd != nil {
		t.Error("Expected no command for Ctrl+W")
	}

	if buf.Len() != 1 || buf.Bytes()[0] != 23 {
		t.Errorf("Expected byte 23 (ETB), got %v", buf.Bytes())
	}
}

func TestInputHandler_HandleKey_Enter(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	input := testutils.TestKeyEnter
	handled, _ := ih.HandleKey(input)

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

	msg := testutils.TestKeyTab
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

	msg := testutils.TestKeyBackspace
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
		msg      tea.KeyPressMsg
		expected string
	}{
		{"Up", testutils.TestKeyUp, "\x1b[A"},
		{"Down", testutils.TestKeyDown, "\x1b[B"},
		{"Right", testutils.TestKeyRight, "\x1b[C"},
		{"Left", testutils.TestKeyLeft, "\x1b[D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ih := NewInputHandler(buf)

			handled, _ := ih.HandleKey(tt.msg)

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

	input := testutils.NewTextKeyPressMsg("a")
	handled, _ := ih.HandleKey(input)

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
	msg := testutils.NewTextKeyPressMsg("/")
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
	typingMsg := testutils.NewTextKeyPressMsg("echo ")
	ih.HandleKey(typingMsg)
	buf.Reset() // Clear the typed chars

	// Now / should be sent to PTY (not at line start)
	slashMsg := testutils.NewTextKeyPressMsg("/")
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

func TestInputHandler_FullScreenMode_BypassesSlashPalette(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	// Enable full-screen mode
	ih.SetFullScreenMode(true)

	// At line start (initial state), / should go to PTY, not palette
	// Test Up Arrow
	msg := testutils.NewTextKeyPressMsg("/")
	handled, cmd := ih.HandleKey(msg)

	if !handled {
		t.Error("Expected / to be handled in fullscreen mode")
	}

	// Should send to PTY (not trigger palette)
	if buf.String() != "/" {
		t.Errorf("Expected '/' sent to PTY, got %q", buf.String())
	}

	// Should NOT return palette command
	if cmd != nil {
		t.Error("Should not trigger palette in fullscreen mode")
	}
}

func TestInputHandler_FullScreenMode_BypassesCtrlD(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	// Enable full-screen mode
	ih.SetFullScreenMode(true)

	// Ctrl+D should go to PTY, not trigger exit confirmation
	msg := testutils.TestKeyCtrlD
	handled, cmd := ih.HandleKey(msg)

	if !handled {
		t.Error("Expected Ctrl+D to be handled in fullscreen mode")
	}

	// Should send ASCII 4 (EOT) to PTY
	if buf.Len() != 1 || buf.Bytes()[0] != 4 {
		t.Errorf("Expected byte 4 (EOT), got %v", buf.Bytes())
	}

	// Should NOT return ctrlDPressedMsg
	if cmd != nil {
		t.Error("Should not return ctrlDPressedMsg in fullscreen mode")
	}
}

func TestInputHandler_FullScreenMode_CtrlX(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	ih.SetFullScreenMode(true)

	msg := testutils.TestKeyCtrlX
	handled, _ := ih.HandleKey(msg)

	if !handled {
		t.Error("Expected Ctrl+X to be handled in fullscreen mode")
	}

	if buf.Len() != 1 || buf.Bytes()[0] != 24 {
		t.Errorf("Expected byte 24 (CAN), got %v", buf.Bytes())
	}
}

func TestInputHandler_SetFullScreenMode(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	if ih.IsFullScreenMode() {
		t.Error("Should not be in fullscreen mode initially")
	}

	ih.SetFullScreenMode(true)
	if !ih.IsFullScreenMode() {
		t.Error("Should be in fullscreen mode after SetFullScreenMode(true)")
	}

	ih.SetFullScreenMode(false)
	if ih.IsFullScreenMode() {
		t.Error("Should not be in fullscreen mode after SetFullScreenMode(false)")
	}
}

func TestInputHandler_FullScreenMode_ArrowKeys(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyPressMsg
		expected string
	}{
		{"Up", testutils.TestKeyUp, "\x1b[A"},
		{"Down", testutils.TestKeyDown, "\x1b[B"},
		{"Right", testutils.TestKeyRight, "\x1b[C"},
		{"Left", testutils.TestKeyLeft, "\x1b[D"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ih := NewInputHandler(buf)
			ih.SetFullScreenMode(true)

			handled, _ := ih.HandleKey(tt.msg)

			if !handled {
				t.Errorf("Expected %s to be handled in fullscreen mode", tt.name)
			}

			if buf.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, buf.String())
			}
		})
	}
}

func TestInputHandler_UpdateTerminalModes_CursorKeys(t *testing.T) {
	buf := &bytes.Buffer{}
	ih := NewInputHandler(buf)

	ih.UpdateTerminalModes([]byte("\x1b[?1h"))
	if !ih.cursorKeysAppMode {
		t.Error("Expected cursor keys app mode to be enabled")
	}

	ih.UpdateTerminalModes([]byte("\x1b[?1l"))
	if ih.cursorKeysAppMode {
		t.Error("Expected cursor keys app mode to be disabled")
	}
}

func TestInputHandler_FullScreenMode_ArrowKeys_AppMode(t *testing.T) {
	tests := []struct {
		name     string
		msg      tea.KeyPressMsg
		expected string
	}{
		{"Up", testutils.TestKeyUp, "\x1bOA"},
		{"Down", testutils.TestKeyDown, "\x1bOB"},
		{"Right", testutils.TestKeyRight, "\x1bOC"},
		{"Left", testutils.TestKeyLeft, "\x1bOD"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			ih := NewInputHandler(buf)
			ih.SetFullScreenMode(true)
			ih.UpdateTerminalModes([]byte("\x1b[?1h"))

			handled, _ := ih.HandleKey(tt.msg)

			if !handled {
				t.Errorf("Expected %s to be handled in fullscreen mode", tt.name)
			}

			if buf.String() != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, buf.String())
			}
		})
	}
}
