package commands

import (
	"testing"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
)

func TestNewContext(t *testing.T) {
	buf := buffer.New(100)
	sess := capture.NewSessionContext()
	ctx := NewContext(buf, sess, "/home/user")

	if ctx.Buffer == nil {
		t.Error("Expected Buffer to be set")
	}
	if ctx.Session == nil {
		t.Error("Expected Session to be set")
	}
	if ctx.CurrentDir != "/home/user" {
		t.Errorf("Expected '/home/user', got %q", ctx.CurrentDir)
	}
}

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()

	if d == nil {
		t.Fatal("NewDispatcher() returned nil")
	}

	// Check all commands are registered
	commands := []string{"/chat", "/explain", "/history", "/settings", "/help"}
	for _, cmd := range commands {
		if _, ok := d.GetHandler(cmd); !ok {
			t.Errorf("Expected handler for %s to be registered", cmd)
		}
	}
}

func TestDispatcher_Dispatch_UnknownCommand(t *testing.T) {
	d := NewDispatcher()
	ctx := NewContext(nil, nil, "")

	result := d.Dispatch("/unknown", ctx)

	if result == nil {
		t.Fatal("Expected result for unknown command")
	}
	if result.Title != "Error" {
		t.Errorf("Expected title 'Error', got %q", result.Title)
	}
}

func TestDispatcher_Dispatch_ExplainCommand(t *testing.T) {
	d := NewDispatcher()
	buf := buffer.New(100)
	ctx := NewContext(buf, nil, "/tmp")

	result := d.Dispatch("/explain", ctx)

	if result == nil {
		t.Fatal("Expected result for /explain command")
	}
	if result.Title != "WTF Analysis" {
		t.Errorf("Expected title 'WTF Analysis', got %q", result.Title)
	}
}

func TestDispatcher_Dispatch_HelpCommand(t *testing.T) {
	d := NewDispatcher()
	ctx := NewContext(nil, nil, "")

	result := d.Dispatch("/help", ctx)

	if result == nil {
		t.Fatal("Expected result for /help command")
	}
	if result.Title != "Help" {
		t.Errorf("Expected title 'Help', got %q", result.Title)
	}
}

func TestContext_GetLastNLines_NilBuffer(t *testing.T) {
	ctx := NewContext(nil, nil, "")

	lines := ctx.GetLastNLines(10)

	if lines != nil {
		t.Error("Expected nil for nil buffer")
	}
}

func TestContext_GetLastNLines_WithBuffer(t *testing.T) {
	buf := buffer.New(100)
	buf.Write([]byte("line 1"))
	buf.Write([]byte("line 2"))
	buf.Write([]byte("line 3"))

	ctx := NewContext(buf, nil, "")
	lines := ctx.GetLastNLines(2)

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
}

func TestDispatcher_Dispatch_ChatCommand(t *testing.T) {
	d := NewDispatcher()
	ctx := NewContext(nil, nil, "")

	result := d.Dispatch("/chat", ctx)

	if result == nil {
		t.Fatal("Expected result for /chat command")
	}
	if result.Title != "Chat" {
		t.Errorf("Expected title 'Chat', got %q", result.Title)
	}
	if result.Action != ResultActionToggleChat {
		t.Errorf("Expected action ResultActionToggleChat, got %q", result.Action)
	}
}
