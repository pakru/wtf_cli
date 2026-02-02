package picker

import (
	"fmt"
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ui/components/testutils"

	"charm.land/lipgloss/v2"
)

func TestModelPicker_ShowSelectCurrent(t *testing.T) {
	picker := NewModelPickerPanel()
	picker.SetSize(80, 24)

	options := []ai.ModelInfo{
		{ID: "model-a", Name: "Alpha"},
		{ID: "model-b", Name: "Beta"},
	}
	picker.Show(options, "model-b", "model")

	if !picker.visible {
		t.Fatal("Expected picker to be visible after Show")
	}
	if picker.filter != "" {
		t.Fatalf("Expected empty filter, got %q", picker.filter)
	}
	if picker.selected != 1 {
		t.Fatalf("Expected selected=1, got %d", picker.selected)
	}
}

func TestModelPicker_FilterAndSelect(t *testing.T) {
	picker := NewModelPickerPanel()
	picker.SetSize(80, 24)

	options := []ai.ModelInfo{
		{ID: "model-a", Name: "Alpha"},
		{ID: "model-b", Name: "Beta"},
	}
	picker.Show(options, "model-a", "model")

	picker.Update(testutils.NewTextKeyPressMsg("b"))
	picker.Update(testutils.NewTextKeyPressMsg("e"))
	if picker.filter != "be" {
		t.Fatalf("Expected filter 'be', got %q", picker.filter)
	}
	if picker.selected != 0 {
		t.Fatalf("Expected selected reset to 0, got %d", picker.selected)
	}

	filtered := picker.filteredOptions()
	if len(filtered) != 1 || filtered[0].ID != "model-b" {
		t.Fatalf("Expected filtered model-b, got %+v", filtered)
	}

	picker.Update(testutils.TestKeyBackspace)
	if picker.filter != "b" {
		t.Fatalf("Expected filter 'b' after backspace, got %q", picker.filter)
	}

	cmd := picker.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected modelPickerSelectMsg command")
	}
	msg := cmd()
	selectMsg, ok := msg.(ModelPickerSelectMsg)
	if !ok {
		t.Fatalf("Expected modelPickerSelectMsg, got %T", msg)
	}
	if selectMsg.ModelID != "model-b" {
		t.Fatalf("Expected model-b, got %q", selectMsg.ModelID)
	}
	if picker.visible {
		t.Fatal("Expected picker to close after selection")
	}
}

func TestModelPicker_ScrollsWithNavigation(t *testing.T) {
	picker := NewModelPickerPanel()
	picker.SetSize(80, 10)

	options := make([]ai.ModelInfo, 5)
	for i := range options {
		options[i] = ai.ModelInfo{
			ID:   fmt.Sprintf("model-%d", i),
			Name: fmt.Sprintf("Model %d", i),
		}
	}
	picker.Show(options, "", "model")

	if picker.listHeight() != 1 {
		t.Fatalf("Expected listHeight=1, got %d", picker.listHeight())
	}
	if picker.scroll != 0 {
		t.Fatalf("Expected scroll=0, got %d", picker.scroll)
	}

	for i := 1; i < len(options); i++ {
		picker.Update(testutils.TestKeyDown)
		if picker.selected != i {
			t.Fatalf("Expected selected=%d, got %d", i, picker.selected)
		}
		if picker.scroll != i {
			t.Fatalf("Expected scroll=%d, got %d", i, picker.scroll)
		}
	}
}

func TestModelPicker_EscCloses(t *testing.T) {
	picker := NewModelPickerPanel()
	picker.SetSize(80, 24)

	options := []ai.ModelInfo{
		{ID: "model-a", Name: "Alpha"},
	}
	picker.Show(options, "model-a", "model")

	cmd := picker.Update(testutils.TestKeyEsc)
	if cmd != nil {
		t.Fatal("Expected nil command on Esc")
	}
	if picker.visible {
		t.Fatal("Expected picker to be hidden after Esc")
	}
}

func TestModelPicker_ClampsToSmallWidth(t *testing.T) {
	picker := NewModelPickerPanel()
	picker.SetSize(30, 10)

	options := []ai.ModelInfo{
		{ID: "model-a", Name: "Alpha"},
		{ID: "model-b", Name: "Beta"},
	}
	picker.Show(options, "model-a", "model")

	view := picker.View()
	if view == "" {
		t.Fatal("expected non-empty view")
	}
	if got := lipgloss.Width(view); got > 30 {
		t.Fatalf("expected width <= 30, got %d", got)
	}
}
