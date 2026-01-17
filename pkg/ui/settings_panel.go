package ui

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SettingField represents a single editable setting
type SettingField struct {
	Label  string
	Key    string
	Value  string
	Type   string // "string", "int", "float", "bool"
	Masked bool   // For sensitive fields like API key
}

// SettingsPanel displays and edits configuration
type SettingsPanel struct {
	config     config.Config
	configPath string
	fields     []SettingField
	selected   int
	editing    bool
	editValue  string
	editCursor int
	changed    bool
	width      int
	height     int
	visible    bool
	errorMsg   string

	modelCache    ai.ModelCache
	modelCacheErr error
}

// NewSettingsPanel creates a new settings panel
func NewSettingsPanel() *SettingsPanel {
	return &SettingsPanel{}
}

// Show displays the settings panel with the given config
func (sp *SettingsPanel) Show(cfg config.Config, configPath string) {
	sp.config = cfg
	sp.configPath = configPath
	sp.visible = true
	sp.selected = 0
	sp.editing = false
	sp.changed = false
	sp.errorMsg = ""
	sp.loadModelCache()
	sp.buildFields()
}

// buildFields creates the field list from config
func (sp *SettingsPanel) buildFields() {
	modelCacheSummary := sp.modelCacheSummary()

	sp.fields = []SettingField{
		{Label: "API Key", Key: "api_key", Value: sp.config.OpenRouter.APIKey, Type: "string", Masked: true},
		{Label: "API URL", Key: "api_url", Value: sp.config.OpenRouter.APIURL, Type: "string"},
		{Label: "Model", Key: "model", Value: sp.config.OpenRouter.Model, Type: "string"},
		{Label: "Models Cache", Key: "models_cache", Value: modelCacheSummary, Type: "info"},
		{Label: "Temperature", Key: "temperature", Value: fmt.Sprintf("%.1f", sp.config.OpenRouter.Temperature), Type: "float"},
		{Label: "Max Tokens", Key: "max_tokens", Value: strconv.Itoa(sp.config.OpenRouter.MaxTokens), Type: "int"},
		{Label: "API Timeout (sec)", Key: "api_timeout", Value: strconv.Itoa(sp.config.OpenRouter.APITimeoutSeconds), Type: "int"},
		{Label: "Buffer Size", Key: "buffer_size", Value: strconv.Itoa(sp.config.BufferSize), Type: "int"},
		{Label: "Context Window", Key: "context_window", Value: strconv.Itoa(sp.config.ContextWindow), Type: "int"},
		{Label: "Dry Run", Key: "dry_run", Value: strconv.FormatBool(sp.config.DryRun), Type: "bool"},
	}
}

// Hide hides the settings panel
func (sp *SettingsPanel) Hide() {
	sp.visible = false
	sp.editing = false
}

// IsVisible returns whether the panel is visible
func (sp *SettingsPanel) IsVisible() bool {
	return sp.visible
}

// SetSize sets the panel dimensions
func (sp *SettingsPanel) SetSize(width, height int) {
	sp.width = width
	sp.height = height
}

// HasChanges returns whether settings have been modified
func (sp *SettingsPanel) HasChanges() bool {
	return sp.changed
}

// settingsSaveMsg is sent when settings should be saved
type settingsSaveMsg struct {
	config     config.Config
	configPath string
}

// settingsCloseMsg is sent when settings panel closes
type settingsCloseMsg struct{}

