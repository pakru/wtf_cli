package picker

import (
	"testing"

	"wtf_cli/pkg/ui/components/testutils"

	"charm.land/lipgloss/v2"
)

func TestOptionPicker_ShowSelectCurrent(t *testing.T) {
	picker := NewOptionPickerPanel()
	picker.SetSize(80, 24)

	options := []string{"debug", "info", "warn", "error"}
	picker.Show("Log Level", "log_level", options, "warn")

	if !picker.visible {
		t.Fatal("Expected picker to be visible after Show")
	}
	if picker.title != "Log Level" {
		t.Fatalf("Expected title Log Level, got %q", picker.title)
	}
	if picker.fieldKey != "log_level" {
		t.Fatalf("Expected fieldKey log_level, got %q", picker.fieldKey)
	}
	if picker.selected != 2 {
		t.Fatalf("Expected selected=2, got %d", picker.selected)
	}
}

func TestOptionPicker_SelectEmitsMsg(t *testing.T) {
	picker := NewOptionPickerPanel()
	picker.SetSize(80, 24)

	options := []string{"json", "text"}
	picker.Show("Log Format", "log_format", options, "json")

	picker.Update(testutils.TestKeyDown)
	cmd := picker.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected optionPickerSelectMsg command")
	}
	msg := cmd()
	selectMsg, ok := msg.(OptionPickerSelectMsg)
	if !ok {
		t.Fatalf("Expected optionPickerSelectMsg, got %T", msg)
	}
	if selectMsg.FieldKey != "log_format" {
		t.Fatalf("Expected fieldKey log_format, got %q", selectMsg.FieldKey)
	}
	if selectMsg.Value != "text" {
		t.Fatalf("Expected value text, got %q", selectMsg.Value)
	}
	if picker.visible {
		t.Fatal("Expected picker to close after selection")
	}
}

func TestOptionPicker_EscCloses(t *testing.T) {
	picker := NewOptionPickerPanel()
	picker.SetSize(80, 24)

	options := []string{"debug", "info"}
	picker.Show("Log Level", "log_level", options, "debug")

	cmd := picker.Update(testutils.TestKeyEsc)
	if cmd != nil {
		t.Fatal("Expected nil command on Esc")
	}
	if picker.visible {
		t.Fatal("Expected picker to be hidden after Esc")
	}
}

func TestOptionPicker_ClampsToSmallWidth(t *testing.T) {
	picker := NewOptionPickerPanel()
	picker.SetSize(28, 8)

	options := []string{"debug", "info", "warn"}
	picker.Show("Log Level", "log_level", options, "debug")

	view := picker.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if got := lipgloss.Width(view); got > 28 {
		t.Fatalf("expected width <= 28, got %d", got)
	}
}
