package settings

import (
	"fmt"
	"strconv"
	"strings"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
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

	modelCache ai.ModelCache
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
	sp.fields = []SettingField{
		{Label: "API Key", Key: "api_key", Value: sp.config.OpenRouter.APIKey, Type: "string", Masked: true},
		{Label: "API URL", Key: "api_url", Value: sp.config.OpenRouter.APIURL, Type: "string"},
		{Label: "Model", Key: "model", Value: sp.config.OpenRouter.Model, Type: "string"},
		{Label: "Temperature", Key: "temperature", Value: fmt.Sprintf("%.1f", sp.config.OpenRouter.Temperature), Type: "float"},
		{Label: "Max Tokens", Key: "max_tokens", Value: fmt.Sprintf("%d", sp.config.OpenRouter.MaxTokens), Type: "int"},
		{Label: "API Timeout (sec)", Key: "api_timeout", Value: fmt.Sprintf("%d", sp.config.OpenRouter.APITimeoutSeconds), Type: "int"},
		{Label: "Buffer Size", Key: "buffer_size", Value: fmt.Sprintf("%d", sp.config.BufferSize), Type: "int"},
		{Label: "Context Window", Key: "context_window", Value: fmt.Sprintf("%d", sp.config.ContextWindow), Type: "int"},
		{Label: "Log Level", Key: "log_level", Value: normalizeLogLevel(sp.config.LogLevel), Type: "string"},
		{Label: "Log Format", Key: "log_format", Value: strings.ToLower(strings.TrimSpace(sp.config.LogFormat)), Type: "string"},
		{Label: "Log File", Key: "log_file", Value: sp.config.LogFile, Type: "string"},
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

// SettingsSaveMsg is sent when settings should be saved
type SettingsSaveMsg struct {
	Config     config.Config
	ConfigPath string
}

// SettingsCloseMsg is sent when settings panel closes
type SettingsCloseMsg struct{}

// Update handles keyboard input for the settings panel
func (sp *SettingsPanel) Update(msg tea.KeyPressMsg) tea.Cmd {
	// Editing mode
	if sp.editing {
		return sp.handleEditMode(msg)
	}

	keyStr := msg.String()

	// Navigation mode
	switch keyStr {
	case "up":
		if sp.selected > 0 {
			sp.selected--
		}
		return nil

	case "down":
		if sp.selected < len(sp.fields)-1 {
			sp.selected++
		}
		return nil

	case "enter":
		// Enter edit mode for current field
		field := &sp.fields[sp.selected]
		if field.Type == "info" {
			return nil
		}
		if field.Key == "model" {
			options := make([]ai.ModelInfo, len(sp.modelCache.Models))
			copy(options, sp.modelCache.Models)
			return func() tea.Msg {
				return picker.OpenModelPickerMsg{
					Options: options,
					Current: sp.config.OpenRouter.Model,
					APIURL:  sp.config.OpenRouter.APIURL,
				}
			}
		}
		if field.Key == "log_level" {
			options := logLevelOptions()
			return func() tea.Msg {
				return picker.OpenOptionPickerMsg{
					Title:    "Log Level",
					FieldKey: "log_level",
					Options:  options,
					Current:  sp.config.LogLevel,
				}
			}
		}
		if field.Key == "log_format" {
			options := []string{"json", "text"}
			return func() tea.Msg {
				return picker.OpenOptionPickerMsg{
					Title:    "Log Format",
					FieldKey: "log_format",
					Options:  options,
					Current:  sp.config.LogFormat,
				}
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
			sp.editCursor = len([]rune(sp.editValue))
		}
		return nil

	case "esc":
		// Close panel
		if sp.changed {
			// Save changes
			return sp.saveAndClose()
		}
		sp.Hide()
		return func() tea.Msg {
			return SettingsCloseMsg{}
		}

	case "s":
		if sp.changed {
			return sp.saveAndClose()
		}
		return nil

	case "e":
		field := &sp.fields[sp.selected]
		if field.Key == "model" && field.Type == "string" {
			sp.editing = true
			sp.editValue = field.Value
			sp.editCursor = len([]rune(sp.editValue))
			return nil
		}
		return nil
	}

	return nil
}

// handleEditMode handles input when editing a field
func (sp *SettingsPanel) handleEditMode(msg tea.KeyPressMsg) tea.Cmd {
	keyStr := msg.String()
	key := msg.Key()

	switch keyStr {
	case "enter":
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

	case "esc":
		// Cancel edit
		sp.editing = false
		sp.errorMsg = ""
		return nil

	case "backspace":
		runes := []rune(sp.editValue)
		if sp.editCursor > len(runes) {
			sp.editCursor = len(runes)
		}
		if sp.editCursor > 0 {
			runes = append(runes[:sp.editCursor-1], runes[sp.editCursor:]...)
			sp.editCursor--
			sp.editValue = string(runes)
		}
		return nil

	case "delete":
		runes := []rune(sp.editValue)
		if sp.editCursor > len(runes) {
			sp.editCursor = len(runes)
		}
		if sp.editCursor < len(runes) {
			runes = append(runes[:sp.editCursor], runes[sp.editCursor+1:]...)
			sp.editValue = string(runes)
		}
		return nil

	case "left":
		if sp.editCursor > 0 {
			sp.editCursor--
		}
		return nil

	case "right":
		if sp.editCursor < len([]rune(sp.editValue)) {
			sp.editCursor++
		}
		return nil

	case "home":
		sp.editCursor = 0
		return nil

	case "end":
		sp.editCursor = len([]rune(sp.editValue))
		return nil

	default:
		// Handle text input
		if key.Text != "" {
			insert := []rune(key.Text)
			// Filter out newlines
			filtered := make([]rune, 0, len(insert))
			for _, r := range insert {
				if r != '\n' && r != '\r' {
					filtered = append(filtered, r)
				}
			}
			if len(filtered) == 0 {
				return nil
			}
			runes := []rune(sp.editValue)
			if sp.editCursor > len(runes) {
				sp.editCursor = len(runes)
			}
			runes = append(runes[:sp.editCursor], append(filtered, runes[sp.editCursor:]...)...)
			sp.editCursor += len(filtered)
			sp.editValue = string(runes)
		}
		return nil
	}
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
	case "log_level":
		sp.config.LogLevel = field.Value
	case "log_format":
		sp.config.LogFormat = field.Value
	case "log_file":
		sp.config.LogFile = field.Value
	}
}

// saveAndClose saves config and closes panel
func (sp *SettingsPanel) saveAndClose() tea.Cmd {
	cfg := sp.config
	path := sp.configPath
	sp.Hide()
	return func() tea.Msg {
		return SettingsSaveMsg{Config: cfg, ConfigPath: path}
	}
}

func (sp *SettingsPanel) loadModelCache() {
	cachePath := ai.DefaultModelCachePath()
	cache, err := ai.LoadModelCache(cachePath)
	if err != nil {
		sp.modelCache = ai.ModelCache{}
		return
	}
	sp.modelCache = cache
}

// View renders the settings panel
func (sp *SettingsPanel) View() string {
	if !sp.visible {
		return ""
	}

	width := sp.width
	if width <= 0 {
		width = 80
	}
	available := width - 2
	if available < 1 {
		available = 1
	}

	boxWidth := available
	if boxWidth > 90 {
		boxWidth = 90
	}
	minWidth := 60
	if minWidth > available {
		minWidth = available
	}
	if boxWidth < minWidth {
		boxWidth = minWidth
	}

	// Styles
	boxStyle := styles.BoxStyle.Width(boxWidth)
	titleStyle := styles.TitleStyle
	labelStyle := styles.LabelStyle
	valueStyle := styles.ValueStyle
	selectedStyle := styles.SelectedStyle
	editStyle := styles.EditStyle
	errorStyle := styles.ErrorStyle
	footerStyle := styles.FooterStyle

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
			value = editStyle.Render(renderEditValue(sp.editValue, sp.editCursor))
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
		selectedKey := sp.selectedFieldKey()
		if selectedKey == "model" {
			if sp.changed {
				hint = "↑↓ Navigate • Enter: Pick • e: Edit • s: Save • Esc: Save & Close"
			} else {
				hint = "↑↓ Navigate • Enter: Pick • e: Edit • Esc: Close"
			}
		} else if selectedKey == "log_level" || selectedKey == "log_format" {
			if sp.changed {
				hint = "↑↓ Navigate • Enter: Pick • s: Save • Esc: Save & Close"
			} else {
				hint = "↑↓ Navigate • Enter: Pick • Esc: Close"
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

// SetLogLevelValue updates the log level and marks settings as changed.
func (sp *SettingsPanel) SetLogLevelValue(value string) {
	sp.setLogLevelValue(value)
	sp.changed = true
}

// SetLogFormatValue updates the log format and marks settings as changed.
func (sp *SettingsPanel) SetLogFormatValue(value string) {
	sp.setLogFormatValue(value)
	sp.changed = true
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

func (sp *SettingsPanel) setLogLevelValue(value string) {
	sp.config.LogLevel = value
	for i := range sp.fields {
		if sp.fields[i].Key == "log_level" {
			sp.fields[i].Value = value
			break
		}
	}
}

func (sp *SettingsPanel) setLogFormatValue(value string) {
	sp.config.LogFormat = value
	for i := range sp.fields {
		if sp.fields[i].Key == "log_format" {
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

// SetModelCache updates the cached model list for picker use.
func (sp *SettingsPanel) SetModelCache(cache ai.ModelCache) {
	sp.modelCache = cache
}

func renderEditValue(value string, cursor int) string {
	runes := []rune(value)
	if cursor < 0 {
		cursor = 0
	}
	if cursor > len(runes) {
		cursor = len(runes)
	}
	withCursor := make([]rune, 0, len(runes)+1)
	withCursor = append(withCursor, runes[:cursor]...)
	withCursor = append(withCursor, '█')
	withCursor = append(withCursor, runes[cursor:]...)
	return string(withCursor)
}

func normalizeLogLevel(value string) string {
	level := strings.ToLower(strings.TrimSpace(value))
	if level == "warning" {
		return "warn"
	}
	return level
}

func logLevelOptions() []string {
	return []string{"trace", "debug", "info", "warn", "error"}
}
