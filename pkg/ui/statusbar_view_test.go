package ui

import (
	"strings"
	"testing"
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

	rendered := sb.Render()

	// Should contain wtf_cli marker
	if !strings.Contains(rendered, "[wtf_cli]") {
		t.Error("Expected [wtf_cli] marker")
	}

	// Should contain directory
	if !strings.Contains(rendered, "/test") {
		t.Error("Expected directory")
	}

	// Should contain help text
	if !strings.Contains(rendered, "Press / for commands") {
		t.Error("Expected help text")
	}
}

func TestStatusBarView_Truncation(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(30)
	sb.SetDirectory("/very/long/directory/path/that/will/be/truncated")

	rendered := sb.Render()

	// Should be truncated
	if strings.Contains(rendered, "be/truncated") {
		t.Error("Expected long path to be truncated")
	}

	// Should have truncation indicator
	if !strings.Contains(rendered, "...") {
		t.Error("Expected truncation indicator")
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

func TestStatusBarView_MessagePriority(t *testing.T) {
	sb := NewStatusBarView()
	sb.SetWidth(100)
	sb.SetDirectory("/home")
	sb.SetMessage("Alert!")

	rendered := sb.Render()

	// Message should be shown
	if !strings.Contains(rendered, "Alert!") {
		t.Error("Expected message to be displayed")
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
