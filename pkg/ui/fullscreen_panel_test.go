package ui

import (
	"strings"
	"testing"
)

func TestNewFullScreenPanel(t *testing.T) {
	p := NewFullScreenPanel(80, 24)
	if p == nil {
		t.Fatal("NewFullScreenPanel returned nil")
	}

	w, h := p.Size()
	if w != 80 || h != 24 {
		t.Errorf("Expected size 80x24, got %dx%d", w, h)
	}
}

func TestFullScreenPanel_WriteAndView(t *testing.T) {
	p := NewFullScreenPanel(40, 10)

	// Write some text
	_, err := p.Write([]byte("Hello, World!"))
	if err != nil {
		t.Fatalf("Write error: %v", err)
	}

	view := p.View()
	if !strings.Contains(view, "Hello, World!") {
		t.Errorf("View should contain 'Hello, World!', got: %s", view)
	}
}

func TestFullScreenPanel_Resize(t *testing.T) {
	p := NewFullScreenPanel(80, 24)

	p.Resize(100, 30)

	w, h := p.Size()
	if w != 100 || h != 30 {
		t.Errorf("Expected size 100x30, got %dx%d", w, h)
	}
}

func TestFullScreenPanel_ShowHide(t *testing.T) {
	p := NewFullScreenPanel(80, 24)

	if p.IsVisible() {
		t.Error("Panel should not be visible initially")
	}

	p.Show()
	if !p.IsVisible() {
		t.Error("Panel should be visible after Show()")
	}

	p.Hide()
	if p.IsVisible() {
		t.Error("Panel should not be visible after Hide()")
	}
}

func TestFullScreenPanel_Reset(t *testing.T) {
	p := NewFullScreenPanel(40, 10)

	// Write some text
	p.Write([]byte("Test content"))

	// Reset
	p.Reset()

	// After reset, view should be mostly empty
	view := p.View()
	// The view might contain spaces, but shouldn't contain test content
	if strings.Contains(view, "Test content") {
		t.Error("View should not contain test content after Reset()")
	}
}

func TestFullScreenPanel_GetCursor(t *testing.T) {
	p := NewFullScreenPanel(80, 24)

	// Initial cursor should be at 0,0
	row, col := p.GetCursor()
	if row != 0 || col != 0 {
		t.Errorf("Initial cursor should be at 0,0, got %d,%d", row, col)
	}

	// Write some text - cursor should move
	p.Write([]byte("Hello"))
	row, col = p.GetCursor()
	if col < 5 {
		t.Errorf("Cursor column should be at least 5 after writing 'Hello', got %d", col)
	}
}

func TestVisibleWidth(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  int
	}{
		{"plain text", "Hello", 5},
		{"with ANSI color", "\x1b[31mRed\x1b[0m", 3},
		{"with bold", "\x1b[1mBold\x1b[0m", 4},
		{"empty", "", 0},
		{"multiple escapes", "\x1b[31m\x1b[1mBold Red\x1b[0m", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := visibleWidth(tt.input)
			if got != tt.want {
				t.Errorf("visibleWidth(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
