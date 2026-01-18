package ui

import "testing"

func TestAppendPTYContent_CRLF(t *testing.T) {
	pending := false
	got := appendPTYContent("", []byte("line 1\r\nline 2\r\n"), &pending)
	want := "line 1\nline 2\n"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestAppendPTYContent_OverwritesLineOnCR(t *testing.T) {
	pending := false
	content := appendPTYContent("", []byte("prompt$ "), &pending)
	content = appendPTYContent(content, []byte("\roverwrite$ "), &pending)
	if content != "overwrite$ " {
		t.Fatalf("expected %q, got %q", "overwrite$ ", content)
	}
}

func TestAppendPTYContent_CRLFSplit(t *testing.T) {
	pending := false
	content := appendPTYContent("", []byte("line 1\r"), &pending)
	content = appendPTYContent(content, []byte("\nline 2\n"), &pending)
	want := "line 1\nline 2\n"
	if content != want {
		t.Fatalf("expected %q, got %q", want, content)
	}
}

// TestAppendPTYContent_CRWithANSI tests that CR followed by ANSI escape codes
// (like ESC[K for clear-to-end-of-line) does not clear the current line.
// This is the pattern sent when Ctrl+C is pressed in bash.
func TestAppendPTYContent_CRWithANSI(t *testing.T) {
	pending := false
	// Simulate: prompt with input, then \r followed by ESC[K (clear to EOL) + ^C + newline + new prompt
	content := appendPTYContent("", []byte("prompt$ ls -la"), &pending)
	// After Ctrl+C, bash sends: \r + ESC[K + ^C + \n + new prompt
	content = appendPTYContent(content, []byte("\r\x1b[K^C\n"), &pending)
	content = appendPTYContent(content, []byte("prompt$ "), &pending)

	// The ESC[K should be preserved (not stripped), and the original line
	// should NOT be cleared because \r was followed by ESC (0x1b)
	want := "prompt$ ls -la\x1b[K^C\nprompt$ "
	if content != want {
		t.Fatalf("expected %q, got %q", want, content)
	}
}
