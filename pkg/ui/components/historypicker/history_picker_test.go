package historypicker

import (
	"testing"

	"wtf_cli/pkg/ui/components/testutils"
)

func TestNewHistoryPickerPanel(t *testing.T) {
	picker := NewHistoryPickerPanel()
	if picker == nil {
		t.Fatal("NewHistoryPickerPanel returned nil")
	}
	if picker.visible {
		t.Error("New picker should not be visible")
	}
}

func TestShow(t *testing.T) {
	picker := NewHistoryPickerPanel()
	commands := []string{"ls -la", "cd /tmp", "git status"}

	picker.Show("", commands)

	if !picker.visible {
		t.Error("Picker should be visible after Show()")
	}
	if len(picker.commands) != 3 {
		t.Errorf("Expected 3 commands, got %d", len(picker.commands))
	}
	if picker.selected != 0 {
		t.Errorf("Expected selected=0, got %d", picker.selected)
	}
}

func TestShow_WithInitialFilter(t *testing.T) {
	picker := NewHistoryPickerPanel()
	commands := []string{"git status", "git commit", "ls -la", "git push"}

	picker.Show("git", commands)

	if picker.filter != "git" {
		t.Errorf("Expected filter='git', got '%s'", picker.filter)
	}

	// Should filter to only git commands
	if len(picker.filtered) != 3 {
		t.Errorf("Expected 3 filtered commands, got %d", len(picker.filtered))
	}
}

func TestHide(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.Show("", []string{"ls"})

	if !picker.visible {
		t.Error("Picker should be visible after Show()")
	}

	picker.Hide()

	if picker.visible {
		t.Error("Picker should not be visible after Hide()")
	}
}

func TestUpdate_Navigation(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"cmd1", "cmd2", "cmd3", "cmd4", "cmd5"}
	picker.Show("", commands)

	// Test down
	picker.Update(testutils.TestKeyDown)
	if picker.selected != 1 {
		t.Errorf("After down, expected selected=1, got %d", picker.selected)
	}

	// Test down again
	picker.Update(testutils.TestKeyDown)
	if picker.selected != 2 {
		t.Errorf("After second down, expected selected=2, got %d", picker.selected)
	}

	// Test up
	picker.Update(testutils.TestKeyUp)
	if picker.selected != 1 {
		t.Errorf("After up, expected selected=1, got %d", picker.selected)
	}

	// Test up at boundary (should not go below 0)
	picker.selected = 0
	picker.Update(testutils.TestKeyUp)
	if picker.selected != 0 {
		t.Errorf("Up at top should stay at 0, got %d", picker.selected)
	}

	// Test down at boundary (should not exceed length)
	picker.selected = 4
	picker.Update(testutils.TestKeyDown)
	if picker.selected != 4 {
		t.Errorf("Down at bottom should stay at 4, got %d", picker.selected)
	}
}

func TestUpdate_HomeEnd(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"cmd1", "cmd2", "cmd3", "cmd4", "cmd5"}
	picker.Show("", commands)

	picker.selected = 2

	// Test Home
	picker.Update(testutils.TestKeyHome)
	if picker.selected != 0 {
		t.Errorf("Home should set selected=0, got %d", picker.selected)
	}

	// Test End
	picker.Update(testutils.TestKeyEnd)
	if picker.selected != 4 {
		t.Errorf("End should set selected=4, got %d", picker.selected)
	}
}

func TestUpdate_PgUpPgDown(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	var commands []string
	for i := 0; i < 20; i++ {
		commands = append(commands, "cmd"+string(rune('A'+i)))
	}
	picker.Show("", commands)

	picker.selected = 10

	// Test PgUp (should go up by listHeight)
	picker.Update(testutils.TestKeyPgUp)
	if picker.selected >= 10 {
		t.Errorf("PgUp should decrease selected, got %d", picker.selected)
	}

	// Test PgDown
	picker.Update(testutils.TestKeyPgDown)
	// Should increase
}

func TestUpdate_EnterSelect(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"ls -la", "cd /tmp", "git status"}
	picker.Show("", commands)

	picker.selected = 1

	cmd := picker.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Enter should return a command")
	}

	msg := cmd()
	selectMsg, ok := msg.(HistoryPickerSelectMsg)
	if !ok {
		t.Fatalf("Expected HistoryPickerSelectMsg, got %T", msg)
	}

	if selectMsg.Command != "cd /tmp" {
		t.Errorf("Expected 'cd /tmp', got '%s'", selectMsg.Command)
	}

	if picker.visible {
		t.Error("Picker should be hidden after selection")
	}
}

func TestUpdate_EscCancel(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	picker.Show("", []string{"ls"})

	cmd := picker.Update(testutils.TestKeyEsc)
	if cmd == nil {
		t.Fatal("Esc should return a command")
	}

	msg := cmd()
	_, ok := msg.(HistoryPickerCancelMsg)
	if !ok {
		t.Fatalf("Expected HistoryPickerCancelMsg, got %T", msg)
	}

	if picker.visible {
		t.Error("Picker should be hidden after cancel")
	}
}

