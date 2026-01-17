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

	modelPickerVisible bool
	modelPickerIndex   int
	modelPickerScroll  int
	modelPickerFilter  string
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
	sp.modelPickerVisible = false
	sp.modelPickerIndex = 0
	sp.modelPickerScroll = 0
	sp.modelPickerFilter = ""
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
	sp.modelPickerVisible = false
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
	if sp.modelPickerVisible {
		return sp.handleModelPicker(msg)
	}

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
			sp.openModelPicker()
			return nil
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

func (sp *SettingsPanel) handleModelPicker(msg tea.KeyMsg) tea.Cmd {
	switch msg.Type {
	case tea.KeyEsc:
		sp.closeModelPicker()
		return nil

	case tea.KeyEnter:
		options := sp.filteredModelOptions()
		if len(options) == 0 {
			return nil
		}
		if sp.modelPickerIndex >= len(options) {
			sp.modelPickerIndex = len(options) - 1
		}
		if sp.modelPickerIndex < 0 {
			sp.modelPickerIndex = 0
		}
		modelID := options[sp.modelPickerIndex].ID
		if modelID != "" {
			sp.setModelValue(modelID)
			sp.changed = true
		}
		sp.closeModelPicker()
		return nil

	case tea.KeyUp:
		if sp.modelPickerIndex > 0 {
			sp.modelPickerIndex--
		}
		sp.ensureModelPickerVisible()
		return nil

	case tea.KeyDown:
		options := sp.filteredModelOptions()
		if sp.modelPickerIndex < len(options)-1 {
			sp.modelPickerIndex++
		}
		sp.ensureModelPickerVisible()
		return nil

	case tea.KeyPgUp:
		if len(sp.filteredModelOptions()) == 0 {
			return nil
		}
		sp.modelPickerIndex -= 10
		if sp.modelPickerIndex < 0 {
			sp.modelPickerIndex = 0
		}
		sp.ensureModelPickerVisible()
		return nil

	case tea.KeyPgDown:
		options := sp.filteredModelOptions()
		if len(options) == 0 {
			return nil
		}
		sp.modelPickerIndex += 10
		if sp.modelPickerIndex >= len(options) {
			sp.modelPickerIndex = len(options) - 1
		}
		sp.ensureModelPickerVisible()
		return nil

	case tea.KeyBackspace:
		if len(sp.modelPickerFilter) > 0 {
			sp.modelPickerFilter = sp.modelPickerFilter[:len(sp.modelPickerFilter)-1]
			sp.resetModelPickerSelection()
		}
		return nil

	case tea.KeyRunes:
		sp.modelPickerFilter += msg.String()
		sp.resetModelPickerSelection()
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

	normalStyle := lipgloss.NewStyle().
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

	descStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("245")).
		Italic(true)

	filterStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

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

	if sp.modelPickerVisible {
		content.WriteString("\n")
		content.WriteString(titleStyle.Render("Model Picker"))
		content.WriteString("\n")
		if sp.modelPickerFilter != "" {
			content.WriteString(filterStyle.Render("Filter: " + sp.modelPickerFilter))
			content.WriteString("\n")
		}
		content.WriteString("\n")

		options := sp.filteredModelOptions()
		if len(options) == 0 {
			content.WriteString(descStyle.Render("No matching models"))
		} else {
			visibleLines := sp.modelPickerVisibleLines()
			start := sp.modelPickerScroll
			end := start + visibleLines
			if end > len(options) {
				end = len(options)
			}
			for i := start; i < end; i++ {
				line := sp.modelOptionLabel(options[i])
				if i == sp.modelPickerIndex {
					content.WriteString(selectedStyle.Render("  " + line + "  "))
				} else {
					content.WriteString(normalStyle.Render("  " + line + "  "))
				}
				content.WriteString("\n")
			}
		}
	}

	// Error message
	if sp.errorMsg != "" {
		content.WriteString("\n")
		content.WriteString(errorStyle.Render("⚠️  " + sp.errorMsg))
	}

	// Footer
	content.WriteString("\n\n")
	if sp.modelPickerVisible {
		content.WriteString(footerStyle.Render("↑↓ Navigate • Enter: Apply • Esc: Cancel • Type to filter"))
	} else if sp.editing {
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

func (sp *SettingsPanel) hasModelOptions() bool {
	return len(sp.modelCache.Models) > 0
}

func (sp *SettingsPanel) filteredModelOptions() []ai.ModelInfo {
	if !sp.hasModelOptions() {
		return nil
	}
	filter := strings.ToLower(strings.TrimSpace(sp.modelPickerFilter))
	if filter == "" {
		return sp.modelCache.Models
	}
	filtered := make([]ai.ModelInfo, 0, len(sp.modelCache.Models))
	for _, model := range sp.modelCache.Models {
		id := strings.ToLower(model.ID)
		name := strings.ToLower(model.Name)
		if strings.Contains(id, filter) || strings.Contains(name, filter) {
			filtered = append(filtered, model)
		}
	}
	return filtered
}

func (sp *SettingsPanel) openModelPicker() {
	sp.modelPickerVisible = true
	sp.modelPickerFilter = ""
	sp.modelPickerIndex = 0
	sp.modelPickerScroll = 0

	options := sp.filteredModelOptions()
	if len(options) == 0 {
		return
	}
	current := strings.TrimSpace(sp.config.OpenRouter.Model)
	for i, model := range options {
		if model.ID == current {
			sp.modelPickerIndex = i
			break
		}
	}
	sp.ensureModelPickerVisible()
}

func (sp *SettingsPanel) closeModelPicker() {
	sp.modelPickerVisible = false
	sp.modelPickerFilter = ""
	sp.modelPickerIndex = 0
	sp.modelPickerScroll = 0
}

func (sp *SettingsPanel) resetModelPickerSelection() {
	sp.modelPickerIndex = 0
	sp.modelPickerScroll = 0
}

func (sp *SettingsPanel) ensureModelPickerVisible() {
	visibleLines := sp.modelPickerVisibleLines()
	if sp.modelPickerIndex < sp.modelPickerScroll {
		sp.modelPickerScroll = sp.modelPickerIndex
	}
	if sp.modelPickerIndex >= sp.modelPickerScroll+visibleLines {
		sp.modelPickerScroll = sp.modelPickerIndex - visibleLines + 1
	}
	if sp.modelPickerScroll < 0 {
		sp.modelPickerScroll = 0
	}
}

func (sp *SettingsPanel) modelPickerVisibleLines() int {
	if sp.height <= 0 {
		return 6
	}
	available := sp.height - (len(sp.fields) + 10)
	if available < 3 {
		return 3
	}
	return available
}

func (sp *SettingsPanel) modelOptionLabel(model ai.ModelInfo) string {
	name := strings.TrimSpace(model.Name)
	if name != "" && name != model.ID {
		return model.ID + " - " + name
	}
	return model.ID
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
