package settings

import (
	"fmt"
	"strconv"
	"strings"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/auth"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

	copilotAuthMessage string
	copilotAuthOpen    bool
	copilotAuthSummary string
	copilotAuthDetail  string
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
	sp.resetCopilotAuthStatus()
	sp.buildFields()
}

// buildFields creates the field list from config
func (sp *SettingsPanel) buildFields() {
	sp.fields = []SettingField{
		{Label: "LLM Provider", Key: "llm_provider", Value: sp.config.LLMProvider, Type: "string"},
		{Label: "Status", Key: "provider_status", Value: sp.getSelectedProviderStatus(), Type: "info"},
	}

	// Add provider-specific fields based on selected provider
	switch sp.config.LLMProvider {
	case "openrouter":
		sp.fields = append(sp.fields,
			SettingField{Label: "API Key", Key: "api_key", Value: sp.config.OpenRouter.APIKey, Type: "string", Masked: true},
			SettingField{Label: "API URL", Key: "api_url", Value: sp.config.OpenRouter.APIURL, Type: "string"},
			SettingField{Label: "Model", Key: "model", Value: sp.config.OpenRouter.Model, Type: "string"},
			SettingField{Label: "Temperature", Key: "temperature", Value: fmt.Sprintf("%.1f", sp.config.OpenRouter.Temperature), Type: "float"},
			SettingField{Label: "Max Tokens", Key: "max_tokens", Value: fmt.Sprintf("%d", sp.config.OpenRouter.MaxTokens), Type: "int"},
			SettingField{Label: "API Timeout (sec)", Key: "api_timeout", Value: fmt.Sprintf("%d", sp.config.OpenRouter.APITimeoutSeconds), Type: "int"},
		)
	case "openai":
		sp.fields = append(sp.fields,
			SettingField{Label: "API Key", Key: "openai_api_key", Value: sp.config.Providers.OpenAI.APIKey, Type: "string", Masked: true},
			SettingField{Label: "Model", Key: "openai_model", Value: sp.getOpenAIModel(), Type: "string"},
			SettingField{Label: "Temperature", Key: "openai_temperature", Value: fmt.Sprintf("%.1f", sp.config.Providers.OpenAI.Temperature), Type: "float"},
			SettingField{Label: "Max Tokens", Key: "openai_max_tokens", Value: fmt.Sprintf("%d", sp.config.Providers.OpenAI.MaxTokens), Type: "int"},
		)
	case "copilot":
		sp.fields = append(sp.fields,
			SettingField{Label: "Auth Status", Key: "copilot_auth", Value: sp.getCopilotAuthStatus(), Type: "info"},
			SettingField{Label: "Model", Key: "copilot_model", Value: sp.getCopilotModel(), Type: "string"},
			SettingField{Label: "Temperature", Key: "copilot_temperature", Value: fmt.Sprintf("%.1f", sp.config.Providers.Copilot.Temperature), Type: "float"},
			SettingField{Label: "Max Tokens", Key: "copilot_max_tokens", Value: fmt.Sprintf("%d", sp.config.Providers.Copilot.MaxTokens), Type: "int"},
		)
	case "anthropic":
		sp.fields = append(sp.fields,
			SettingField{Label: "API Key", Key: "anthropic_api_key", Value: sp.config.Providers.Anthropic.APIKey, Type: "string", Masked: true},
			SettingField{Label: "Model", Key: "anthropic_model", Value: sp.getAnthropicModel(), Type: "string"},
			SettingField{Label: "Temperature", Key: "anthropic_temperature", Value: fmt.Sprintf("%.1f", sp.config.Providers.Anthropic.Temperature), Type: "float"},
			SettingField{Label: "Max Tokens", Key: "anthropic_max_tokens", Value: fmt.Sprintf("%d", sp.config.Providers.Anthropic.MaxTokens), Type: "int"},
		)
	case "google":
		sp.fields = append(sp.fields,
			SettingField{Label: "API Key", Key: "google_api_key", Value: sp.config.Providers.Google.APIKey, Type: "string", Masked: true},
			SettingField{Label: "Model", Key: "google_model", Value: sp.getGoogleModel(), Type: "string"},
			SettingField{Label: "Temperature", Key: "google_temperature", Value: fmt.Sprintf("%.1f", sp.config.Providers.Google.Temperature), Type: "float"},
			SettingField{Label: "Max Tokens", Key: "google_max_tokens", Value: fmt.Sprintf("%d", sp.config.Providers.Google.MaxTokens), Type: "int"},
		)
	}

	// Common fields
	sp.fields = append(sp.fields,
		SettingField{Label: "Buffer Size", Key: "buffer_size", Value: fmt.Sprintf("%d", sp.config.BufferSize), Type: "int"},
		SettingField{Label: "Context Window", Key: "context_window", Value: fmt.Sprintf("%d", sp.config.ContextWindow), Type: "int"},
		SettingField{Label: "Log Level", Key: "log_level", Value: normalizeLogLevel(sp.config.LogLevel), Type: "string"},
		SettingField{Label: "Log Format", Key: "log_format", Value: strings.ToLower(strings.TrimSpace(sp.config.LogFormat)), Type: "string"},
		SettingField{Label: "Log File", Key: "log_file", Value: sp.config.LogFile, Type: "string"},
	)
}

