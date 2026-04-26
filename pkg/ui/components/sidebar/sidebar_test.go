package sidebar

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_Table(t *testing.T) {
	input := strings.Join([]string{
		"| Situation | Fix |",
		"| --- | --- |",
		"| A | Do this |",
		"| Longer cell | Another fix |",
	}, "\n")

	lines := renderMarkdown(input, 60)
	if len(lines) < 3 {
		t.Fatalf("Expected table lines, got %d", len(lines))
	}

	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "| Situation") {
		t.Fatalf("Expected header row to render, got:\n%s", joined)
	}
	if !strings.Contains(joined, "| ---") {
		t.Fatalf("Expected separator row to render, got:\n%s", joined)
	}
	if !strings.Contains(joined, "Longer cell") {
		t.Fatalf("Expected body row to render, got:\n%s", joined)
	}
}

func TestRenderMarkdown_TableFallback(t *testing.T) {
	input := strings.Join([]string{
		"| A | B | C |",
		"| --- | --- | --- |",
		"| one | two | three |",
	}, "\n")

	lines := renderMarkdown(input, 10)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "one") {
		t.Fatalf("Expected fallback to include cell text, got:\n%s", joined)
	}
}

func TestSidebarSelectionPointMapsMessageViewport(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 12)
	s.Show("Title", "alpha\nbravo\ncharlie")

	originX := 10
	row, col, ok := s.SelectionPoint(originX+sidebarBorderSize+sidebarPaddingH+1, sidebarBorderSize+sidebarPaddingV+1, originX)
	if !ok {
		t.Fatal("expected message viewport point to be selectable")
	}
	if row != 0 || col != 1 {
		t.Fatalf("expected row=0 col=1, got row=%d col=%d", row, col)
	}

	if _, _, ok := s.SelectionPoint(originX+sidebarBorderSize+sidebarPaddingH+1, sidebarBorderSize+sidebarPaddingV, originX); ok {
		t.Fatal("expected title row to be outside selectable message viewport")
	}
}

func TestSidebarFinishSelectionExtractsText(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 12)
	s.Show("Title", "alpha\nbravo\ncharlie")

	s.StartSelection(0, 1)
	s.UpdateSelection(1, 3)

	got := s.FinishSelection()
	want := "lpha\nbra"
	if got != want {
		t.Fatalf("expected selected text %q, got %q", want, got)
	}
	if s.HasActiveSelection() {
		t.Fatal("expected selection to be inactive after finish")
	}
}
