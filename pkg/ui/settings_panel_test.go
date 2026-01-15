package ui

import (
	"testing"

	"wtf_cli/pkg/config"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewSettingsPanel(t *testing.T) {
	sp := NewSettingsPanel()

	if sp == nil {
		t.Fatal("NewSettingsPanel() returned nil")
	}

	if sp.visible {
		t.Error("Panel should not be visible initially")
	}
}

func TestSettingsPanel_Show(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()

	sp.Show(cfg, "/tmp/test_config.json")

	if !sp.visible {
		t.Error("Panel should be visible after Show()")
	}

	if sp.selected != 0 {
		t.Error("Selection should be reset to 0")
	}

	if sp.editing {
		t.Error("Should not be in editing mode")
	}

	if len(sp.fields) == 0 {
		t.Error("Fields should be populated")
	}
}

func TestSettingsPanel_Hide(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.Hide()

	if sp.visible {
		t.Error("Panel should not be visible after Hide()")
	}
}

func TestSettingsPanel_Navigation(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move down
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if sp.selected != 1 {
		t.Errorf("Expected selected=1, got %d", sp.selected)
	}

	// Move down again
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	if sp.selected != 2 {
		t.Errorf("Expected selected=2, got %d", sp.selected)
	}

	// Move up
	sp.Update(tea.KeyMsg{Type: tea.KeyUp})
	if sp.selected != 1 {
		t.Errorf("Expected selected=1, got %d", sp.selected)
	}

	// Can't go above 0
	sp.Update(tea.KeyMsg{Type: tea.KeyUp})
	sp.Update(tea.KeyMsg{Type: tea.KeyUp})
	if sp.selected != 0 {
		t.Errorf("Expected selected=0, got %d", sp.selected)
	}
}

func TestSettingsPanel_EditMode(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move to Model field (index 1)
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Enter edit mode
	sp.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !sp.editing {
		t.Error("Should be in editing mode after Enter")
	}

	// Type something
	sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'t', 'e', 's', 't'}})

	if sp.editValue != cfg.OpenRouter.Model+"test" {
		t.Errorf("Expected edit value to contain 'test', got %q", sp.editValue)
	}

	// Backspace
	sp.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	expected := cfg.OpenRouter.Model + "tes"
	if sp.editValue != expected {
		t.Errorf("Expected %q, got %q", expected, sp.editValue)
	}

	// Cancel edit
	sp.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if sp.editing {
		t.Error("Should exit editing mode after Esc")
	}
}

func TestSettingsPanel_BoolToggle(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()
	cfg.DryRun = false
	sp.Show(cfg, "/tmp/test_config.json")

	// Navigate to Dry Run field (last one, index 7)
	for i := 0; i < 7; i++ {
		sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	// Initial value should be false
	if sp.fields[7].Value != "false" {
		t.Errorf("Expected 'false', got %q", sp.fields[7].Value)
	}

	// Toggle with Enter
	sp.Update(tea.KeyMsg{Type: tea.KeyEnter})

	// Should toggle to true
	if sp.fields[7].Value != "true" {
		t.Errorf("Expected 'true' after toggle, got %q", sp.fields[7].Value)
	}

	// Should be marked as changed
	if !sp.changed {
		t.Error("Should be marked as changed")
	}
}

func TestSettingsPanel_ValidateValue(t *testing.T) {
	sp := NewSettingsPanel()

	tests := []struct {
		fieldType string
		value     string
		valid     bool
	}{
		{"int", "123", true},
		{"int", "abc", false},
		{"int", "12.5", false},
		{"float", "0.7", true},
		{"float", "1", true},
		{"float", "abc", false},
		{"bool", "true", true},
		{"bool", "false", true},
		{"bool", "yes", false},
		{"string", "anything", true},
		{"string", "", true},
	}

	for _, tt := range tests {
		result := sp.validateValue(tt.fieldType, tt.value)
		if result != tt.valid {
			t.Errorf("validateValue(%q, %q) = %v, want %v",
				tt.fieldType, tt.value, result, tt.valid)
		}
	}
}

func TestSettingsPanel_ApplyField(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Modify model field
	sp.fields[1].Value = "new-model"
	sp.applyField(&sp.fields[1])

	if sp.config.OpenRouter.Model != "new-model" {
		t.Errorf("Expected model 'new-model', got %q", sp.config.OpenRouter.Model)
	}

	// Modify temperature
	sp.fields[2].Value = "0.9"
	sp.applyField(&sp.fields[2])

	if sp.config.OpenRouter.Temperature != 0.9 {
		t.Errorf("Expected temperature 0.9, got %f", sp.config.OpenRouter.Temperature)
	}

	// Modify buffer size
	sp.fields[5].Value = "5000"
	sp.applyField(&sp.fields[5])

	if sp.config.BufferSize != 5000 {
		t.Errorf("Expected buffer size 5000, got %d", sp.config.BufferSize)
	}
}

func TestSettingsPanel_View(t *testing.T) {
	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.SetSize(80, 24)
	sp.Show(cfg, "/tmp/test_config.json")

	view := sp.View()

	if view == "" {
		t.Error("View should not be empty when visible")
	}

	// Should contain title
	if !containsString(view, "Settings") {
		t.Error("View should contain 'Settings' title")
	}

	// Should contain field labels
	if !containsString(view, "API Key") {
		t.Error("View should contain 'API Key' label")
	}

	if !containsString(view, "Model") {
		t.Error("View should contain 'Model' label")
	}
}

func TestSettingsPanel_ViewHidden(t *testing.T) {
	sp := NewSettingsPanel()
	sp.SetSize(80, 24)

	view := sp.View()

	if view != "" {
		t.Error("View should be empty when not visible")
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && 
		(s == substr || len(s) > len(substr) && 
			(s[:len(substr)] == substr || containsString(s[1:], substr)))
}
