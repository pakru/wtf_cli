package terminal

import "testing"

func TestNormalizer_CRLF(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("line 1\r\nline 2\r\n"))

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if string(lines[0]) != "line 1" {
		t.Fatalf("expected line 1, got %q", string(lines[0]))
	}
	if string(lines[1]) != "line 2" {
		t.Fatalf("expected line 2, got %q", string(lines[1]))
	}
}

func TestNormalizer_OverwriteOnCR(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("prompt$ \roverwrite$ \n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "overwrite$ " {
		t.Fatalf("expected overwrite line, got %q", string(lines[0]))
	}
}

func TestNormalizer_Backspace(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("ab\x08c\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "ac" {
		t.Fatalf("expected %q, got %q", "ac", string(lines[0]))
	}
}

func TestNormalizer_CursorLeftCSI(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("abcd\x1b[2Def\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "abef" {
		t.Fatalf("expected %q, got %q", "abef", string(lines[0]))
	}
}

func TestNormalizer_OSCStripped(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("\x1b]0;dev@host: ~/project\x07dev@host$ ls\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "dev@host$ ls" {
		t.Fatalf("expected %q, got %q", "dev@host$ ls", string(lines[0]))
	}
}

func TestNormalizer_TabExpansion(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("a\tb\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "a    b" {
		t.Fatalf("expected %q, got %q", "a    b", string(lines[0]))
	}
}
