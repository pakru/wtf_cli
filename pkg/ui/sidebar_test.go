package ui

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
