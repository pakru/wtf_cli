package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/ai"
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
	withTempHome(t, nil)

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
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.Hide()

	if sp.visible {
		t.Error("Panel should not be visible after Hide()")
	}
}

func TestSettingsPanel_Navigation(t *testing.T) {
	withTempHome(t, nil)

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
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move to Model field (index 2)
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Enter edit mode
	sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})

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

func TestSettingsPanel_EditModeCursorNavigation(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !sp.editing {
		t.Fatal("Should be in editing mode after Enter")
	}

	sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("abcd")})
	if sp.editValue != "abcd" {
		t.Fatalf("Expected edit value %q, got %q", "abcd", sp.editValue)
	}

	sp.Update(tea.KeyMsg{Type: tea.KeyLeft})
	sp.Update(tea.KeyMsg{Type: tea.KeyLeft})
	sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'X'}})
	if sp.editValue != "abXcd" {
		t.Fatalf("Expected edit value %q, got %q", "abXcd", sp.editValue)
	}

	sp.Update(tea.KeyMsg{Type: tea.KeyHome})
	sp.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}})
	if sp.editValue != "ZabXcd" {
		t.Fatalf("Expected edit value %q, got %q", "ZabXcd", sp.editValue)
	}

	sp.Update(tea.KeyMsg{Type: tea.KeyEnd})
	sp.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	if sp.editValue != "ZabXc" {
		t.Fatalf("Expected edit value %q, got %q", "ZabXc", sp.editValue)
	}

	sp.Update(tea.KeyMsg{Type: tea.KeyLeft})
	sp.Update(tea.KeyMsg{Type: tea.KeyDelete})
	if sp.editValue != "ZabX" {
		t.Fatalf("Expected edit value %q, got %q", "ZabX", sp.editValue)
	}
}

func TestSettingsPanel_EditModePasteUsesRunes(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !sp.editing {
		t.Fatal("Should be in editing mode after Enter")
	}

	paste := tea.KeyMsg{
		Type:  tea.KeyRunes,
		Runes: []rune("sk-or-v1-123\n"),
		Paste: true,
	}
	sp.Update(paste)

	if sp.editValue != "sk-or-v1-123" {
		t.Fatalf("Expected edit value %q, got %q", "sk-or-v1-123", sp.editValue)
	}
	if sp.editCursor != len([]rune(sp.editValue)) {
		t.Fatalf("Expected cursor at %d, got %d", len([]rune(sp.editValue)), sp.editCursor)
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
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Modify model field
	sp.fields[2].Value = "new-model"
	sp.applyField(&sp.fields[2])

	if sp.config.OpenRouter.Model != "new-model" {
		t.Errorf("Expected model 'new-model', got %q", sp.config.OpenRouter.Model)
	}

	// Modify temperature
	sp.fields[3].Value = "0.9"
	sp.applyField(&sp.fields[3])

	if sp.config.OpenRouter.Temperature != 0.9 {
		t.Errorf("Expected temperature 0.9, got %f", sp.config.OpenRouter.Temperature)
	}

	// Modify buffer size
	sp.fields[6].Value = "5000"
	sp.applyField(&sp.fields[6])

	if sp.config.BufferSize != 5000 {
		t.Errorf("Expected buffer size 5000, got %d", sp.config.BufferSize)
	}
}

func TestSettingsPanel_View(t *testing.T) {
	withTempHome(t, nil)

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

func TestSettingsPanel_ModelPicker(t *testing.T) {
	withTempHome(t, func(home string) {
		cachePath := filepath.Join(home, ".wtf_cli", "models_cache.json")
		cache := ai.ModelCache{
			UpdatedAt: time.Date(2025, 1, 15, 12, 0, 0, 0, time.UTC),
			Models: []ai.ModelInfo{
				{ID: "model-a", Name: "Model A"},
				{ID: "model-b", Name: "Model B"},
			},
		}
		if err := ai.SaveModelCache(cachePath, cache); err != nil {
			t.Fatalf("SaveModelCache() error: %v", err)
		}
	})

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move to Model field (index 2)
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	sp.Update(tea.KeyMsg{Type: tea.KeyDown})

	// Open model picker
	cmd := sp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Expected openModelPickerMsg command")
	}
	msg := cmd()
	openMsg, ok := msg.(openModelPickerMsg)
	if !ok {
		t.Fatalf("Expected openModelPickerMsg, got %T", msg)
	}
	if len(openMsg.options) != 2 {
		t.Fatalf("Expected 2 model options, got %d", len(openMsg.options))
	}
	if openMsg.current != cfg.OpenRouter.Model {
		t.Fatalf("Expected current model %q, got %q", cfg.OpenRouter.Model, openMsg.current)
	}
	if openMsg.apiURL != cfg.OpenRouter.APIURL {
		t.Fatalf("Expected apiURL %q, got %q", cfg.OpenRouter.APIURL, openMsg.apiURL)
	}

	picker := NewModelPickerPanel()
	picker.SetSize(80, 24)
	picker.Show(openMsg.options, openMsg.current)

	// Select second model
	picker.Update(tea.KeyMsg{Type: tea.KeyDown})
	cmd = picker.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Expected modelPickerSelectMsg command")
	}
	msg = cmd()
	selectMsg, ok := msg.(modelPickerSelectMsg)
	if !ok {
		t.Fatalf("Expected modelPickerSelectMsg, got %T", msg)
	}
	if picker.IsVisible() {
		t.Fatal("Expected model picker to close after selection")
	}

	sp.SetModelValue(selectMsg.modelID)
	if sp.config.OpenRouter.Model != "model-b" {
		t.Errorf("Expected model 'model-b', got %q", sp.config.OpenRouter.Model)
	}
}

