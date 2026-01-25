package statusbar

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func TestNewStatusBarView(t *testing.T) {
	sb := NewStatusBarView()

	if sb == nil {
		t.Fatal("NewStatusBarView() returned nil")
	}

	if sb.currentDir == "" {
		t.Error("Expected currentDir to be set")
	}

	if sb.width != 80 {
		t.Errorf("Expected default width 80, got %d", sb.width)
	}
}

func TestStatusBarView_SetDirectory(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(100)

	sb.SetDirectory("/home/user/projects")

	rendered := sb.Render()
	if !strings.Contains(rendered, "/home/user/projects") {
		t.Error("Expected directory in rendered output")
	}
}

func TestStatusBarView_SetMessage(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(100)
	sb.SetModel("test-model")

	sb.SetMessage("Important notification")

	rendered := sb.Render()
	if !strings.Contains(rendered, "Important notification") {
		t.Error("Expected message in rendered output")
	}

	// Message takes priority over directory
	if strings.Contains(rendered, "Press / for commands") {
		t.Error("Expected message to replace directory info")
	}
}

func TestStatusBarView_Render(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(80)
	sb.SetDirectory("/test")
	sb.SetModel("model-1")

	rendered := sb.Render()

	// Should contain wtf_cli marker
	if !strings.Contains(rendered, "[wtf_cli]") {
		t.Error("Expected [wtf_cli] marker")
	}

	// Should contain directory
	if !strings.Contains(rendered, "/test") {
		t.Error("Expected directory")
	}

	// Should contain model label
	if !strings.Contains(rendered, "[llm]: model-1") {
		t.Error("Expected model indicator")
	}

	// Should contain help text
	if !strings.Contains(rendered, "Press / for commands") {
		t.Error("Expected help text")
	}
}

func TestStatusBarView_Truncation(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(80)
	longPath := "/very/long/directory/path/that/will/be/truncated"
	sb.SetDirectory(longPath)

	rendered := sb.Render()
	stripped := ansi.Strip(rendered)

	// Should be truncated
	if strings.Contains(stripped, longPath) {
		t.Error("Expected long path to be truncated")
	}

	// Should have middle truncation indicator
	if !strings.Contains(stripped, "/../") {
		t.Error("Expected middle truncation indicator")
	}
}

func TestStatusBarView_LayoutAlignment(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(80)
	sb.SetDirectory("/home/user/projects/wtf_cli/pkg/ui/components")
	sb.SetModel("model-1")

	rendered := sb.Render()
	stripped := ansi.Strip(rendered)
	trimmed := strings.TrimSpace(stripped)
	right := "[llm]: model-1 | Press / for commands"

	if !strings.HasPrefix(trimmed, "[wtf_cli]") {
		t.Fatalf("expected left content to start with [wtf_cli], got %q", trimmed)
	}
	if !strings.HasSuffix(trimmed, right) {
		t.Fatalf("expected right content to be aligned at end, got %q", trimmed)
	}
	if width := ansi.StringWidth(stripped); width != 80 {
		t.Fatalf("expected width 80, got %d", width)
	}

	idx := strings.Index(trimmed, right)
	if idx <= 0 {
		t.Fatalf("expected right content to appear, got %q", trimmed)
	}
	if trimmed[idx-1] != ' ' {
		t.Fatalf("expected space gap before right content, got %q", trimmed)
	}
}

func TestTruncatePath_ShortPath(t *testing.T) {
	path := "/home/user/a"
	got := truncatePath(path, 50)
	if got != path {
		t.Fatalf("expected %q, got %q", path, got)
	}
}

func TestTruncatePath_LongPath(t *testing.T) {
	path := "/home/user/projects/wtf_cli/pkg/ui"
	got := truncatePath(path, 30)
	expected := "/home/../wtf_cli/pkg/ui"
	if got != expected {
		t.Fatalf("expected %q, got %q", expected, got)
	}
}

func TestTruncatePath_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		maxWidth int
		expected string
	}{
		{name: "root", path: "/", maxWidth: 4, expected: "/"},
		{name: "home", path: "~", maxWidth: 4, expected: "~"},
		{name: "empty", path: "", maxWidth: 4, expected: ""},
		{name: "single", path: "single", maxWidth: 4, expected: "si.."},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := truncatePath(tc.path, tc.maxWidth)
			if got != tc.expected {
				t.Fatalf("expected %q, got %q", tc.expected, got)
			}
		})
	}
}

func TestStatusBarView_Width(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetDirectory("/test")

	widths := []int{40, 80, 120}

	for _, width := range widths {
		sb.SetWidth(width)
		rendered := sb.Render()

		// Lipgloss adds styling chars, so check it's reasonable
		// Just verify it doesn't panic and produces output
		if len(rendered) == 0 {
			t.Errorf("Expected non-empty output for width %d", width)
		}
	}
}

func TestStatusBarView_FullWidth(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(80)
	sb.SetDirectory("/home/user/projects/wtf_cli")
	sb.SetModel("model-1")

	rendered := sb.Render()
	stripped := ansi.Strip(rendered)
	if width := ansi.StringWidth(stripped); width != 80 {
		t.Fatalf("expected width 80, got %d", width)
	}
}

func TestStatusBarView_NarrowTerminal(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(40)
	sb.SetDirectory("/home/user/projects/wtf_cli/pkg/ui/components")
	sb.SetModel("model-1")

	rendered := sb.Render()
	stripped := ansi.Strip(rendered)

	if width := ansi.StringWidth(stripped); width != 40 {
		t.Fatalf("expected width 40, got %d", width)
	}
	if strings.Contains(stripped, "/home/user/projects") {
		t.Fatalf("expected path to be truncated or omitted at narrow width, got %q", stripped)
	}
	if !strings.Contains(stripped, "[llm]: model-1") {
		t.Fatalf("expected model label to remain visible at narrow width, got %q", stripped)
	}
}

func TestStatusBarView_MessagePriority(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(100)
	sb.SetDirectory("/home")
	sb.SetMessage("Alert!")
	sb.SetModel("model-2")

	rendered := sb.Render()

	// Message should be shown
	if !strings.Contains(rendered, "Alert!") {
		t.Error("Expected message to be displayed")
	}

	if !strings.Contains(rendered, "[llm]: model-2") {
		t.Error("Expected model indicator")
	}

	// Clear message
	sb.SetMessage("")
	rendered = sb.Render()

	// Directory should be shown now
	if !strings.Contains(rendered, "/home") {
		t.Error("Expected directory after clearing message")
	}
}

func TestStatusBarView_SetTheme(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(80)

	// Test setting different themes (just verify no panic)
	themes := []string{"cyan", "dark", "default", "invalid"}

	for _, theme := range themes {
		sb.SetTheme(theme)
		rendered := sb.Render()

		if len(rendered) == 0 {
			t.Errorf("Expected output for theme %s", theme)
		}
	}
}

func TestGetCurrentWorkingDir(t *testing.T) {
	dir := getCurrentWorkingDir()

	if dir == "" {
		t.Error("Expected non-empty directory")
	}

	// Should not be empty even on error
	if dir != "~" && !strings.Contains(dir, "/") {
		t.Error("Expected valid directory path or ~")
	}
}
