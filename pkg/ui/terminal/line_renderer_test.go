package terminal

import "testing"

func TestLineRenderer_CRLF(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("line 1\r\nline 2\r\n"))

	if got := r.Content(); got != "line 1\nline 2\n" {
		t.Fatalf("expected %q, got %q", "line 1\nline 2\n", got)
	}
}

func TestLineRenderer_CRWithClearToEOL(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("prompt$ ls"))
	r.Append([]byte("\r\x1b[Kprompt$ "))

	if got := r.Content(); got != "prompt$ " {
		t.Fatalf("expected %q, got %q", "prompt$ ", got)
	}
}

func TestLineRenderer_BackspaceMovesCursor(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc\x08"))

	if got := r.Content(); got != "abc" {
		t.Fatalf("expected %q, got %q", "abc", got)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 2 {
		t.Fatalf("expected cursor at (0,2), got (%d,%d)", row, col)
	}
}

func TestLineRenderer_InlineOverwriteWithCSI(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcd\x1b[2DXY"))

	if got := r.Content(); got != "abXY" {
		t.Fatalf("expected %q, got %q", "abXY", got)
	}
}

func TestLineRenderer_InlineOverwrite_Backspace(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc\x08\x08X"))

	if got := r.Content(); got != "aXc" {
		t.Fatalf("expected %q, got %q", "aXc", got)
	}
}

func TestLineRenderer_BackspaceDeleteSequence(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc\x08 \x08"))

	if got := r.Content(); got != "ab " {
		t.Fatalf("expected %q, got %q", "ab ", got)
	}
}

func TestLineRenderer_CursorRight_MoveOnly(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hi\x1b[3C"))

	if got := r.Content(); got != "hi" {
		t.Fatalf("expected %q, got %q", "hi", got)
	}
	_, col := r.CursorPosition()
	if col != 5 {
		t.Fatalf("expected cursor col 5, got %d", col)
	}
}

func TestLineRenderer_ANSISequencePreserved(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mRed\x1b[0m"))

	content := r.Content()
	if content != "\x1b[31mRed\x1b[0m" {
		t.Fatalf("expected ANSI preserved, got %q", content)
	}
}

func TestLineRenderer_HomeEndEdits(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcd\x1b[HXY\x1b[FZ"))

	if got := r.Content(); got != "XYcdZ" {
		t.Fatalf("expected %q, got %q", "XYcdZ", got)
	}
}

// New tests for better coverage

func TestLineRenderer_WideCharacters(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hello世界"))

	content := r.Content()
	if content != "hello世界" {
		t.Fatalf("expected %q, got %q", "hello世界", content)
	}
	_, col := r.CursorPosition()
	// "hello" = 5 cols, "世" = 2 cols, "界" = 2 cols = 9 total
	if col != 9 {
		t.Fatalf("expected cursor at col 9, got %d", col)
	}
}

func TestLineRenderer_WideCharacterOverwrite(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc世\x1b[2DX"))

	content := r.Content()
	// "abc世" (col 7: a=1,b=1,c=1,世=2), back 2 (col 5, middle of 世), then 'X' at col 5
	if content != "abcX" {
		t.Fatalf("expected %q, got %q", "abcX", content)
	}
}

func TestLineRenderer_TabExpansion(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("ab\tc"))

	content := r.Content()
	// "ab" (2 chars) + tab (2 spaces to reach col 4) + "c" = "ab  c"
	if content != "ab  c" {
		t.Fatalf("expected %q, got %q", "ab  c", content)
	}
	_, col := r.CursorPosition()
	if col != 5 {
		t.Fatalf("expected cursor at col 5, got %d", col)
	}
}

func TestLineRenderer_TabAtBoundary(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcd\tx"))

	content := r.Content()
	// "abcd" (4 chars) + tab (4 spaces to reach col 8) + "x" = "abcd    x"
	if content != "abcd    x" {
		t.Fatalf("expected %q, got %q", "abcd    x", content)
	}
}

func TestLineRenderer_OSCSequenceStripped(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b]0;Window Title\x07test"))

	content := r.Content()
	if content != "test" {
		t.Fatalf("expected %q, got %q", "test", content)
	}
}

func TestLineRenderer_OSCWithEscapeTerminator(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b]0;Title\x1b\\hello"))

	content := r.Content()
	if content != "hello" {
		t.Fatalf("expected %q, got %q", "hello", content)
	}
}

func TestLineRenderer_Reset(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("line 1\nline 2"))

	r.Reset()

	content := r.Content()
	if content != "" {
		t.Fatalf("expected empty content after reset, got %q", content)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 0 {
		t.Fatalf("expected cursor at (0,0) after reset, got (%d,%d)", row, col)
	}
}

func TestLineRenderer_BackspaceAtStartOfLine(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x08\x08abc"))

	content := r.Content()
	if content != "abc" {
		t.Fatalf("expected %q, got %q", "abc", content)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 3 {
		t.Fatalf("expected cursor at (0,3), got (%d,%d)", row, col)
	}
}

func TestLineRenderer_DELCharacter(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc\x7f"))

	content := r.Content()
	if content != "abc" {
		t.Fatalf("expected %q, got %q", "abc", content)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 2 {
		t.Fatalf("expected cursor at (0,2), got (%d,%d)", row, col)
	}
}

func TestLineRenderer_CursorLeftWithParameter(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcdef\x1b[4Dxy"))

	content := r.Content()
	if content != "abxyef" {
		t.Fatalf("expected %q, got %q", "abxyef", content)
	}
}

func TestLineRenderer_CursorLeftAtStartClamps(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("ab\x1b[10Dxy"))

	content := r.Content()
	// Cursor at col 2, move left 10 (clamps to 0), then write "xy"
	if content != "xy" {
		t.Fatalf("expected %q, got %q", "xy", content)
	}
}

func TestLineRenderer_CursorRightWithParameter(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("ab\x1b[5Cx"))

	content := r.Content()
	// "ab" (col 2), move right 5 (to col 7), write "x" at col 7
	if content != "ab     x" {
		t.Fatalf("expected %q, got %q", "ab     x", content)
	}
}

func TestLineRenderer_AbsolutePositioning_CUP(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("line1\nline2\nline3\x1b[2;3Hx"))

	content := r.Content()
	// CSI 2;3 H = row 2 (1-indexed, so line index 1), col 3 (1-indexed, so col index 2)
	// "line2" -> "lixe2" (no space padding, just overwrite 'n' with 'x')
	expected := "line1\nlixe2\nline3"
	if content != expected {
		t.Fatalf("expected %q, got %q", expected, content)
	}
}

func TestLineRenderer_AbsolutePositioning_DefaultParams(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("test\x1b[HX"))

	content := r.Content()
	// CSI H with no params = CSI 1;1 H = move to (0,0)
	if content != "Xest" {
		t.Fatalf("expected %q, got %q", "Xest", content)
	}
}

func TestLineRenderer_CursorForward_NoParameter(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("ab\x1b[Cx"))

	content := r.Content()
	// "ab" (col 2), move right 1 (default), write "x" at col 3
	if content != "ab x" {
		t.Fatalf("expected %q, got %q", "ab x", content)
	}
}

func TestLineRenderer_CursorBack_NoParameter(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc\x1b[Dx"))

	content := r.Content()
	// "abc" (col 3), move left 1 (default), write "x" at col 2
	if content != "abx" {
		t.Fatalf("expected %q, got %q", "abx", content)
	}
}

func TestLineRenderer_MultipleLines(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("first\nsecond\nthird"))

	content := r.Content()
	if content != "first\nsecond\nthird" {
		t.Fatalf("expected %q, got %q", "first\nsecond\nthird", content)
	}
	row, col := r.CursorPosition()
	if row != 2 || col != 5 {
		t.Fatalf("expected cursor at (2,5), got (%d,%d)", row, col)
	}
}

func TestLineRenderer_EmptyLineInMiddle(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("first\n\nthird"))

	content := r.Content()
	if content != "first\n\nthird" {
		t.Fatalf("expected %q, got %q", "first\n\nthird", content)
	}
}

func TestLineRenderer_OnlyCR(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hello\rworld"))

	content := r.Content()
	// CR returns to column 0 on same line, "world" overwrites "hello"
	if content != "world" {
		t.Fatalf("expected %q, got %q", "world", content)
	}
}

func TestLineRenderer_ControlCharsIgnored(t *testing.T) {
	r := NewLineRenderer()
	// Include some control chars that should be ignored (< 0x20 and not special)
	r.Append([]byte("ab\x02\x03cd"))

	content := r.Content()
	if content != "abcd" {
		t.Fatalf("expected %q, got %q", "abcd", content)
	}
}

func TestLineRenderer_ClearToEOLAtEnd(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("test\x1b[K"))

	content := r.Content()
	if content != "test" {
		t.Fatalf("expected %q, got %q", "test", content)
	}
}

func TestLineRenderer_ClearToEOLInMiddle(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hello world\x1b[6D\x1b[K"))

	content := r.Content()
	// "hello world" (col 11), back 6 (col 5), clear to EOL
	if content != "hello" {
		t.Fatalf("expected %q, got %q", "hello", content)
	}
}

func TestLineRenderer_DeleteCharacterCSI(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcdef\x1b[3D\x1b[P"))

	if got := r.Content(); got != "abcef" {
		t.Fatalf("expected %q, got %q", "abcef", got)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 3 {
		t.Fatalf("expected cursor at (0,3), got (%d,%d)", row, col)
	}
}
