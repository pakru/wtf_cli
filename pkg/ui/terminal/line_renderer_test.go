package terminal

import (
	"os"
	"testing"

	"wtf_cli/pkg/ui/components/welcome"

	"github.com/charmbracelet/x/ansi"
)

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

func TestLineRenderer_InsertCharacterCSI(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcdef\x1b[3D\x1b[@X"))

	if got := r.Content(); got != "abcXdef" {
		t.Fatalf("expected %q, got %q", "abcXdef", got)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 4 {
		t.Fatalf("expected cursor at (0,4), got (%d,%d)", row, col)
	}
}

func TestLineRenderer_InsertMode(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcdef\x1b[3D\x1b[4hX\x1b[4l"))

	if got := r.Content(); got != "abcXdef" {
		t.Fatalf("expected %q, got %q", "abcXdef", got)
	}
}

// CSI A — Cursor Up

func TestLineRenderer_CursorUp(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("line 0\nline 1\nline 2"))
	// Cursor at row 2. Move up 2 → row 0.
	r.Append([]byte("\x1b[2A\rNEW 0"))

	expected := "NEW 00\nline 1\nline 2"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_CursorUp_ClampToZero(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("only\nrow"))
	// Cursor at row 1. Move up 10 → clamps to 0.
	r.Append([]byte("\x1b[10A\rX"))

	expected := "Xnly\nrow"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_CursorUp_DefaultParam(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("r0\nr1\nr2"))
	// CSI A without param → up 1
	r.Append([]byte("\x1b[A\rX"))

	expected := "r0\nX1\nr2"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// CSI B — Cursor Down

func TestLineRenderer_CursorDown(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("row 0\nrow 1\nrow 2"))
	r.Append([]byte("\x1b[1;1H")) // Go to top-left
	r.Append([]byte("\x1b[2B"))   // Down 2
	r.Append([]byte("\rNEW"))

	expected := "row 0\nrow 1\nNEW 2"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_CursorDown_Extends(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("only"))
	// Down 3 from row 0 → ensures rows exist
	r.Append([]byte("\x1b[3Bhello"))

	row, _ := r.CursorPosition()
	if row != 3 {
		t.Fatalf("expected row 3, got %d", row)
	}
}

// CSI G — Cursor Horizontal Absolute

func TestLineRenderer_CursorHorizontalAbsolute(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abcdef"))
	r.Append([]byte("\x1b[1Gxyz"))

	expected := "xyzdef"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_CursorHorizontalAbsolute_DefaultParam(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hello"))
	r.Append([]byte("\x1b[GX"))

	expected := "Xello"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_CursorHorizontalAbsolute_Spinner(t *testing.T) {
	// Simulates npm spinner with CSI G
	r := NewLineRenderer()
	r.Append([]byte("\x1b[?25l")) // hide cursor
	r.Append([]byte("⠙"))
	r.Append([]byte("\x1b[1G⠹"))
	r.Append([]byte("\x1b[1G⠸"))
	r.Append([]byte("\x1b[1G⠼"))

	if got := r.Content(); got != "⠼" {
		t.Fatalf("expected single spinner char %q, got %q", "⠼", got)
	}
}

// CSI J — Erase in Display

func TestLineRenderer_EraseDisplay_CursorToEnd(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("line 0\nline 1\nline 2"))
	r.Append([]byte("\x1b[1;1H")) // Go to top-left
	r.Append([]byte("\x1b[J"))    // Erase from cursor (param 0)

	// Row 0 truncated from col 0 = empty, rows 1-2 deleted
	expected := ""
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_EraseDisplay_CursorToEnd_MidLine(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hello\nworld\nfoo"))
	r.Append([]byte("\x1b[2;3H")) // Row 2, col 3 (0-indexed: row 1, col 2)
	r.Append([]byte("\x1b[J"))    // Erase from cursor

	// Row 1 truncated from col 2 = "wo", rows after deleted
	expected := "hello\nwo"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_EraseDisplay_All(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("line 0\nline 1\nline 2"))
	r.Append([]byte("\x1b[2J")) // Erase all

	if got := r.Content(); got != "" {
		t.Fatalf("expected empty after CSI 2J, got %q", got)
	}
	row, col := r.CursorPosition()
	if row != 0 || col != 0 {
		t.Fatalf("expected cursor at (0,0), got (%d,%d)", row, col)
	}
}

// CSI K param 2 — Erase Entire Line

func TestLineRenderer_ClearEntireLine(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("hello world"))
	r.Append([]byte("\x1b[2K"))

	if got := r.Content(); got != "" {
		t.Fatalf("expected empty line after CSI 2K, got %q", got)
	}
}

func TestLineRenderer_ClearEntireLine_MultiRow(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("row 0\nrow 1 content\nrow 2"))
	r.Append([]byte("\x1b[2;1H")) // Go to row 2 (1-indexed), col 1
	r.Append([]byte("\x1b[2K"))   // Clear entire row 1

	expected := "row 0\n\nrow 2"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// CSI H no-params fix

func TestLineRenderer_CursorHome_NoParams(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("old content\nmore old"))
	r.Append([]byte("\x1b[H"))

	row, col := r.CursorPosition()
	if row != 0 || col != 0 {
		t.Fatalf("expected (0,0) after CSI H, got (%d,%d)", row, col)
	}
}

func TestLineRenderer_CursorHome_NoParams_ThenWrite(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("old content\nmore old"))
	r.Append([]byte("\x1b[HNEW"))

	expected := "NEW content\nmore old"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// Integration: Ink render cycle

func TestLineRenderer_InkRenderCycle(t *testing.T) {
	r := NewLineRenderer()

	// Frame 1
	r.Append([]byte("> Type your message\n"))
	r.Append([]byte("~ no sandbox\n"))
	r.Append([]byte("shift+tab to accept"))

	frame1 := "> Type your message\n~ no sandbox\nshift+tab to accept"
	if got := r.Content(); got != frame1 {
		t.Fatalf("frame 1: expected %q, got %q", frame1, got)
	}

	// Frame 2: cursor up 3, CR, erase to end, redraw
	r.Append([]byte("\x1b[3A"))  // up 3
	r.Append([]byte("\r\x1b[J")) // CR + erase to end of display
	r.Append([]byte("> d\n"))
	r.Append([]byte("~ no sandbox\n"))
	r.Append([]byte("shift+tab to accept"))

	frame2 := "> d\n~ no sandbox\nshift+tab to accept"
	if got := r.Content(); got != frame2 {
		t.Fatalf("frame 2: expected %q, got %q", frame2, got)
	}
}

// --- Issue #73: color bleed & progress-bar corruption --------------------

// TestLineRenderer_ColorBleed_EraseNoLongerDropsReset is the original
// issue #73 repro: erasing to end-of-line used to delete the stored SGR
// reset while an opener earlier in the line survived, bleeding color into
// every following line. SGR is state now, not a positional cell, so the
// erase can no longer touch it.
func TestLineRenderer_ColorBleed_EraseNoLongerDropsReset(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[33mYELLOW STATUS\x1b[0m"))
	r.Append([]byte("\rok\x1b[K\n"))
	r.Append([]byte("plain line\n"))

	expected := "ok\nplain line\n"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// TestLineRenderer_ColorBleed_OverwriteNoLongerKeepsStaleStyle is the
// second issue #73 repro: overwriting after CR used to leave the old
// zero-width SGR cells in place around the new plain text.
func TestLineRenderer_ColorBleed_OverwriteNoLongerKeepsStaleStyle(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[32m[=== ] 50%\x1b[0m"))
	r.Append([]byte("\rDONE......"))

	expected := "DONE......"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_MidLineColorChangeBalanced(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("A\x1b[31mB\x1b[0mC"))

	expected := "A\x1b[31mB\x1b[0mC"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// TestLineRenderer_SGR_PenPersistsAcrossNewline verifies the pen is
// terminal-like state, not per-line content: a color opened before a
// newline continues to apply on the next line, and each rendered line is
// independently balanced.
func TestLineRenderer_SGR_PenPersistsAcrossNewline(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mred\nstill red\x1b[0m\n"))

	expected := "\x1b[31mred\x1b[0m\n\x1b[31mstill red\x1b[0m\n"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_PartialReset22ClearsBoldAndDim(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[1;2mBD\x1b[22mplain"))

	// SGR 22 must clear BOTH bold(1) and dim(2), returning to the fully
	// default style (which interns to nil) — not just one of the two. If
	// either attribute survived, "plain" would render under a non-default
	// (non-empty) style instead of plain text after a single reset.
	expected := "\x1b[1;2mBD\x1b[0mplain"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// TestLineRenderer_SGR_PartialResets_EachAttributeFullyClears covers the
// remaining partial resets (22/bold+dim is covered above): setting a single
// attribute then clearing it with its dedicated code must return fully to
// default (interning to nil), not merely toggle a bit that leaves some
// other latent state behind.
func TestLineRenderer_SGR_PartialResets_EachAttributeFullyClears(t *testing.T) {
	cases := []struct {
		name      string
		setCode   string
		clearCode string
	}{
		{"italic", "3", "23"},
		{"underline", "4", "24"},
		{"blink", "5", "25"},
		{"reverse", "7", "27"},
		{"conceal", "8", "28"},
		{"strike", "9", "29"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := NewLineRenderer()
			r.Append([]byte("\x1b[" + c.setCode + "mX\x1b[" + c.clearCode + "mplain"))
			expected := "\x1b[" + c.setCode + "mX\x1b[0mplain"
			if got := r.Content(); got != expected {
				t.Fatalf("attribute %s: expected %q, got %q", c.name, expected, got)
			}
		})
	}
}

func TestLineRenderer_SGR_21IsIgnoredNotTreatedAsReset(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[1;31mA\x1b[21mB\x1b[0m"))

	// SGR 21 is double-underline, not bold-off. A styled run must
	// continue uninterrupted across it (no transition at 'B').
	expected := "\x1b[1;31mAB\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_256Color(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[38;5;208mX\x1b[0m"))

	expected := "\x1b[38;5;208mX\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_TruecolorBackground(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[48;2;10;20;30mX\x1b[0m"))

	expected := "\x1b[48;2;10;20;30mX\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_MalformedExtendedColor_NoIndex(t *testing.T) {
	r := NewLineRenderer()
	// "38;5" with no color index must not crash and must not corrupt fg.
	r.Append([]byte("\x1b[38;5mX"))

	if got := r.Content(); got != "X" {
		t.Fatalf("expected unstyled %q, got %q", "X", got)
	}
}

func TestLineRenderer_SGR_ExtendedColor_ConsumesExactlyItsParams(t *testing.T) {
	r := NewLineRenderer()
	// "38;5;1" is well-formed (fg = 256-color index 1); this asserts the
	// happy path consumes exactly its 3 parameters and nothing bleeds into
	// a following attribute.
	r.Append([]byte("\x1b[38;5;1mX\x1b[0m"))

	expected := "\x1b[38;5;1mX\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_MalformedExtendedColor_ConsumesRestOfSequence(t *testing.T) {
	r := NewLineRenderer()
	// "38;5" with no index: the malformed parse must consume the rest of
	// the sequence and leave the pen exactly as it was (fg unchanged from
	// the prior well-formed 38;5;1), never partially applying a stray
	// attribute from the leftover parameter.
	r.Append([]byte("\x1b[38;5;1mX\x1b[38;5mY"))

	expected := "\x1b[38;5;1mXY\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_SGR_EmptyCSIMeansReset(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mA\x1b[mB"))

	expected := "\x1b[31mA\x1b[0mB"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// TestLineRenderer_StyleImmutability_RedBlueRed is a direct regression test
// for interned style mutation: if applySGR ever mutated a *cellStyle in
// place instead of interning a fresh value, the first "red1" run would be
// retroactively recolored once the pen changes later in the sequence.
func TestLineRenderer_StyleImmutability_RedBlueRed(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mred1\x1b[34mblue\x1b[31mred2\x1b[0m"))

	expected := "\x1b[31mred1\x1b[0m\x1b[34mblue\x1b[0m\x1b[31mred2\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_StyleInterning_ReusesPointerForIdenticalValue(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mA"))
	first := r.pen
	r.Append([]byte("\x1b[34mB\x1b[31mC"))
	second := r.pen
	if first == nil || second == nil || first != second {
		t.Fatalf("expected identical SGR state to reuse the same interned pointer, got %p vs %p", first, second)
	}
}

// --- Issue #73: DECSC/DECRC and CSI s/u -----------------------------------

func TestLineRenderer_DECSC_DECRC_RestoresPositionAndPen(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("label \x1b[32m"))
	r.Append([]byte("\x1b7")) // save: after "label ", pen=green
	// Change the pen to red BEFORE restoring, so the test actually
	// discriminates: if DECRC failed to restore the pen (only position),
	// "frame two" would come out red instead of green.
	r.Append([]byte("\x1b[31m[frame one]\n"))
	r.Append([]byte("\x1b8"))       // restore: back to col 6, pen=green (not red)
	r.Append([]byte("[frame two]")) // overwrites frame one in place

	// "label " was written before any pen change, so it stays unstyled;
	// the restored frame is green, proving DECRC restored the saved pen
	// rather than leaving the red pen active from before the restore.
	expected := "label \x1b[32m[frame two]\x1b[0m\n"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_DECRC_WithoutSaveIsNoOp(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("abc"))
	r.Append([]byte("\x1b8")) // no prior ESC 7: documented no-op
	r.Append([]byte("XYZ"))

	// Cursor stays where it was (end of "abc"); "XYZ" appends, not overwrites.
	expected := "abcXYZ"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_CSI_su_SaveRestore(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("head-"))
	r.Append([]byte("\x1b[s"))
	r.Append([]byte("AAAA\n"))
	r.Append([]byte("\x1b[u"))
	r.Append([]byte("BBBB"))

	// The saved position is row 0 (before the newline), so BBBB overwrites
	// AAAA on row 0. The newline already materialized an empty row 1, and
	// restoring the cursor does not erase the display, so it persists.
	expected := "head-BBBB\n"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_DECSC_RepeatedSaveOverwritesSlot(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("first-"))
	r.Append([]byte("\x1b7")) // save at col 6
	r.Append([]byte("XXXXXX"))
	r.Append([]byte("\x1b7")) // save again at col 12 (end of XXXXXX)
	r.Append([]byte("\x1b8")) // restore: must go to col 12, not col 6
	r.Append([]byte("Y"))

	expected := "first-XXXXXXY"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_DECSC_ESC7SplitFromESC8ByChunking(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("head-"))
	r.Append([]byte{0x1b})
	r.Append([]byte{'7'})
	r.Append([]byte("AAAA"))
	r.Append([]byte{0x1b})
	r.Append([]byte{'8'})
	r.Append([]byte("BBBB"))

	expected := "head-BBBB"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// --- Issue #73: empty CSI parameters ---------------------------------------

func TestLineRenderer_EmptyCSIParam_LeadingSemicolon(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("first\nsecond\nthird"))
	r.Append([]byte("\x1b[;5H")) // CSI ;5H == row default(1) -> parsed [0,5] -> row=0,col=4
	r.Append([]byte("X"))

	expected := "firsX\nsecond\nthird"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_EmptyCSIParam_TrailingSemicolon(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("first\nsecond\nthird"))
	r.Append([]byte("\x1b[2;H")) // CSI 2;H -> parsed [2,0] -> row=1,col=0
	r.Append([]byte("X"))

	expected := "first\nXecond\nthird"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_EmptyCSIParam_AllEmpty(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("first\nsecond\nthird"))
	r.Append([]byte("\x1b[;;H")) // CSI ;;H -> parsed [0,0,0] -> row=0,col=0 (extra param ignored)
	r.Append([]byte("X"))

	expected := "Xirst\nsecond\nthird"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// --- Issue #73: styled edit/erase ops --------------------------------------

func TestLineRenderer_Styled_ErasureDoesNotResetPen(t *testing.T) {
	r := NewLineRenderer()
	// "red and more" all under the red pen; erase actually removes "and
	// more" (proving CSI K did something), and the pen must still be red
	// for "NEW" afterward, since erasure does not touch the pen.
	r.Append([]byte("\x1b[31mred and more"))
	r.Append([]byte("\x1b[8D\x1b[K")) // back to right after "red ", erase the rest
	r.Append([]byte("NEW"))

	expected := "\x1b[31mred NEW\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_Styled_ClearToEOLDropsOnlyErasedCells(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mred\x1b[0mplain"))
	r.Append([]byte("\r\x1b[3C\x1b[K")) // to col 3 (end of "red"), erase rest

	expected := "\x1b[31mred\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_Styled_DeleteCharacterCSI(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[32mabcdef\x1b[0m"))
	r.Append([]byte("\x1b[3D\x1b[P")) // back to col 3, delete 1 char ('d')

	expected := "\x1b[32mabcef\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_Styled_InsertCharacterCSI(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[35mabcdef\x1b[0m"))
	r.Append([]byte("\x1b[3D\x1b[@")) // back to col 3, insert 1 blank

	expected := "\x1b[35mabc\x1b[0m \x1b[35mdef\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_Styled_EraseCharacterCSI(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[36mabcdef\x1b[0m"))
	r.Append([]byte("\x1b[6D\x1b[3X")) // back to col 0, erase 3 chars

	expected := "   \x1b[36mdef\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestLineRenderer_Styled_EraseDisplayAll(t *testing.T) {
	r := NewLineRenderer()
	// No trailing reset before CSI 2J: the pen is still yellow when the
	// display is erased. ED clears all lines but, like other erase ops,
	// must not touch the pen — "after" must render yellow, not default.
	r.Append([]byte("\x1b[33mline0\nline1"))
	r.Append([]byte("\x1b[2J"))
	r.Append([]byte("after"))

	expected := "\x1b[33mafter\x1b[0m"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// --- Issue #73: Reset() clears the new state -------------------------------

func TestLineRenderer_Reset_ClearsStyleAndSavedCursor(t *testing.T) {
	r := NewLineRenderer()
	r.Append([]byte("\x1b[31mred\x1b7"))
	r.Reset()

	r.Append([]byte("plain"))
	if got := r.Content(); got != "plain" {
		t.Fatalf("expected pen cleared by Reset, got %q", got)
	}

	r.Append([]byte("\x1b8MOVED"))
	expected := "plainMOVED"
	if got := r.Content(); got != expected {
		t.Fatalf("expected saved cursor cleared by Reset (ESC 8 no-op), got %q", got)
	}
}

// --- Issue #73: chunk-split robustness (ASCII/escape sequences only) ------

func TestLineRenderer_ChunkSplitRobustness(t *testing.T) {
	inputs := [][]byte{
		[]byte("\x1b[33mYELLOW STATUS\x1b[0m\rok\x1b[K\nplain line\n"),
		[]byte("label-\x1b7AAAA\x1b8BBBB\n"),
		[]byte("\x1b[1;31;4mstyled\x1b[0m\x1b[38;5;208mextended\x1b[0m"),
		[]byte("\x1b[;5H\x1b[5;H\x1b[;;H"),
		[]byte("\x1b[s move \x1b[u done"),
	}

	for _, in := range inputs {
		whole := NewLineRenderer()
		whole.Append(in)
		wantContent := whole.Content()
		wantRow, wantCol := whole.CursorPosition()

		split := NewLineRenderer()
		for _, b := range in {
			split.Append([]byte{b})
		}
		gotContent := split.Content()
		gotRow, gotCol := split.CursorPosition()

		if gotContent != wantContent {
			t.Errorf("chunk-split content mismatch for %q:\n whole: %q\n split: %q", in, wantContent, gotContent)
		}
		if gotRow != wantRow || gotCol != wantCol {
			t.Errorf("chunk-split cursor mismatch for %q: whole=(%d,%d) split=(%d,%d)", in, wantRow, wantCol, gotRow, gotCol)
		}
	}
}

// --- Issue #73: pkcon fixture (ESC 7 / ESC 8 progress-bar redraw) ---------

func TestLineRenderer_PkconFixture_NoFrameConcatenation(t *testing.T) {
	data, err := os.ReadFile("testdata/pkcon_capture.raw")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	r := NewLineRenderer()
	r.Append(data)

	// Without DECSC/DECRC support, every ESC 8 frame would append after
	// the previous one instead of overwriting it. The exact expected
	// content pins the fix: only the final (100%) frame survives (the
	// fixture's trailing newline materializes an empty second line).
	expected := "Downloading updates [====================] (100%)\n"
	if got := r.Content(); got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

// --- Issue #73: welcome banner semantics survive rendering -----------------

func TestLineRenderer_WelcomeBannerSemanticsPreserved(t *testing.T) {
	r := NewLineRenderer()
	msg := welcome.WelcomeMessage()
	r.Append([]byte(msg))

	got := ansi.Strip(r.Content())
	want := ansi.Strip(msg)
	if got != want {
		t.Fatalf("welcome banner text changed after rendering:\ngot:  %q\nwant: %q", got, want)
	}
}

// --- Issue #73: memory/rendering overhead of the style representation ----
//
// Documents the real cost the plan's Risks section flagged: an 8-byte
// style pointer on every cell, plus one interned *cellStyle allocation per
// distinct pen. Compare with: go test ./pkg/ui/terminal/ -bench Scrollback -run xxx -benchmem

func BenchmarkLineRenderer_PlainScrollback(b *testing.B) {
	line := "the quick brown fox jumps over the lazy dog 0123456789\n"
	for i := 0; i < b.N; i++ {
		r := NewLineRenderer()
		for j := 0; j < 2000; j++ {
			r.Append([]byte(line))
		}
		_ = r.Content()
	}
}

func BenchmarkLineRenderer_StyledScrollback(b *testing.B) {
	line := "\x1b[31mthe quick\x1b[0m brown \x1b[32mfox jumps\x1b[0m over the lazy dog\n"
	for i := 0; i < b.N; i++ {
		r := NewLineRenderer()
		for j := 0; j < 2000; j++ {
			r.Append([]byte(line))
		}
		_ = r.Content()
	}
}