func TestUpdate_Filtering(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"git status", "git commit", "ls -la", "cd /tmp", "git push"}
	picker.Show("", commands)

	// Type 'g'
	picker.Update(testutils.NewTextKeyPressMsg("g"))
	if picker.filter != "g" {
		t.Errorf("Expected filter='g', got '%s'", picker.filter)
	}
	if len(picker.filtered) != 3 {
		t.Errorf("Expected 3 filtered items for 'g', got %d", len(picker.filtered))
	}

	// Type 'i'
	picker.Update(testutils.NewTextKeyPressMsg("i"))
	if picker.filter != "gi" {
		t.Errorf("Expected filter='gi', got '%s'", picker.filter)
	}
	if len(picker.filtered) != 3 {
		t.Errorf("Expected 3 filtered items for 'gi', got %d", len(picker.filtered))
	}

	// Type 't'
	picker.Update(testutils.NewTextKeyPressMsg("t"))
	if picker.filter != "git" {
		t.Errorf("Expected filter='git', got '%s'", picker.filter)
	}
	if len(picker.filtered) != 3 {
		t.Errorf("Expected 3 filtered items for 'git', got %d", len(picker.filtered))
	}
}

func TestUpdate_Backspace(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"git status", "ls -la"}
	picker.Show("git", commands)

	if len(picker.filtered) != 1 {
		t.Errorf("Initial filter should match 1 command, got %d", len(picker.filtered))
	}

	// Backspace once
	picker.Update(testutils.TestKeyBackspace)
	if picker.filter != "gi" {
		t.Errorf("Expected filter='gi' after backspace, got '%s'", picker.filter)
	}

	// Backspace again
	picker.Update(testutils.TestKeyBackspace)
	if picker.filter != "g" {
		t.Errorf("Expected filter='g' after second backspace, got '%s'", picker.filter)
	}

	// Backspace last char
	picker.Update(testutils.TestKeyBackspace)
	if picker.filter != "" {
		t.Errorf("Expected empty filter after third backspace, got '%s'", picker.filter)
	}
	if len(picker.filtered) != 2 {
		t.Errorf("Expected all commands with empty filter, got %d", len(picker.filtered))
	}

	// Backspace on empty filter should do nothing
	picker.Update(testutils.TestKeyBackspace)
	if picker.filter != "" {
		t.Errorf("Backspace on empty should stay empty, got '%s'", picker.filter)
	}
}

func TestUpdate_CtrlU(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"git status", "ls -la"}
	picker.Show("git", commands)

	// Ctrl+U should clear entire filter
	picker.Update(testutils.NewCtrlKeyPressMsg('u'))
	if picker.filter != "" {
		t.Errorf("Ctrl+U should clear filter, got '%s'", picker.filter)
	}
	if len(picker.filtered) != 2 {
		t.Errorf("Expected all commands after clear, got %d", len(picker.filtered))
	}
}

func TestFiltering_CaseInsensitive(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"Git Status", "GIT COMMIT", "ls -la"}
	picker.Show("", commands)

	picker.filter = "git"
	picker.updateFiltered()

	if len(picker.filtered) != 2 {
		t.Errorf("Case-insensitive filter should match 2 commands, got %d", len(picker.filtered))
	}
}

func TestFiltering_Substring(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"git commit -m 'test'", "ls -la", "cd /tmp"}
	picker.Show("", commands)

	picker.filter = "commit"
	picker.updateFiltered()

	if len(picker.filtered) != 1 {
		t.Errorf("Substring filter should match 1 command, got %d", len(picker.filtered))
	}
	if len(picker.filtered) > 0 && picker.filtered[0] != "git commit -m 'test'" {
		t.Errorf("Expected 'git commit -m 'test'', got '%s'", picker.filtered[0])
	}
}

func TestFiltering_EmptyResult(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	commands := []string{"ls -la", "cd /tmp"}
	picker.Show("", commands)

	picker.filter = "nonexistent"
	picker.updateFiltered()

	if len(picker.filtered) != 0 {
		t.Errorf("Filter with no matches should return empty, got %d", len(picker.filtered))
	}
}

func TestView_Visible(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	picker.Show("", []string{"ls"})

	view := picker.View()
	if view == "" {
		t.Error("View should return content when visible")
	}
	if !contains(view, "Command History Search") {
		t.Error("View should contain title")
	}
}

func TestView_Hidden(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	picker.Show("", []string{"ls"})
	picker.Hide()

	view := picker.View()
	if view != "" {
		t.Error("View should return empty string when hidden")
	}
}

func TestView_EmptyHistory(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	picker.Show("", []string{})

	view := picker.View()
	if !contains(view, "No commands in history") {
		t.Error("View should indicate empty history")
	}
}

func TestView_NoMatches(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 24)
	picker.Show("xyz", []string{"ls", "cd"})

	view := picker.View()
	if !contains(view, "No matching commands") {
		t.Error("View should indicate no matches")
	}
}

func TestScrolling(t *testing.T) {
	picker := NewHistoryPickerPanel()
	picker.SetSize(80, 10) // Small height to force scrolling

	// Create many commands
	var commands []string
	for i := 0; i < 50; i++ {
		commands = append(commands, "command"+string(rune('A'+i)))
	}
	picker.Show("", commands)

	// Navigate down many times
	for i := 0; i < 20; i++ {
		picker.Update(testutils.TestKeyDown)
	}

	if picker.selected != 20 {
		t.Errorf("Expected selected=20, got %d", picker.selected)
	}

	// Scroll should have adjusted
	if picker.scroll == 0 {
		t.Error("Scroll should have moved from 0")
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) >= len(substr) &&
			(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
				containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
