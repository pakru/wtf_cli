package terminal

import (
	"testing"
)

func TestNewCursorTracker(t *testing.T) {
	ct := NewCursorTracker()

	if ct == nil {
		t.Fatal("NewCursorTracker() returned nil")
	}

	row, col := ct.GetPosition()
	if row != 0 || col != 0 {
		t.Errorf("Expected initial position (0,0), got (%d,%d)", row, col)
	}
}

func TestCursorTracker_RenderCursorOverlay_EmptyContent(t *testing.T) {
	ct := NewCursorTracker()

	result := ct.RenderCursorOverlay("", "█")

	if result != "█" {
		t.Errorf("Expected '█', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_SimpleContent(t *testing.T) {
	ct := NewCursorTracker()

	result := ct.RenderCursorOverlay("hello", "█")

	if result != "hello█" {
		t.Errorf("Expected 'hello█', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_WithTrailingNewline(t *testing.T) {
	ct := NewCursorTracker()

	result := ct.RenderCursorOverlay("hello\n", "█")

	// Should add cursor before the trailing newline
	if result != "hello█\n" {
		t.Errorf("Expected 'hello█\\n', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_MultipleTrailingNewlines(t *testing.T) {
	ct := NewCursorTracker()

	result := ct.RenderCursorOverlay("hello\n\n\n", "█")

	// Should preserve all trailing newlines
	if result != "hello█\n\n\n" {
		t.Errorf("Expected 'hello█\\n\\n\\n', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_MultiLine(t *testing.T) {
	ct := NewCursorTracker()

	content := "line1\nline2\nline3"
	result := ct.RenderCursorOverlay(content, "█")

	expected := "line1\nline2\nline3█"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_ShellPrompt(t *testing.T) {
	ct := NewCursorTracker()

	// Simulate a typical shell prompt
	content := "user@host:~/projects $ "
	result := ct.RenderCursorOverlay(content, "█")

	expected := "user@host:~/projects $ █"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_DifferentCursorChar(t *testing.T) {
	ct := NewCursorTracker()

	// Test with different cursor characters
	testCases := []struct {
		cursor   string
		expected string
	}{
		{"█", "hello█"},
		{"|", "hello|"},
		{"_", "hello_"},
		{"▌", "hello▌"},
	}

	for _, tc := range testCases {
		result := ct.RenderCursorOverlay("hello", tc.cursor)
		if result != tc.expected {
			t.Errorf("With cursor %q: expected %q, got %q", tc.cursor, tc.expected, result)
		}
	}
}

func TestCursorTracker_UpdateFromOutput_CarriageReturn(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 10 // Simulate cursor at column 10

	ct.UpdateFromOutput([]byte("\r"))

	_, col := ct.GetPosition()
	if col != 0 {
		t.Errorf("Expected col 0 after carriage return, got %d", col)
	}
}

func TestCursorTracker_UpdateFromOutput_Newline(t *testing.T) {
	ct := NewCursorTracker()
	ct.row = 5

	ct.UpdateFromOutput([]byte("\n"))

	row, _ := ct.GetPosition()
	if row != 6 {
		t.Errorf("Expected row 6 after newline, got %d", row)
	}
}

func TestCursorTracker_UpdateFromOutput_AbsolutePosition(t *testing.T) {
	ct := NewCursorTracker()

	// ESC[5;10H - move to row 5, column 10 (1-indexed)
	ct.UpdateFromOutput([]byte("\x1b[5;10H"))

	row, col := ct.GetPosition()
	if row != 4 || col != 9 { // 0-indexed
		t.Errorf("Expected position (4,9), got (%d,%d)", row, col)
	}
}

func TestCursorTracker_UpdateFromOutput_Backspace(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 5

	ct.UpdateFromOutput([]byte("\x7f")) // DEL character

	_, col := ct.GetPosition()
	if col != 4 {
		t.Errorf("Expected col 4 after backspace, got %d", col)
	}
}

func TestCursorTracker_UpdateFromOutput_BackspaceAtStart(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 0

	ct.UpdateFromOutput([]byte("\x7f"))

	_, col := ct.GetPosition()
	if col != 0 {
		t.Errorf("Expected col 0 (can't go negative), got %d", col)
	}
}

func TestCursorTracker_UpdateFromOutput_CursorForward(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 5

	// ESC[3C - move cursor forward 3 positions
	ct.UpdateFromOutput([]byte("\x1b[3C"))

	_, col := ct.GetPosition()
	if col != 8 {
		t.Errorf("Expected col 8 after forward 3, got %d", col)
	}
}

func TestCursorTracker_UpdateFromOutput_CursorBack(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 10

	// ESC[4D - move cursor back 4 positions
	ct.UpdateFromOutput([]byte("\x1b[4D"))

	_, col := ct.GetPosition()
	if col != 6 {
		t.Errorf("Expected col 6 after back 4, got %d", col)
	}
}
