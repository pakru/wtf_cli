package sidebar

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
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

func TestSidebar_EmojiPrefixDoesNotOverflowBoxHeight(t *testing.T) {
	// Regression: the assistant prefix emoji (🖥️ = U+1F5A5 + VS16) is measured
	// as width 1 by go-runewidth but width 2 by lipgloss/the terminal. When such a
	// line wraps to fill the width, the over-wide rendered line previously made the
	// border box wrap it onto an extra row, growing the box past its allotted height.
	for _, width := range []int{30, 40, 46, 60, 80} {
		s := NewSidebar()
		s.SetSize(width, 18)
		s.StartAssistantMessageWithContent(strings.Join([]string{
			"🖥️ **Assistant:** While your current output only shows the Cursor repo failing, if it happens to multiple repos here are the primary reasons it occurs.",
			"Many ISPs, corporate networks, or public Wi-Fi hotspots use Transparent Proxies to save bandwidth and cache index files.",
		}, "\n"))
		s.Show()

		view := s.View()
		gotHeight := lipgloss.Height(view)
		if gotHeight != s.height {
			t.Fatalf("width=%d: box height %d != allotted %d (vertical overflow)", width, gotHeight, s.height)
		}
		for i, row := range strings.Split(view, "\n") {
			if w := lipgloss.Width(row); w != width {
				t.Fatalf("width=%d: row %d has width %d (expected %d)", width, i, w, width)
			}
		}
	}
}

func TestSidebarSelectionPointMapsMessageViewport(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 12)
	s.SetContent("alpha\nbravo\ncharlie")
	s.Show()

	originX := 10
	// Viewport starts after border(1) + title(1) + emptyLine(1).
	row, col, ok := s.SelectionPoint(originX+sidebarBorderSize+sidebarPaddingH+1, sidebarBorderSize+1+1, originX)
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
	s.SetContent("alpha\nbravo\ncharlie")
	s.Show()

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