func (sp *SettingsPanel) getOpenAIModel() string {
	if sp.config.Providers.OpenAI.Model != "" {
		return sp.config.Providers.OpenAI.Model
	}
	return "gpt-4o"
}

func (sp *SettingsPanel) getCopilotModel() string {
	if sp.config.Providers.Copilot.Model != "" {
		return sp.config.Providers.Copilot.Model
	}
	return "gpt-4o"
}

func (sp *SettingsPanel) getAnthropicModel() string {
	if sp.config.Providers.Anthropic.Model != "" {
		return sp.config.Providers.Anthropic.Model
	}
	return "claude-3-5-sonnet-20241022"
}

func (sp *SettingsPanel) getGoogleModel() string {
	if sp.config.Providers.Google.Model != "" {
		return sp.config.Providers.Google.Model
	}
	return "gemini-3-flash-preview"
}

func (sp *SettingsPanel) getCopilotAuthStatus() string {
	if strings.TrimSpace(sp.copilotAuthDetail) != "" {
		return sp.copilotAuthDetail
	}
	return "Not checked (Enter to refresh)"
}

func (sp *SettingsPanel) getCopilotStatus() string {
	if strings.TrimSpace(sp.copilotAuthSummary) != "" {
		return sp.copilotAuthSummary
	}
	return "Not checked"
}

func (sp *SettingsPanel) getOpenAIStatus() string {
	if strings.TrimSpace(sp.config.Providers.OpenAI.APIKey) != "" {
		return "✅ API key set"
	}
	authMgr := auth.NewAuthManager(auth.DefaultAuthPath())
	if authMgr.HasCredentials("openai") {
		creds, err := authMgr.Load("openai")
		if err == nil && !creds.IsExpired() {
			return "✅ OAuth connected"
		}
		if err == nil && creds.IsExpired() {
			return "❌ OAuth expired"
		}
	}
	return "❌ Not connected"
}

func (sp *SettingsPanel) getOpenRouterStatus() string {
	if strings.TrimSpace(sp.config.OpenRouter.APIKey) != "" {
		return "✅ Ready"
	}
	return "❌ Missing API key"
}

func (sp *SettingsPanel) getAnthropicStatus() string {
	if strings.TrimSpace(sp.config.Providers.Anthropic.APIKey) != "" {
		return "✅ Ready"
	}
	return "❌ Missing API key"
}

func (sp *SettingsPanel) getGoogleStatus() string {
	if strings.TrimSpace(sp.config.Providers.Google.APIKey) != "" {
		return "✅ Ready"
	}
	return "❌ Missing API key"
}

