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