// Update handles keyboard input for the settings panel
func (sp *SettingsPanel) Update(msg tea.KeyMsg) tea.Cmd {
	// Editing mode
	if sp.editing {
		return sp.handleEditMode(msg)
	}

	// Navigation mode
	switch msg.Type {
	case tea.KeyUp:
		if sp.selected > 0 {
			sp.selected--
		}
		return nil

	case tea.KeyDown:
		if sp.selected < len(sp.fields)-1 {
			sp.selected++
		}
		return nil

	case tea.KeyEnter:
		// Enter edit mode for current field
		field := &sp.fields[sp.selected]
		if field.Type == "info" {
			return nil
		}
		if field.Key == "model" && sp.hasModelOptions() {
			options := make([]ai.ModelInfo, len(sp.modelCache.Models))
			copy(options, sp.modelCache.Models)
			current := sp.config.OpenRouter.Model
			return func() tea.Msg {
				return openModelPickerMsg{options: options, current: current}
			}
		}
		if field.Type == "bool" {
			// Toggle bool directly
			if field.Value == "true" {
				field.Value = "false"
			} else {
				field.Value = "true"
			}
			sp.changed = true
			sp.applyField(field)
		} else {
			sp.editing = true
			sp.editValue = field.Value
			sp.editCursor = len(sp.editValue)
		}
		return nil

	case tea.KeyEsc:
		// Close panel
		if sp.changed {
			// Save changes
			return sp.saveAndClose()
		}
		sp.Hide()
		return func() tea.Msg {
			return settingsCloseMsg{}
		}
	}

	// 's' to save
	if msg.String() == "s" && sp.changed {
		return sp.saveAndClose()
	}

	if msg.String() == "e" {
		field := &sp.fields[sp.selected]
		if field.Key == "model" && field.Type == "string" {
			sp.editing = true
			sp.editValue = field.Value
			sp.editCursor = len(sp.editValue)
			return nil
		}
	}

	return nil
}

// handleEditMode handles input when editing a field
func (sp *SettingsPanel) handleEditMode(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEnter:
		// Apply edit
		field := &sp.fields[sp.selected]
		if sp.validateValue(field.Type, sp.editValue) {
			field.Value = sp.editValue
			sp.changed = true
			sp.applyField(field)
			sp.errorMsg = ""
		} else {
			sp.errorMsg = "Invalid value for " + field.Label
		}
		sp.editing = false
		return nil

	case tea.KeyEsc:
		// Cancel edit
		sp.editing = false
		sp.errorMsg = ""
		return nil

	case tea.KeyBackspace:
		if len(sp.editValue) > 0 {
			sp.editValue = sp.editValue[:len(sp.editValue)-1]
		}
		return nil

	case tea.KeyRunes:
		sp.editValue += msg.String()
		return nil
	}

	return nil
}

// validateValue checks if a value is valid for its type
func (sp *SettingsPanel) validateValue(fieldType, value string) bool {
	switch fieldType {
	case "int":
		_, err := strconv.Atoi(value)
		return err == nil
	case "float":
		_, err := strconv.ParseFloat(value, 64)
		return err == nil
	case "bool":
		return value == "true" || value == "false"
	default:
		return true // strings are always valid
	}
}

// applyField updates the config with the field value
func (sp *SettingsPanel) applyField(field *SettingField) {
	switch field.Key {
	case "api_key":
		sp.config.OpenRouter.APIKey = field.Value
	case "api_url":
		sp.config.OpenRouter.APIURL = field.Value
	case "model":
		sp.config.OpenRouter.Model = field.Value
	case "temperature":
		if v, err := strconv.ParseFloat(field.Value, 64); err == nil {
			sp.config.OpenRouter.Temperature = v
		}
	case "max_tokens":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.OpenRouter.MaxTokens = v
		}
	case "api_timeout":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.OpenRouter.APITimeoutSeconds = v
		}
	case "buffer_size":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.BufferSize = v
		}
	case "context_window":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.ContextWindow = v
		}
	case "dry_run":
		sp.config.DryRun = field.Value == "true"
	}
}

// saveAndClose saves config and closes panel
func (sp *SettingsPanel) saveAndClose() tea.Cmd {
	cfg := sp.config
	path := sp.configPath
	sp.Hide()
	return func() tea.Msg {
		return settingsSaveMsg{config: cfg, configPath: path}
	}
}

func (sp *SettingsPanel) loadModelCache() {
	cachePath := ai.DefaultModelCachePath()
	cache, err := ai.LoadModelCache(cachePath)
	if err != nil {
		sp.modelCache = ai.ModelCache{}
		sp.modelCacheErr = err
		return
	}
	sp.modelCache = cache
	sp.modelCacheErr = nil
}

