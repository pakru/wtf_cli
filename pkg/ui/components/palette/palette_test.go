package palette

import (
	"testing"

	"charm.land/lipgloss/v2"
)

func TestCommandPalette_ClampsToSmallWidth(t *testing.T) {
	p := NewCommandPalette()
	p.SetSize(20, 8)
	p.Show()

	view := p.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if got := lipgloss.Width(view); got > 20 {
		t.Fatalf("expected width <= 20, got %d", got)
	}
}
