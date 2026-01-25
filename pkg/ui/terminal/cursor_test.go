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
	ct.SetPosition(0, len("hello"))

	result := ct.RenderCursorOverlay("hello", "█")

	if result != "hello█" {
		t.Errorf("Expected 'hello█', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_WithTrailingNewline(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, len("hello"))

	result := ct.RenderCursorOverlay("hello\n", "█")

	// Should add cursor before the trailing newline
	if result != "hello█\n" {
		t.Errorf("Expected 'hello█\\n', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_MultipleTrailingNewlines(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, len("hello"))

	result := ct.RenderCursorOverlay("hello\n\n\n", "█")

	// Should preserve all trailing newlines
	if result != "hello█\n\n\n" {
		t.Errorf("Expected 'hello█\\n\\n\\n', got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_MultiLine(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(2, len("line3"))

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
	ct.SetPosition(0, len(content))
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
		ct.SetPosition(0, len("hello"))
		result := ct.RenderCursorOverlay("hello", tc.cursor)
		if result != tc.expected {
			t.Errorf("With cursor %q: expected %q, got %q", tc.cursor, tc.expected, result)
		}
	}
}

func TestCursorTracker_RenderCursorOverlay_MiddleOfLine(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, 2)

	result := ct.RenderCursorOverlay("hello", "█")
	expected := "he\x1b[7ml\x1b[27mlo"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_MiddleRow(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(1, 1)

	content := "line1\nline2\nline3"
	result := ct.RenderCursorOverlay(content, "█")
	expected := "line1\nl\x1b[7mi\x1b[27mne2\nline3"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
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
	ct.col = 5

	// Move cursor back 2 positions
	ct.UpdateFromOutput([]byte("\x1b[2D"))

	row, col := ct.GetPosition()
	if row != 0 || col != 3 {
		t.Errorf("Expected position (0,3), got (%d,%d)", row, col)
	}
}

// Additional tests for better coverage

func TestCursorTracker_SetPosition_NegativeValues(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(-5, -10)

	row, col := ct.GetPosition()
	if row != 0 || col != 0 {
		t.Errorf("Expected negative values clamped to (0,0), got (%d,%d)", row, col)
	}
}

func TestCursorTracker_RenderCursorOverlay_EmptyCursorChar(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, 5)

	result := ct.RenderCursorOverlay("hello", "")
	if result != "hello" {
		t.Errorf("Expected empty cursor to return unchanged content, got %q", result)
	}
}

func TestCursorTracker_RenderCursorOverlay_CursorBeyondContent(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, 10)

	result := ct.RenderCursorOverlay("hi", "█")
	expected := "hi        █"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_WithANSICodes(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, 3)

	result := ct.RenderCursorOverlay("\x1b[31mred\x1b[0mtext", "█")
	// Cursor at visible position 3, which is after "red" escape sequences
	expected := "\x1b[31mred\x1b[0m\x1b[7mt\x1b[27mext"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_MultiLineWithANSI(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(1, 2)

	content := "first\n\x1b[32msecond\x1b[0m\nthird"
	result := ct.RenderCursorOverlay(content, "█")
	expected := "first\n\x1b[32mse\x1b[7mc\x1b[27mond\x1b[0m\nthird"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_AtStartOfLine(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(0, 0)

	result := ct.RenderCursorOverlay("hello", "█")
	expected := "\x1b[7mh\x1b[27mello"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_RenderCursorOverlay_BeyondLastLine(t *testing.T) {
	ct := NewCursorTracker()
	ct.SetPosition(5, 0)

	result := ct.RenderCursorOverlay("line1\nline2", "█")
	expected := "line1\nline2\n\n\n\n█"
	if result != expected {
		t.Errorf("Expected %q, got %q", expected, result)
	}
}

func TestCursorTracker_UpdateFromOutput_CursorForwardWithParam(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 2

	ct.UpdateFromOutput([]byte("\x1b[5C"))

	_, col := ct.GetPosition()
	if col != 7 {
		t.Errorf("Expected col 7, got %d", col)
	}
}

func TestCursorTracker_UpdateFromOutput_BackspaceAtColumn0(t *testing.T) {
	ct := NewCursorTracker()
	ct.col = 0

	ct.UpdateFromOutput([]byte("\x08"))

	_, col := ct.GetPosition()
	if col != 0 {
		t.Errorf("Expected col to stay at 0, got %d", col)
	}
}

func TestCursorTracker_UpdateFromOutput_MultipleNewlines(t *testing.T) {
	ct := NewCursorTracker()

	ct.UpdateFromOutput([]byte("\n\n\n"))

	row, col := ct.GetPosition()
	// CursorTracker only tracks last newline, so row=1
	if row != 1 || col != 0 {
		t.Errorf("Expected position (1,0), got (%d,%d)", row, col)
	}
}

func TestCursorTracker_UpdateFromOutput_AbsolutePositionEdgeCases(t *testing.T) {
	ct := NewCursorTracker()

	// Move to position 1;1 (top-left)
	ct.UpdateFromOutput([]byte("\x1b[1;1H"))

	row, col := ct.GetPosition()
	if row != 0 || col != 0 {
		t.Errorf("Expected position (0,0), got (%d,%d)", row, col)
	}

	// Move to position 10;20
	ct.UpdateFromOutput([]byte("\x1b[10;20H"))

	row, col = ct.GetPosition()
	if row != 9 || col != 19 {
		t.Errorf("Expected position (9,19), got (%d,%d)", row, col)
	}
}
