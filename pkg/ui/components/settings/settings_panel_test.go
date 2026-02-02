package settings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/components/testutils"

	"charm.land/lipgloss/v2"
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

func TestSettingsPanel_ClampsToSmallWidth(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")
	sp.SetSize(30, 8)

	view := sp.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if got := lipgloss.Width(view); got > 30 {
		t.Fatalf("expected width <= 30, got %d", got)
	}
}

func TestSettingsPanel_Navigation(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	// Move down
	sp.Update(testutils.TestKeyDown)
	if sp.selected != 1 {
		t.Errorf("Expected selected=1, got %d", sp.selected)
	}

	// Move down again
	sp.Update(testutils.TestKeyDown)
	if sp.selected != 2 {
		t.Errorf("Expected selected=2, got %d", sp.selected)
	}

	// Move up
	sp.Update(testutils.TestKeyUp)
	if sp.selected != 1 {
		t.Errorf("Expected selected=1, got %d", sp.selected)
	}

	// Can't go above 0
	sp.Update(testutils.TestKeyUp)
	sp.Update(testutils.TestKeyUp)
	if sp.selected != 0 {
		t.Errorf("Expected selected=0, got %d", sp.selected)
	}
}

func TestSettingsPanel_EditMode(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.selected = findFieldIndex(t, sp, "model")

	// Enter edit mode using 'e' key
	sp.Update(testutils.NewTextKeyPressMsg("e"))

	if !sp.editing {
		t.Error("Should be in editing mode after 'e'")
	}

	// Type something
	sp.Update(testutils.NewTextKeyPressMsg("test"))

	if sp.editValue != cfg.OpenRouter.Model+"test" {
		t.Errorf("Expected edit value to contain 'test', got %q", sp.editValue)
	}

	// Backspace
	sp.Update(testutils.TestKeyBackspace)
	expected := cfg.OpenRouter.Model + "tes"
	if sp.editValue != expected {
		t.Errorf("Expected %q, got %q", expected, sp.editValue)
	}

	// Cancel edit
	sp.Update(testutils.TestKeyEsc)

	if sp.editing {
		t.Error("Should exit editing mode after Esc")
	}
}

func TestSettingsPanel_EditModeCursorNavigation(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.selected = findFieldIndex(t, sp, "api_url")

	// Use Enter key to enter edit mode for text field
	sp.Update(testutils.TestKeyEnter)
	if !sp.editing {
		t.Fatal("Should be in editing mode after Enter")
	}

	// Clear and start fresh for predictable testing
	sp.editValue = ""
	sp.editCursor = 0

	sp.Update(testutils.NewTextKeyPressMsg("abcd"))
	if sp.editValue != "abcd" {
		t.Fatalf("Expected edit value %q, got %q", "abcd", sp.editValue)
	}

	sp.Update(testutils.TestKeyLeft)
	sp.Update(testutils.TestKeyLeft)
	sp.Update(testutils.NewTextKeyPressMsg("X"))
	if sp.editValue != "abXcd" {
		t.Fatalf("Expected edit value %q, got %q", "abXcd", sp.editValue)
	}

	sp.Update(testutils.TestKeyHome)
	sp.Update(testutils.NewTextKeyPressMsg("Z"))
	if sp.editValue != "ZabXcd" {
		t.Fatalf("Expected edit value %q, got %q", "ZabXcd", sp.editValue)
	}

	sp.Update(testutils.TestKeyEnd)
	sp.Update(testutils.TestKeyBackspace)
	if sp.editValue != "ZabXc" {
		t.Fatalf("Expected edit value %q, got %q", "ZabXc", sp.editValue)
	}

	sp.Update(testutils.TestKeyLeft)
	sp.Update(testutils.TestKeyDelete)
	if sp.editValue != "ZabX" {
		t.Fatalf("Expected edit value %q, got %q", "ZabX", sp.editValue)
	}
}