func (sp *SettingsPanel) modelCacheSummary() string {
	if sp.modelCacheErr != nil {
		if errors.Is(sp.modelCacheErr, os.ErrNotExist) {
			return "Not cached (run /models)"
		}
		return "Cache read error"
	}

	if len(sp.modelCache.Models) == 0 {
		return "Empty cache (run /models)"
	}

	if sp.modelCache.UpdatedAt.IsZero() {
		return fmt.Sprintf("%d models (timestamp unknown)", len(sp.modelCache.Models))
	}

	return fmt.Sprintf("%d models, updated %s", len(sp.modelCache.Models), sp.modelCache.UpdatedAt.Local().Format("2006-01-02 15:04"))
}

// View renders the settings panel
func (sp *SettingsPanel) View() string {
	if !sp.visible {
		return ""
	}

	boxWidth := sp.width - 2
	if boxWidth > 90 {
		boxWidth = 90
	}
	if boxWidth < 60 {
		boxWidth = 60
	}

	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("141")).
		Padding(1, 2).
		Width(boxWidth)

	titleStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("141")).
		Bold(true)

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	selectedStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("15")).
		Background(lipgloss.Color("141")).
		Bold(true)

	editStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	footerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	// Build content
	var content strings.Builder

	content.WriteString(titleStyle.Render("⚙️  Settings"))
	content.WriteString("\n\n")

	// Render fields
	for i, field := range sp.fields {
		label := labelStyle.Render(field.Label + ":")

		var value string
		if sp.editing && i == sp.selected {
			// Show edit cursor
			value = editStyle.Render(sp.editValue + "█")
		} else if field.Masked && field.Value != "" {
			// Mask sensitive values
			value = strings.Repeat("•", len(field.Value))
		} else {
			value = field.Value
		}

		var line string
		if i == sp.selected {
			if sp.editing {
				line = "▶ " + label + " " + value
			} else {
				labelText := fmt.Sprintf("%-20s", field.Label+":")
				line = selectedStyle.Render("  " + labelText + " " + value + " ")
			}
		} else {
			line = "  " + label + " " + valueStyle.Render(value)
		}
		content.WriteString(line + "\n")
	}

	// Error message
	if sp.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("⚠️  " + sp.errorMsg))
	}

	// Footer
	content.WriteString("\n\n")
	if sp.editing {
		content.WriteString(footerStyle.Render("Enter: Confirm • Esc: Cancel"))
	} else {
		hint := "↑↓ Navigate • Enter: Edit • Esc: Close"
		if sp.changed {
			hint = "↑↓ Navigate • Enter: Edit • s: Save • Esc: Save & Close"
		}
		if sp.selectedFieldKey() == "model" && sp.hasModelOptions() {
			if sp.changed {
				hint = "↑↓ Navigate • Enter: Pick • e: Edit • s: Save • Esc: Save & Close"
			} else {
				hint = "↑↓ Navigate • Enter: Pick • e: Edit • Esc: Close"
			}
		}
		content.WriteString(footerStyle.Render(hint))
	}

	return boxStyle.Render(content.String())
}

// GetConfig returns the current config
func (sp *SettingsPanel) GetConfig() config.Config {
	return sp.config
}

// SetModelValue updates the selected model and marks settings as changed.
func (sp *SettingsPanel) SetModelValue(value string) {
	sp.setModelValue(value)
	sp.changed = true
}

func (sp *SettingsPanel) hasModelOptions() bool {
	return len(sp.modelCache.Models) > 0
}

func (sp *SettingsPanel) setModelValue(value string) {
	sp.config.OpenRouter.Model = value
	for i := range sp.fields {
		if sp.fields[i].Key == "model" {
			sp.fields[i].Value = value
			break
		}
	}
}

func (sp *SettingsPanel) selectedFieldKey() string {
	if sp.selected < 0 || sp.selected >= len(sp.fields) {
		return ""
	}
	return sp.fields[sp.selected].Key
}