func (sp *SettingsPanel) getSelectedProviderStatus() string {
	switch sp.config.LLMProvider {
	case "openai":
		return sp.getOpenAIStatus()
	case "copilot":
		return sp.getCopilotStatus()
	case "anthropic":
		return sp.getAnthropicStatus()
	case "google":
		return sp.getGoogleStatus()
	default:
		return sp.getOpenRouterStatus()
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

// StartCopilotAuthMsg is sent when user wants to authenticate with GitHub Copilot
type StartCopilotAuthMsg struct{}

// ProviderChangedMsg is sent when the LLM provider is changed
type ProviderChangedMsg struct {
	Provider string
}

// Update handles keyboard input for the settings panel
func (sp *SettingsPanel) Update(msg tea.KeyPressMsg) tea.Cmd {
	// Editing mode
	if sp.editing {
		return sp.handleEditMode(msg)
	}

	keyStr := msg.String()

	// Modal mode: Copilot auth prompt
	if sp.copilotAuthOpen {
		switch keyStr {
		case "enter", "esc":
			sp.ClearCopilotAuthMessage()
		}
		return nil
	}

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
			// Handle special info fields like copilot auth
			if field.Key == "copilot_auth" {
				return func() tea.Msg {
					return StartCopilotAuthMsg{}
				}
			}
			return nil
		}
		if field.Key == "llm_provider" {
			options := config.SupportedProviders()
			return func() tea.Msg {
				return picker.OpenOptionPickerMsg{
					Title:    "LLM Provider",
					FieldKey: "llm_provider",
					Options:  options,
					Current:  sp.config.LLMProvider,
				}
			}
		}
		if field.Key == "model" {
			options := make([]ai.ModelInfo, len(sp.modelCache.Models))
			copy(options, sp.modelCache.Models)
			return func() tea.Msg {
				return picker.OpenModelPickerMsg{
					Options:  options,
					Current:  sp.config.OpenRouter.Model,
					APIURL:   sp.config.OpenRouter.APIURL,
					FieldKey: "model",
				}
			}
		}
		if field.Key == "openai_model" {
			options := ai.GetProviderModels("openai")
			apiKey := sp.config.Providers.OpenAI.APIKey
			return func() tea.Msg {
				return picker.OpenModelPickerMsg{
					Options:  options,
					Current:  sp.config.Providers.OpenAI.Model,
					FieldKey: "openai_model",
					APIKey:   apiKey,
				}
			}
		}
		if field.Key == "copilot_model" {
			options := ai.GetCopilotModels()
			return func() tea.Msg {
				return picker.OpenModelPickerMsg{
					Options:  options,
					Current:  sp.config.Providers.Copilot.Model,
					FieldKey: "copilot_model",
				}
			}
		}
		if field.Key == "anthropic_model" {
			options := ai.GetProviderModels("anthropic")
			apiKey := sp.config.Providers.Anthropic.APIKey
			return func() tea.Msg {
				return picker.OpenModelPickerMsg{
					Options:  options,
					Current:  sp.config.Providers.Anthropic.Model,
					FieldKey: "anthropic_model",
					APIKey:   apiKey,
				}
			}
		}
		if field.Key == "google_model" {
			options := ai.GetProviderModels("google")
			apiKey := sp.config.Providers.Google.APIKey
			return func() tea.Msg {
				return picker.OpenModelPickerMsg{
					Options:  options,
					Current:  sp.config.Providers.Google.Model,
					FieldKey: "google_model",
					APIKey:   apiKey,
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
	// OpenRouter fields
	case "api_key":
		sp.config.OpenRouter.APIKey = field.Value
		sp.refreshProviderStatusFields()
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

	// OpenAI fields
	case "openai_api_key":
		sp.config.Providers.OpenAI.APIKey = field.Value
		sp.refreshProviderStatusFields()
	case "openai_model":
		sp.config.Providers.OpenAI.Model = field.Value
	case "openai_temperature":
		if v, err := strconv.ParseFloat(field.Value, 64); err == nil {
			sp.config.Providers.OpenAI.Temperature = v
		}
	case "openai_max_tokens":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.Providers.OpenAI.MaxTokens = v
		}

	// Copilot fields
	case "copilot_model":
		sp.config.Providers.Copilot.Model = field.Value
	case "copilot_temperature":
		if v, err := strconv.ParseFloat(field.Value, 64); err == nil {
			sp.config.Providers.Copilot.Temperature = v
		}
	case "copilot_max_tokens":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.Providers.Copilot.MaxTokens = v
		}

	// Anthropic fields
	case "anthropic_api_key":
		sp.config.Providers.Anthropic.APIKey = field.Value
		sp.refreshProviderStatusFields()
	case "anthropic_model":
		sp.config.Providers.Anthropic.Model = field.Value
	case "anthropic_temperature":
		if v, err := strconv.ParseFloat(field.Value, 64); err == nil {
			sp.config.Providers.Anthropic.Temperature = v
		}
	case "anthropic_max_tokens":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.Providers.Anthropic.MaxTokens = v
		}

	// Google fields
	case "google_api_key":
		sp.config.Providers.Google.APIKey = field.Value
		sp.refreshProviderStatusFields()
	case "google_model":
		sp.config.Providers.Google.Model = field.Value
	case "google_temperature":
		if v, err := strconv.ParseFloat(field.Value, 64); err == nil {
			sp.config.Providers.Google.Temperature = v
		}
	case "google_max_tokens":
		if v, err := strconv.Atoi(field.Value); err == nil {
			sp.config.Providers.Google.MaxTokens = v
		}

	// Common fields
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
	} else if sp.copilotAuthOpen {
		content.WriteString(footerStyle.Render("Enter: OK • Esc: Close"))
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
		} else if selectedKey == "openai_model" || selectedKey == "copilot_model" || selectedKey == "anthropic_model" || selectedKey == "google_model" {
			if sp.changed {
				hint = "↑↓ Navigate • Enter: Pick • s: Save • Esc: Save & Close"
			} else {
				hint = "↑↓ Navigate • Enter: Pick • Esc: Close"
			}
		} else if selectedKey == "llm_provider" || selectedKey == "log_level" || selectedKey == "log_format" {
			if sp.changed {
				hint = "↑↓ Navigate • Enter: Pick • s: Save • Esc: Save & Close"
			} else {
				hint = "↑↓ Navigate • Enter: Pick • Esc: Close"
			}
		} else if selectedKey == "copilot_auth" {
			if sp.changed {
				hint = "↑↓ Navigate • Enter: Details • s: Save • Esc: Save & Close"
			} else {
				hint = "↑↓ Navigate • Enter: Details • Esc: Close"
			}
		}
		content.WriteString(footerStyle.Render(hint))
	}

	panel := boxStyle.Render(content.String())
	if sp.copilotAuthOpen {
		panelWidth := lipgloss.Width(panel)
		authBox := sp.renderCopilotAuthBox(panelWidth - 6)
		if authBox != "" {
			panel = panel + "\n\n" + lipgloss.PlaceHorizontal(panelWidth, lipgloss.Center, authBox)
		}
	}

	return panel
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