func TestSettingsPanel_OpenLogLevelPicker(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move to Log Level field (index 8)
	for i := 0; i < 8; i++ {
		sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	cmd := sp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Expected openOptionPickerMsg command")
	}
	msg := cmd()
	openMsg, ok := msg.(openOptionPickerMsg)
	if !ok {
		t.Fatalf("Expected openOptionPickerMsg, got %T", msg)
	}
	if openMsg.fieldKey != "log_level" {
		t.Fatalf("Expected fieldKey log_level, got %q", openMsg.fieldKey)
	}
	if openMsg.current != normalizeLogLevel(cfg.LogLevel) {
		t.Fatalf("Expected current log level %q, got %q", normalizeLogLevel(cfg.LogLevel), openMsg.current)
	}
}

func TestSettingsPanel_OpenLogFormatPicker(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move to Log Format field (index 9)
	for i := 0; i < 9; i++ {
		sp.Update(tea.KeyMsg{Type: tea.KeyDown})
	}

	cmd := sp.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Expected openOptionPickerMsg command")
	}
	msg := cmd()
	openMsg, ok := msg.(openOptionPickerMsg)
	if !ok {
		t.Fatalf("Expected openOptionPickerMsg, got %T", msg)
	}
	if openMsg.fieldKey != "log_format" {
		t.Fatalf("Expected fieldKey log_format, got %q", openMsg.fieldKey)
	}
	if openMsg.current != strings.ToLower(strings.TrimSpace(cfg.LogFormat)) {
		t.Fatalf("Expected current log format %q, got %q", cfg.LogFormat, openMsg.current)
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || containsString(s[1:], substr)))
}

func withTempHome(t *testing.T, setup func(string)) {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome, hadHome := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatalf("Setenv(HOME) failed: %v", err)
	}
	t.Cleanup(func() {
		if hadHome {
			_ = os.Setenv("HOME", oldHome)
		} else {
			_ = os.Unsetenv("HOME")
		}
	})
	if setup != nil {
		setup(tmpDir)
	}
}
