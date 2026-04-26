package selection

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestSelectionFinishPreservesRange(t *testing.T) {
	var sel Selection
	sel.Start(0, 1)
	sel.Update(1, 3)
	sel.Finish()

	if sel.Active {
		t.Fatal("expected selection to be inactive after Finish")
	}
	if sel.IsEmpty() {
		t.Fatal("expected Finish to preserve selected range")
	}
}

func TestExtractTextNormalizesReverseSelection(t *testing.T) {
	var sel Selection
	sel.Start(1, 3)
	sel.Update(0, 1)

	got := ExtractText([]string{"alpha", "bravo"}, sel)
	want := "lpha\nbra"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestApplyLineHighlightPreservesPlainText(t *testing.T) {
	line := "\x1b[31mhello\x1b[0m world"

	got := ApplyLineHighlight(line, 1, 4)
	if !strings.Contains(got, inverseOn) || !strings.Contains(got, inverseOff) {
		t.Fatalf("expected reverse-video markers in %q", got)
	}
	if ansi.Strip(got) != ansi.Strip(line) {
		t.Fatalf("expected highlighted text to preserve content, got %q", got)
	}
}