// SetProviderValue updates the LLM provider and rebuilds fields.
func (sp *SettingsPanel) SetProviderValue(value string) {
	sp.config.LLMProvider = value
	sp.changed = true
	if value == "copilot" {
		sp.resetCopilotAuthStatus()
	} else {
		sp.clearCopilotAuthPrompt()
	}
	sp.buildFields()
}

// SetOpenAIModelValue updates the OpenAI model field.
func (sp *SettingsPanel) SetOpenAIModelValue(value string) {
	sp.config.Providers.OpenAI.Model = value
	sp.changed = true
	for i := range sp.fields {
		if sp.fields[i].Key == "openai_model" {
			sp.fields[i].Value = value
			break
		}
	}
}

// SetCopilotModelValue updates the Copilot model field.
func (sp *SettingsPanel) SetCopilotModelValue(value string) {
	sp.config.Providers.Copilot.Model = value
	sp.changed = true
	for i := range sp.fields {
		if sp.fields[i].Key == "copilot_model" {
			sp.fields[i].Value = value
			break
		}
	}
}

// SetAnthropicModelValue updates the Anthropic model field.
func (sp *SettingsPanel) SetAnthropicModelValue(value string) {
	sp.config.Providers.Anthropic.Model = value
	sp.changed = true
	for i := range sp.fields {
		if sp.fields[i].Key == "anthropic_model" {
			sp.fields[i].Value = value
			break
		}
	}
}

