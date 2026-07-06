package utils

import (
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestEscapeControl_PlainStringUnchanged(t *testing.T) {
	if got := EscapeControl("/etc/hosts"); got != "/etc/hosts" {
		t.Errorf("EscapeControl(plain) = %q, want unchanged", got)
	}
}

func TestEscapeControl_ControlCharactersAreQuoted(t *testing.T) {
	tests := []string{
		"a\nb",
		"a\tb",
		"a\x1bb", // ESC
		"a\x00b", // NUL
		"a\x7fb", // DEL
		"a\xC3b", // invalid UTF-8
	}
	for _, s := range tests {
		got := EscapeControl(s)
		want := strconv.Quote(s)
		if got != want {
			t.Errorf("EscapeControl(%q) = %q, want %q", s, got, want)
		}
		if strings.ContainsAny(got, "\n\t\x1b\x00\x7f") {
			t.Errorf("EscapeControl(%q) result still contains a raw control byte: %q", s, got)
		}
	}
}

func TestTailPreservingTruncate_FitsUnchanged(t *testing.T) {
	if got := TailPreservingTruncate("short", 10); got != "short" {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestTailPreservingTruncate_KeepsSuffix(t *testing.T) {
	long := "/safe/looking/prefix/dir/secretfile.txt"
	got := TailPreservingTruncate(long, 20)
	if !strings.HasSuffix(got, "secretfile.txt") {
		t.Fatalf("expected the tail to survive truncation, got %q", got)
	}
	if ansi.StringWidth(got) > 20 {
		t.Fatalf("result exceeds requested width: %q (%d)", got, ansi.StringWidth(got))
	}
	if !strings.HasPrefix(got, "…") {
		t.Errorf("expected a leading ellipsis marker, got %q", got)
	}
}

func TestTailPreservingTruncate_NeverExceedsWidth(t *testing.T) {
	long := strings.Repeat("x", 500)
	for width := 1; width <= 30; width++ {
		got := TailPreservingTruncate(long, width)
		if ansi.StringWidth(got) > width {
			t.Fatalf("width=%d: result %q has width %d", width, got, ansi.StringWidth(got))
		}
	}
}

func TestTailPreservingTruncate_ZeroWidth(t *testing.T) {
	if got := TailPreservingTruncate("anything", 0); got != "" {
		t.Errorf("got %q, want empty string for width<=0", got)
	}
}
