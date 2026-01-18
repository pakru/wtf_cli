package ui

import "testing"

func TestDetectAltScreenSimple_EnterOnly(t *testing.T) {
	data := []byte("some text\x1b[?1049hmore text")
	entering, exiting := DetectAltScreenSimple(data)

	if !entering {
		t.Error("Expected entering to be true for smcup sequence")
	}
	if exiting {
		t.Error("Expected exiting to be false")
	}
}

func TestDetectAltScreenSimple_ExitOnly(t *testing.T) {
	data := []byte("some text\x1b[?1049lmore text")
	entering, exiting := DetectAltScreenSimple(data)

	if entering {
		t.Error("Expected entering to be false")
	}
	if !exiting {
		t.Error("Expected exiting to be true for rmcup sequence")
	}
}

func TestDetectAltScreenSimple_Enter1047(t *testing.T) {
	data := []byte("text\x1b[?1047hmore text")
	entering, exiting := DetectAltScreenSimple(data)

	if !entering {
		t.Error("Expected entering to be true for 1047h sequence")
	}
	if exiting {
		t.Error("Expected exiting to be false")
	}
}

func TestDetectAltScreenSimple_Exit1047(t *testing.T) {
	data := []byte("text\x1b[?1047lmore text")
	entering, exiting := DetectAltScreenSimple(data)

	if entering {
		t.Error("Expected entering to be false")
	}
	if !exiting {
		t.Error("Expected exiting to be true for 1047l sequence")
	}
}

func TestDetectAltScreenSimple_Both(t *testing.T) {
	// This can happen if app enters and exits quickly in same buffer
	data := []byte("\x1b[?1049h...content...\x1b[?1049l")
	entering, exiting := DetectAltScreenSimple(data)

	if !entering {
		t.Error("Expected entering to be true")
	}
	if !exiting {
		t.Error("Expected exiting to be true")
	}
}

func TestDetectAltScreenSimple_Neither(t *testing.T) {
	data := []byte("just regular text with no escape sequences")
	entering, exiting := DetectAltScreenSimple(data)

	if entering {
		t.Error("Expected entering to be false")
	}
	if exiting {
		t.Error("Expected exiting to be false")
	}
}

func TestDetectAltScreenSimple_OlderXterm(t *testing.T) {
	// Test older xterm variant
	data := []byte("\x1b[?47h")
	entering, exiting := DetectAltScreenSimple(data)

	if !entering {
		t.Error("Expected entering to be true for older xterm smcup")
	}
	if exiting {
		t.Error("Expected exiting to be false")
	}
}

func TestAltScreenState_SplitSequence(t *testing.T) {
	state := NewAltScreenState()

	// First chunk ends with partial escape sequence
	chunk1 := []byte("text\x1b[?104")
	entering1, exiting1 := state.DetectAltScreen(chunk1)

	if entering1 || exiting1 {
		t.Error("Should not detect on incomplete sequence")
	}

	// Second chunk completes the sequence
	chunk2 := []byte("9h more text")
	entering2, exiting2 := state.DetectAltScreen(chunk2)

	if !entering2 {
		t.Error("Expected entering to be true after completing split sequence")
	}
	if exiting2 {
		t.Error("Expected exiting to be false")
	}
}

func TestAltScreenState_SplitTransitions_EnterExitSameChunk(t *testing.T) {
	state := NewAltScreenState()

	data := []byte("before\x1b[?1049hinside\x1b[?1049lafter")
	chunks := state.SplitTransitions(data)

	if len(chunks) != 5 {
		t.Fatalf("Expected 5 chunks, got %d", len(chunks))
	}

	if string(chunks[0].data) != "before" {
		t.Errorf("Expected chunk 0 to be 'before', got %q", chunks[0].data)
	}
	if !chunks[1].entering || chunks[1].exiting {
		t.Errorf("Expected chunk 1 to be entering sequence")
	}
	if string(chunks[2].data) != "inside" {
		t.Errorf("Expected chunk 2 to be 'inside', got %q", chunks[2].data)
	}
	if !chunks[3].exiting || chunks[3].entering {
		t.Errorf("Expected chunk 3 to be exiting sequence")
	}
	if string(chunks[4].data) != "after" {
		t.Errorf("Expected chunk 4 to be 'after', got %q", chunks[4].data)
	}
}

func TestAltScreenState_SplitTransitions_PendingPrefix(t *testing.T) {
	state := NewAltScreenState()

	chunks := state.SplitTransitions([]byte("text\x1b[?104"))
	if len(chunks) != 1 {
		t.Fatalf("Expected 1 chunk, got %d", len(chunks))
	}
	if string(chunks[0].data) != "text" {
		t.Errorf("Expected chunk data 'text', got %q", chunks[0].data)
	}
	if string(state.pending) != "\x1b[?104" {
		t.Errorf("Expected pending to contain partial sequence, got %q", state.pending)
	}

	chunks = state.SplitTransitions([]byte("9hmore"))
	if len(chunks) < 2 {
		t.Fatalf("Expected at least 2 chunks, got %d", len(chunks))
	}
	if !chunks[0].entering {
		t.Error("Expected first chunk to be entering sequence")
	}
	if string(chunks[1].data) != "more" {
		t.Errorf("Expected trailing chunk 'more', got %q", chunks[1].data)
	}
}

func TestAltScreenState_Reset(t *testing.T) {
	state := NewAltScreenState()

	// Add some pending data
	state.DetectAltScreen([]byte("text\x1b[?104"))

	// Reset should clear pending
	state.Reset()

	if len(state.pending) != 0 {
		t.Error("Expected pending to be empty after Reset()")
	}
}

func TestIsCompleteEscapeSequence(t *testing.T) {
	tests := []struct {
		name     string
		data     []byte
		complete bool
	}{
		{"empty", []byte{}, true},
		{"no escape", []byte("hello"), true},
		{"just ESC", []byte{0x1b}, false},
		{"ESC [", []byte("\x1b["), false},
		{"ESC [ partial", []byte("\x1b[?1049"), false},
		{"ESC [ complete", []byte("\x1b[?1049h"), true},
		{"ESC single char", []byte("\x1b7"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isCompleteEscapeSequence(tt.data)
			if got != tt.complete {
				t.Errorf("isCompleteEscapeSequence(%q) = %v, want %v", tt.data, got, tt.complete)
			}
		})
	}
}