// SetGoogleModelValue updates the Google model field.
func (sp *SettingsPanel) SetGoogleModelValue(value string) {
	sp.config.Providers.Google.Model = value
	sp.changed = true
	for i := range sp.fields {
		if sp.fields[i].Key == "google_model" {
			sp.fields[i].Value = value
			break
		}
	}
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

// RefreshCopilotAuthStatus updates only the Copilot auth status field
// without resetting other panel state or discarding unsaved edits.
func (sp *SettingsPanel) RefreshCopilotAuthStatus() {
	sp.refreshProviderStatusFields()
	for i := range sp.fields {
		if sp.fields[i].Key == "copilot_auth" {
			sp.fields[i].Value = sp.getCopilotAuthStatus()
			break
		}
	}
}

// UpdateCopilotAuthStatus sets the cached Copilot status values and refreshes fields.
func (sp *SettingsPanel) UpdateCopilotAuthStatus(summary, detail string) {
	if strings.TrimSpace(summary) != "" {
		sp.copilotAuthSummary = summary
	}
	if strings.TrimSpace(detail) != "" {
		sp.copilotAuthDetail = detail
	}
	sp.RefreshCopilotAuthStatus()
}

// SetCopilotAuthMessage updates the displayed Copilot auth message prompt.
func (sp *SettingsPanel) SetCopilotAuthMessage(message string) {
	sp.copilotAuthMessage = strings.TrimSpace(message)
	sp.copilotAuthOpen = sp.copilotAuthMessage != ""
}

// ClearCopilotAuthMessage hides the Copilot auth message prompt.
func (sp *SettingsPanel) ClearCopilotAuthMessage() {
	sp.clearCopilotAuthPrompt()
}

func (sp *SettingsPanel) clearCopilotAuthPrompt() {
	sp.copilotAuthMessage = ""
	sp.copilotAuthOpen = false
}

func (sp *SettingsPanel) refreshProviderStatusFields() {
	for i := range sp.fields {
		if sp.fields[i].Key == "provider_status" {
			sp.fields[i].Value = sp.getSelectedProviderStatus()
			break
		}
	}
}

func (sp *SettingsPanel) resetCopilotAuthStatus() {
	sp.copilotAuthSummary = "Not checked"
	sp.copilotAuthDetail = "Not checked (Enter to refresh)"
}

func (sp *SettingsPanel) renderCopilotAuthBox(maxWidth int) string {
	if !sp.copilotAuthOpen {
		return ""
	}
	boxWidth := maxWidth
	if boxWidth > 70 {
		boxWidth = 70
	}
	if boxWidth < 40 {
		boxWidth = 40
	}

	var body strings.Builder
	body.WriteString(styles.TitleStyle.Render("GitHub Copilot Status"))
	body.WriteString("\n\n")
	body.WriteString(styles.TextStyle.Render(sp.copilotAuthMessage))
	body.WriteString("\n\n")

	okButton := styles.SelectedStyle.Render("  OK  ")
	okLine := lipgloss.PlaceHorizontal(boxWidth-4, lipgloss.Center, okButton)
	body.WriteString(okLine)

	return styles.BoxStyleCompact.Width(boxWidth).Render(body.String())
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