func TestSettingsPanel_EditModePasteUsesText(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.selected = findFieldIndex(t, sp, "api_url")
	sp.Update(testutils.TestKeyEnter)
	if !sp.editing {
		t.Fatal("Should be in editing mode after Enter")
	}

	// Clear existing value for clean test
	sp.editValue = ""
	sp.editCursor = 0

	// Simulate paste by sending text (newlines should be filtered)
	sp.Update(testutils.NewTextKeyPressMsg("sk-or-v1-123"))

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

	modelIdx := findFieldIndex(t, sp, "model")
	sp.fields[modelIdx].Value = "new-model"
	sp.applyField(&sp.fields[modelIdx])

	if sp.config.OpenRouter.Model != "new-model" {
		t.Errorf("Expected model 'new-model', got %q", sp.config.OpenRouter.Model)
	}

	tempIdx := findFieldIndex(t, sp, "temperature")
	sp.fields[tempIdx].Value = "0.9"
	sp.applyField(&sp.fields[tempIdx])

	if sp.config.OpenRouter.Temperature != 0.9 {
		t.Errorf("Expected temperature 0.9, got %f", sp.config.OpenRouter.Temperature)
	}

	bufferIdx := findFieldIndex(t, sp, "buffer_size")
	sp.fields[bufferIdx].Value = "5000"
	sp.applyField(&sp.fields[bufferIdx])

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

	sp.selected = findFieldIndex(t, sp, "model")

	// Open model picker
	cmd := sp.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected openModelPickerMsg command")
	}
	msg := cmd()
	openMsg := msg.(picker.OpenModelPickerMsg)
	if openMsg.Current != "google/gemini-3.0-flash" {
		t.Errorf("Expected current 'google/gemini-3.0-flash', got %q", openMsg.Current)
	}
	if len(openMsg.Options) != 2 {
		t.Errorf("Expected 2 options, got %d", len(openMsg.Options))
	}
	if openMsg.APIURL != cfg.OpenRouter.APIURL {
		t.Fatalf("Expected apiURL %q, got %q", cfg.OpenRouter.APIURL, openMsg.APIURL)
	}

	modelPicker := picker.NewModelPickerPanel()
	modelPicker.SetSize(80, 24)
	modelPicker.Show(openMsg.Options, openMsg.Current, openMsg.FieldKey)

	// Select second model
	modelPicker.Update(testutils.TestKeyDown)
	cmd = modelPicker.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected modelPickerSelectMsg command")
	}
	msg = cmd()
	selectMsg := cmd().(picker.ModelPickerSelectMsg)
	if selectMsg.ModelID != "model-b" {
		t.Errorf("Expected model-b, got %q", selectMsg.ModelID)
	}
	if modelPicker.IsVisible() {
		t.Fatal("Expected model picker to close after selection")
	}

	sp.SetModelValue(selectMsg.ModelID)
	if sp.config.OpenRouter.Model != "model-b" {
		t.Errorf("Expected model 'model-b', got %q", sp.config.OpenRouter.Model)
	}
}

func TestSettingsPanel_OpenLogLevelPicker(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.selected = findFieldIndex(t, sp, "log_level")

	cmd := sp.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected openOptionPickerMsg command")
	}
	msg := cmd()
	openMsg := msg.(picker.OpenOptionPickerMsg)
	if openMsg.FieldKey != "log_level" {
		t.Fatalf("Expected fieldKey log_level, got %q", openMsg.FieldKey)
	}
	if openMsg.Current != normalizeLogLevel(cfg.LogLevel) {
		t.Fatalf("Expected current log level %q, got %q", normalizeLogLevel(cfg.LogLevel), openMsg.Current)
	}
}

func TestSettingsPanel_OpenLogFormatPicker(t *testing.T) {
	withTempHome(t, nil)

	sp := NewSettingsPanel()
	cfg := config.Default()
	sp.Show(cfg, "/tmp/test_config.json")

	sp.selected = findFieldIndex(t, sp, "log_format")

	cmd := sp.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected openOptionPickerMsg command")
	}
	msg := cmd()
	openMsg := msg.(picker.OpenOptionPickerMsg)
	if openMsg.FieldKey != "log_format" {
		t.Fatalf("Expected fieldKey log_format, got %q", openMsg.FieldKey)
	}
	if openMsg.Current != strings.ToLower(strings.TrimSpace(cfg.LogFormat)) {
		t.Fatalf("Expected current log format %q, got %q", cfg.LogFormat, openMsg.Current)
	}
}

func containsString(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 &&
		(s == substr || len(s) > len(substr) &&
			(s[:len(substr)] == substr || containsString(s[1:], substr)))
}

func findFieldIndex(t *testing.T, sp *SettingsPanel, key string) int {
	t.Helper()
	for i, field := range sp.fields {
		if field.Key == key {
			return i
		}
	}
	t.Fatalf("field %q not found", key)
	return -1
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
