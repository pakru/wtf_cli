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

// TestAppendPTYContent_CRWithANSI tests CR + ANSI escape sequence behavior.
// When CR is followed by ANSI escape sequence (like color codes), the line is NOT cleared.
// This preserves colored prompts when bash redraws.
func TestAppendPTYContent_CRWithANSI(t *testing.T) {
	pending := false
	// Simulate: prompt with input, then \r followed by ESC[K (clear to EOL) + ^C + newline + new prompt
	content := appendPTYContent("", []byte("prompt$ ls -la"), &pending)
	// After Ctrl+C, bash sends: \r + ESC[K + ^C + \n + new prompt
	content = appendPTYContent(content, []byte("\r\x1b[K^C\n"), &pending)
	content = appendPTYContent(content, []byte("prompt$ "), &pending)

	// CR followed by ESC doesn't clear the line (preserves colors/content)
	// ESC[K is filtered, ^C and newline added, then new prompt
	want := "prompt$ ls -la^C\nprompt$ "
	if content != want {
		t.Fatalf("expected %q, got %q", want, content)
	}
}

func TestAppendPTYContent_Backspace(t *testing.T) {
	pending := false

	// Test basic backspace: type "ab", backspace, type "c" -> "ac"
	content := appendPTYContent("", []byte("ab\x08c"), &pending)
	if content != "ac" {
		t.Fatalf("expected %q, got %q", "ac", content)
	}

	// Test multiple backspaces
	pending = false
	content = appendPTYContent("", []byte("abc\x08\x08d"), &pending)
	if content != "ad" {
		t.Fatalf("expected %q, got %q", "ad", content)
	}

	// Test backspace at start of line (should not go past line start)
	pending = false
	content = appendPTYContent("", []byte("\x08\x08abc"), &pending)
	if content != "abc" {
		t.Fatalf("expected %q, got %q", "abc", content)
	}

	// Test backspace doesn't cross newline
	pending = false
	content = appendPTYContent("", []byte("line1\n\x08abc"), &pending)
	if content != "line1\nabc" {
		t.Fatalf("expected %q, got %q", "line1\nabc", content)
	}
}
