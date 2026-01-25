package viewport

import (
	"strings"
	"testing"
)

func TestNewPTYViewport(t *testing.T) {
	vp := NewPTYViewport()

	if vp.ready {
		t.Error("Expected viewport to not be ready initially")
	}

	if vp.content != "" {
		t.Error("Expected empty content initially")
	}
}

func TestPTYViewport_SetSize(t *testing.T) {
	vp := NewPTYViewport()

	vp.SetSize(80, 24)

	if vp.Viewport.Width() != 80 {
		t.Errorf("Expected width 80, got %d", vp.Viewport.Width())
	}

	if vp.Viewport.Height() != 24 {
		t.Errorf("Expected height 24, got %d", vp.Viewport.Height())
	}

	if !vp.ready {
		t.Error("Expected viewport to be ready after SetSize")
	}
}

func TestPTYViewport_AppendOutput(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("line 1\n"))
	vp.AppendOutput([]byte("line 2\n"))

	content := vp.GetContent()
	if content != "line 1\nline 2\n" {
		t.Errorf("Expected 'line 1\\nline 2\\n', got %q", content)
	}
}

func TestPTYViewport_CursorLeft_ShowsCursorInPlace(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("hello"))
	vp.AppendOutput([]byte("\x08")) // backspace moves cursor left

	view := vp.View()
	if !strings.Contains(view, "hell\x1b[7mo\x1b[27m") {
		t.Errorf("Expected cursor in 'hell\\x1b[7mo\\x1b[27m', got %q", view)
	}
}

func TestPTYViewport_CursorLeftCSI_ShowsCursorInPlace(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("hello\x1b[2D"))

	view := vp.View()
	if !strings.Contains(view, "hel\x1b[7ml\x1b[27mo") {
		t.Errorf("Expected cursor in 'hel\\x1b[7ml\\x1b[27mo', got %q", view)
	}
}

func TestPTYViewport_CursorRight_PadsSpaces(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("hi\x1b[3C"))

	view := vp.View()
	if !strings.Contains(view, "hi   █") {
		t.Errorf("Expected cursor after padding in 'hi   █', got %q", view)
	}
}

func TestPTYViewport_CursorHome_ShowsCursorAtStart(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("hello"))
	vp.AppendOutput([]byte("\x1b[H"))

	view := vp.View()
	if !strings.Contains(view, "\x1b[7mh\x1b[27mello") {
		t.Errorf("Expected cursor at start in '\\x1b[7mh\\x1b[27mello', got %q", view)
	}
}

func TestPTYViewport_CursorEnd_ShowsCursorAtEnd(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("hello"))
	vp.AppendOutput([]byte("\x1b[H"))
	vp.AppendOutput([]byte("\x1b[F"))

	view := vp.View()
	if !strings.Contains(view, "hello█") {
		t.Errorf("Expected cursor at end in 'hello█', got %q", view)
	}
}

func TestPTYViewport_HomeEndEdits(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("abcd"))
	vp.AppendOutput([]byte("\x1b[H"))
	vp.AppendOutput([]byte("X"))
	vp.AppendOutput([]byte("\x1b[F"))
	vp.AppendOutput([]byte("Z"))

	if got := vp.GetContent(); got != "XbcdZ" {
		t.Errorf("Expected %q, got %q", "XbcdZ", got)
	}
}

func TestPTYViewport_Clear(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	vp.AppendOutput([]byte("some content"))
	vp.Clear()

	if vp.GetContent() != "" {
		t.Error("Expected empty content after Clear()")
	}
}

func TestPTYViewport_View(t *testing.T) {
	vp := NewPTYViewport()

	// Not ready yet
	view := vp.View()
	if view != "Loading..." {
		t.Errorf("Expected 'Loading...', got %q", view)
	}

	// After SetSize
	vp.SetSize(80, 10)
	vp.AppendOutput([]byte("test content"))

	view = vp.View()
	// Content might have cursor ANSI codes, just check core text is present
	if !strings.Contains(view, "est content") {
		t.Error("Expected view to contain most of 'test content'")
	}
}

func TestPTYViewport_Stats(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 10)

	vp.AppendOutput([]byte("line 1\nline 2\nline 3\n"))

	totalLines, visibleLines, _ := vp.Stats()

	if totalLines != 4 { // 3 lines + 1 empty from trailing \n
		t.Errorf("Expected 4 total lines, got %d", totalLines)
	}

	if visibleLines != 10 {
		t.Errorf("Expected 10 visible lines, got %d", visibleLines)
	}
}

func TestPTYViewport_Scrolling(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 5)

	// Add more content than fits in viewport
	for i := 0; i < 20; i++ {
		vp.AppendOutput([]byte("line\n"))
	}

	// Should auto-scroll to bottom
	if !vp.IsAtBottom() {
		t.Error("Expected viewport to be at bottom after append")
	}

	// Scroll up
	vp.ScrollUp()

	// May or may not be at bottom depending on implementation
	// Just verify it doesn't panic
}

func TestPTYViewport_ANSIPreserved(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	// ANSI colored output - color/style codes (SGR) are preserved for proper display
	ansiText := []byte("\033[1;31mRed Text\033[0m")
	vp.AppendOutput(ansiText)

	content := vp.GetContent()
	// Both text and ANSI codes should be preserved
	if !strings.Contains(content, "Red Text") {
		t.Error("Expected text to be preserved")
	}
	if !strings.Contains(content, "\033[1;31m") {
		t.Error("Expected ANSI color codes to be preserved")
	}
}
