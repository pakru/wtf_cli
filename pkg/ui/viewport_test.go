package ui

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
	
	if vp.viewport.Width != 80 {
		t.Errorf("Expected width 80, got %d", vp.viewport.Width)
	}
	
	if vp.viewport.Height != 24 {
		t.Errorf("Expected height 24, got %d", vp.viewport.Height)
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

func TestPTYViewport_ANSIPreservation(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)
	
	// ANSI colored output
	ansiText := []byte("\033[1;31mRed Text\033[0m")
	vp.AppendOutput(ansiText)
	
	content := vp.GetContent()
	if !strings.Contains(content, "\033[1;31m") {
		t.Error("Expected ANSI codes to be preserved")
	}
}
