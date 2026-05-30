package sidebar

import (
	"testing"
)

func TestSplitByWidth(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		width int
		want  []string
	}{
		{"empty string", "", 10, []string{""}},
		{"fits in width", "hello", 10, []string{"hello"}},
		{"exact width", "hello", 5, []string{"hello"}},
		{"splits at boundary", "hello world", 5, []string{"hello", " worl", "d"}},
		{"zero width returns as-is", "hello", 0, []string{"hello"}},
		{"single wide char", "あ", 2, []string{"あ"}},
		{"wide chars split correctly", "ああ", 2, []string{"あ", "あ"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitByWidth(tt.text, tt.width)
			if len(got) != len(tt.want) {
				t.Fatalf("splitByWidth(%q, %d) = %v, want %v", tt.text, tt.width, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("part[%d]: got %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestTrimToWidth(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  string
	}{
		{"hello", 3, "hel"},
		{"hello", 10, "hello"},
		{"hello", 0, ""},
		{"", 5, ""},
		{"あいう", 4, "あい"},
	}

	for _, tt := range tests {
		got := trimToWidth(tt.text, tt.width)
		if got != tt.want {
			t.Errorf("trimToWidth(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
		}
	}
}

func TestTruncateToWidth(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "he..."},
		{"hi", 2, "hi"},
		{"hi", 1, "h"},
		{"hello", 0, ""},
		{"hello", 3, "hel"},
		{"hello world", 8, "hello..."},
	}

	for _, tt := range tests {
		got := truncateToWidth(tt.text, tt.width)
		if got != tt.want {
			t.Errorf("truncateToWidth(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
		}
	}
}

func TestPadPlain(t *testing.T) {
	tests := []struct {
		text  string
		width int
		want  string
	}{
		{"hi", 5, "hi   "},
		{"hello", 5, "hello"},
		{"hello", 3, "hello"},
		{"", 4, "    "},
		{"hi", 0, "hi"},
	}

	for _, tt := range tests {
		got := padPlain(tt.text, tt.width)
		if got != tt.want {
			t.Errorf("padPlain(%q, %d) = %q, want %q", tt.text, tt.width, got, tt.want)
		}
	}
}

func TestSanitizeContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"newlines preserved", "a\nb", "a\nb"},
		{"tabs preserved", "a\tb", "a\tb"},
		{"control chars stripped", "a\x01\x02b", "ab"},
		{"DEL stripped", "a\x7fb", "ab"},
		{"normal text", "hello world", "hello world"},
		{"unicode preserved", "hello 世界", "hello 世界"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := sanitizeContent(tt.input)
			if got != tt.want {
				t.Errorf("sanitizeContent(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestStripANSICodes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"plain text", "hello", "hello"},
		{"CSI reset", "\x1b[0mhello\x1b[0m", "hello"},
		{"CSI color", "\x1b[31mred\x1b[0m", "red"},
		{"OSC sequence", "\x1b]0;title\x07text", "text"},
		{"two-byte escape", "\x1b=rest", "rest"},
		{"truncated ESC at end", "hi\x1b", "hi"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripANSICodes(tt.input)
			if got != tt.want {
				t.Errorf("stripANSICodes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestMessagePrefix(t *testing.T) {
	if got := MessagePrefix("user"); got != "**You:** " {
		t.Errorf("user prefix = %q, want %q", got, "**You:** ")
	}
	if got := MessagePrefix("assistant"); got != "**Assistant:** " {
		t.Errorf("assistant prefix = %q, want %q", got, "**Assistant:** ")
	}
}

func TestMessagePrefix_Tool(t *testing.T) {
	if got := MessagePrefix("tool"); got != "**Tool:** " {
		t.Errorf("tool prefix = %q, want %q", got, "**Tool:** ")
	}
}

func TestMessagePrefix_Error(t *testing.T) {
	if got := MessagePrefix("error"); got != "Error: " {
		t.Errorf("error prefix = %q, want %q", got, "Error: ")
	}
}
