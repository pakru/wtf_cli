package sidebar

import (
	"strings"
	"testing"
)

func TestTokenizeBoldWords(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []markdownToken
	}{
		{
			"plain words",
			"hello world",
			[]markdownToken{{text: "hello"}, {text: "world"}},
		},
		{
			"bold words",
			"**bold** text",
			[]markdownToken{{text: "bold", bold: true}, {text: "text"}},
		},
		{
			"mixed",
			"plain **bold** again",
			[]markdownToken{{text: "plain"}, {text: "bold", bold: true}, {text: "again"}},
		},
		{
			"empty",
			"",
			nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tokenizeBoldWords(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("tokenizeBoldWords(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("token[%d]: got %+v, want %+v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestWrapTokens(t *testing.T) {
	t.Run("fits on one line", func(t *testing.T) {
		tokens := []markdownToken{{text: "hello"}, {text: "world"}}
		lines := wrapTokens(tokens, 20)
		if len(lines) != 1 {
			t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
		}
	})

	t.Run("wraps when too wide", func(t *testing.T) {
		tokens := []markdownToken{{text: "hello"}, {text: "world"}}
		lines := wrapTokens(tokens, 7) // "hello" fits, "world" overflows
		if len(lines) < 2 {
			t.Fatalf("expected wrap, got %d lines: %v", len(lines), lines)
		}
	})

	t.Run("zero width returns empty line", func(t *testing.T) {
		tokens := []markdownToken{{text: "hello"}}
		lines := wrapTokens(tokens, 0)
		if len(lines) != 1 || lines[0] != "" {
			t.Fatalf("expected single empty line, got %v", lines)
		}
	})
}

func TestRenderMarkdownLine(t *testing.T) {
	t.Run("empty line returns empty", func(t *testing.T) {
		lines := renderMarkdownLine("   ", 80)
		if len(lines) != 1 || lines[0] != "" {
			t.Fatalf("expected single empty line, got %v", lines)
		}
	})

	t.Run("plain text renders", func(t *testing.T) {
		lines := renderMarkdownLine("hello world", 80)
		joined := stripANSICodes(strings.Join(lines, " "))
		if !strings.Contains(joined, "hello") || !strings.Contains(joined, "world") {
			t.Fatalf("expected rendered words, got %v", lines)
		}
	})
}

func TestRenderCodeLine(t *testing.T) {
	t.Run("empty line pads to width", func(t *testing.T) {
		lines := renderCodeLine("", 10)
		if len(lines) != 1 {
			t.Fatalf("expected 1 line, got %d", len(lines))
		}
		plain := stripANSICodes(lines[0])
		if len(plain) != 10 {
			t.Errorf("expected 10-char padded line, got %q (len=%d)", plain, len(plain))
		}
	})

	t.Run("long line splits", func(t *testing.T) {
		lines := renderCodeLine("abcdefghij", 5)
		if len(lines) < 2 {
			t.Fatalf("expected split into multiple lines, got %v", lines)
		}
	})

	t.Run("zero width returns line as-is", func(t *testing.T) {
		lines := renderCodeLine("hello", 0)
		if len(lines) != 1 || lines[0] != "hello" {
			t.Fatalf("expected raw line returned, got %v", lines)
		}
	})
}

func TestIsTableRow(t *testing.T) {
	tests := []struct {
		line string
		want bool
	}{
		{"| A | B |", true},
		{"| --- | --- |", true},
		{"no pipes here", false},
		{"| only one |", false},
		{"", false},
	}

	for _, tt := range tests {
		got := isTableRow(tt.line)
		if got != tt.want {
			t.Errorf("isTableRow(%q) = %v, want %v", tt.line, got, tt.want)
		}
	}
}

func TestSplitTableRow(t *testing.T) {
	cells := splitTableRow("| hello | world |")
	if len(cells) != 2 {
		t.Fatalf("expected 2 cells, got %d: %v", len(cells), cells)
	}
	if cells[0] != "hello" || cells[1] != "world" {
		t.Errorf("expected [hello world], got %v", cells)
	}

	if splitTableRow("") != nil {
		t.Error("expected nil for empty line")
	}
}

func TestIsSeparatorRow(t *testing.T) {
	tests := []struct {
		cells []string
		want  bool
	}{
		{[]string{"---", "---"}, true},
		{[]string{":---", "---:"}, true},
		{[]string{"---", "text"}, false},
		{[]string{}, false},
		{[]string{"--"}, false},
	}

	for _, tt := range tests {
		got := isSeparatorRow(tt.cells)
		if got != tt.want {
			t.Errorf("isSeparatorRow(%v) = %v, want %v", tt.cells, got, tt.want)
		}
	}
}

func TestFitColumnWidths(t *testing.T) {
	t.Run("no shrink needed", func(t *testing.T) {
		out := fitColumnWidths([]int{3, 4}, 20)
		if out[0] != 3 || out[1] != 4 {
			t.Errorf("expected unchanged widths, got %v", out)
		}
	})

	t.Run("shrinks largest column first", func(t *testing.T) {
		out := fitColumnWidths([]int{10, 4}, 8)
		total := 0
		for _, w := range out {
			total += w
		}
		if total > 8 {
			t.Errorf("total %d exceeds maxContent 8, widths=%v", total, out)
		}
	})

	t.Run("zero maxContent gives minimum width 1", func(t *testing.T) {
		out := fitColumnWidths([]int{5, 5}, 0)
		for i, w := range out {
			if w != 1 {
				t.Errorf("expected min width 1 at index %d, got %d", i, w)
			}
		}
	})
}

func TestRenderMarkdownWithCommandLines_CodeBlock(t *testing.T) {
	content := "```\nls -la\necho hello\n```"
	lines, cmdRendered := renderMarkdownWithCommandLines(content, 40, nil)

	if len(cmdRendered) != 0 {
		t.Errorf("expected empty cmdRendered for nil input, got %v", cmdRendered)
	}
	joined := stripANSICodes(strings.Join(lines, "\n"))
	if !strings.Contains(joined, "ls -la") {
		t.Errorf("expected code content in output, got:\n%s", joined)
	}
}

func TestRenderMarkdownWithCommandLines_BRTags(t *testing.T) {
	content := "line one<br>line two<br/>line three"
	lines, _ := renderMarkdownWithCommandLines(content, 40, nil)
	joined := stripANSICodes(strings.Join(lines, "\n"))

	for _, want := range []string{"line one", "line two", "line three"} {
		if !strings.Contains(joined, want) {
			t.Errorf("expected %q in output, got:\n%s", want, joined)
		}
	}
}
