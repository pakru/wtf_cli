package terminal

import (
	"testing"

	"wtf_cli/pkg/capture"
)

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

func TestNormalizer_BackspaceDeleteSequence(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("ab\x08 \x08\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "a" {
		t.Fatalf("expected %q, got %q", "a", string(lines[0]))
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

func TestNormalizer_InlineEdit_PromptExtraction(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("user$ git staus"))
	lines = append(lines, n.Append([]byte("\x1b[2Dtus\n"))...)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}

	line := string(lines[0])
	if line != "user$ git status" {
		t.Fatalf("expected %q, got %q", "user$ git status", line)
	}

	cmd := capture.ExtractCommandFromPrompt(line)
	if cmd != "git status" {
		t.Fatalf("expected %q, got %q", "git status", cmd)
	}
}

func TestNormalizer_HomeEndEdits(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("abcd\x1b[HXY\x1b[FZ\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "XYcdZ" {
		t.Fatalf("expected %q, got %q", "XYcdZ", string(lines[0]))
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
	// "\t" in normalizer expands to 4 spaces (TabSpaces constant)
	if string(lines[0]) != "a    b" {
		t.Fatalf("expected %q, got %q", "a    b", string(lines[0]))
	}
}

// Additional tests for better coverage

func TestNormalizer_CursorRight(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("hello\x1b[2Dxx\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "hello" (col 5), back 2 (col 3), write "xx" -> "helxxo"
	// Wait, actually it's "helxx" because we overwrite at col 3-4
	if string(lines[0]) != "helxx" {
		t.Fatalf("expected %q, got %q", "helxx", string(lines[0]))
	}
}

func TestNormalizer_ClearToEOL_InMiddle(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("hello world\x1b[6D\x1b[K\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "hello world" (col 11), back 6 (col 5), clear to EOL
	if string(lines[0]) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(lines[0]))
	}
}

func TestNormalizer_ClearToEOL_AtEnd(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("test\x1b[K\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "test" {
		t.Fatalf("expected %q, got %q", "test", string(lines[0]))
	}
}

func TestNormalizer_ClearToEOL_BeyondLine(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("ab\x1b[10C\x1b[K\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "ab" (col 2), right 10 (col 12), clear (no-op since we're beyond), end
	if string(lines[0]) != "ab" {
		t.Fatalf("expected %q, got %q", "ab", string(lines[0]))
	}
}

func TestNormalizer_CursorMovement_BeyondLine(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("hi\x1b[10Cx\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "hi" (col 2), right 10 (col 12), write "x" with padding
	if string(lines[0]) != "hi          x" {
		t.Fatalf("expected %q, got %q", "hi          x", string(lines[0]))
	}
}

func TestNormalizer_OnlyCR_NoLF(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("hello\rworld"))

	// CR without LF doesn't flush the line
	if len(lines) != 0 {
		t.Fatalf("expected 0 lines without LF, got %d", len(lines))
	}

	// Flush with LF
	lines = n.Append([]byte("\n"))
	if len(lines) != 1 {
		t.Fatalf("expected 1 line after LF, got %d", len(lines))
	}
	// "hello" then CR (col 0), "world" overwrites
	if string(lines[0]) != "world" {
		t.Fatalf("expected %q, got %q", "world", string(lines[0]))
	}
}

func TestNormalizer_BackspaceAtStart(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("\x08\x08hello\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// Backspace at col 0 stays at 0
	if string(lines[0]) != "hello" {
		t.Fatalf("expected %q, got %q", "hello", string(lines[0]))
	}
}

func TestNormalizer_ComplexEdit_TypoCorrection(t *testing.T) {
	n := NewNormalizer()
	// Type "git staus", then fix: left 2, type "tus"
	lines := n.Append([]byte("git staus\x1b[2Dtus\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	if string(lines[0]) != "git status" {
		t.Fatalf("expected %q, got %q", "git status", string(lines[0]))
	}
}

func TestNormalizer_WriteAtStartOfLine(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("hello\x1b[5Dworld\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "hello" (col 5), back 5 (col 0), write "world"
	if string(lines[0]) != "world" {
		t.Fatalf("expected %q, got %q", "world", string(lines[0]))
	}
}

func TestNormalizer_CursorLeftWithParam(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("abcdef\x1b[3Dxy\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "abcdef" (col 6), back 3 (col 3), write "xy" -> "abcxy"
	if string(lines[0]) != "abcxyf" {
		t.Fatalf("expected %q, got %q", "abcxyf", string(lines[0]))
	}
}

func TestNormalizer_CursorRightWithParam(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("ab\x1b[3Cx\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "ab" (col 2), right 3 (col 5), write "x"
	if string(lines[0]) != "ab   x" {
		t.Fatalf("expected %q, got %q", "ab   x", string(lines[0]))
	}
}

func TestNormalizer_DeleteSequence_BackspaceSpaceBackspace(t *testing.T) {
	n := NewNormalizer()
	// This is actual terminal delete: backspace (move left), space (overwrite), backspace (move left again)
	lines := n.Append([]byte("test\x08 \x08more\n"))

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d", len(lines))
	}
	// "test" (col 4), BS (col 3), space (col 4), BS (col 3), "more"
	// Result: "tesmore"
	// Actually with the deleteAtCursor logic, it should be "tesmore"
	if string(lines[0]) != "tesmore" {
		t.Fatalf("expected %q, got %q", "tesmore", string(lines[0]))
	}
}

func TestNormalizer_PendingCR_WithText(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte("first\r\nsecond\n"))

	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if string(lines[0]) != "first" || string(lines[1]) != "second" {
		t.Fatalf("expected %q and %q, got %q and %q", "first", "second", string(lines[0]), string(lines[1]))
	}
}

func TestNormalizer_EmptyAppend(t *testing.T) {
	n := NewNormalizer()
	lines := n.Append([]byte{})

	if len(lines) != 0 {
		t.Fatalf("expected 0 lines for empty input, got %d", len(lines))
	}
}
