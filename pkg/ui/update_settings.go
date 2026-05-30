package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/logging"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/components/settings"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleSettingsClose() (Model, tea.Cmd) {
	// Settings panel closed
	slog.Info("settings_close")
	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		m.modelPicker.Hide()
	}
	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		m.optionPicker.Hide()
	}
	return m, nil
}

func (m Model) handleStartCopilotAuth() (Model, tea.Cmd) {
	slog.Info("copilot_auth_status_request")
	return m, fetchCopilotAuthStatusCmd(true)
}

func (m Model) handleCopilotAuthStatus(msg copilotAuthStatusMsg) (Model, tea.Cmd) {
	summary, detail, message := formatCopilotAuthStatus(msg.Status, msg.Err)
	if m.settingsPanel != nil {
		m.settingsPanel.UpdateCopilotAuthStatus(summary, detail)
		if msg.ShowPrompt {
			m.settingsPanel.SetCopilotAuthMessage(message)
		}
	}
	return m, nil
}

func (m Model) handleSettingsSave(msg settings.SettingsSaveMsg) (Model, tea.Cmd) {
	// Save settings to file
	if err := config.Save(msg.ConfigPath, msg.Config); err != nil {
		slog.Error("settings_save_error", "error", err)
	} else {
		slog.Info("settings_save",
			"provider", msg.Config.LLMProvider,
			"model", getModelForProvider(msg.Config),
			"log_level", msg.Config.LogLevel,
			"log_format", msg.Config.LogFormat,
			"log_file", msg.Config.LogFile,
		)
		logging.SetLevel(msg.Config.LogLevel)
	}
	provider, model := getProviderAndModel(msg.Config)
	m.sidebar.SetActiveLLM(provider, model)
	return m, nil
}

func (m Model) handleOpenModelPicker(msg picker.OpenModelPickerMsg) (Model, tea.Cmd) {
	slog.Info("model_picker_open", "current", msg.Current, "field_key", msg.FieldKey, "cached_models", len(msg.Options))
	slog.Debug("model_picker_open_details",
		"field_key", msg.FieldKey,
		"api_url", msg.APIURL,
		"has_api_key", msg.APIKey != "",
	)
	if m.modelPicker != nil {
		m.modelPicker.SetSize(m.width, m.height)
		m.modelPicker.Show(msg.Options, msg.Current, msg.FieldKey)
	}
	// Fetch dynamic model list based on provider
	var cmd tea.Cmd
	switch msg.FieldKey {
	case "model":
		if msg.APIURL != "" {
			cmd = refreshModelCacheCmd(msg.APIURL)
		} else {
			slog.Debug("model_picker_no_api_url")
		}
	case "openai_model":
		if msg.APIKey != "" {
			cmd = fetchOpenAIModelsCmd(msg.APIKey)
		} else {
			slog.Debug("openai_models_fetch_skipped", "reason", "missing_api_key")
		}
	case "copilot_model":
		cmd = fetchCopilotModelsCmd()
	case "anthropic_model":
		if msg.APIKey != "" {
			cmd = fetchAnthropicModelsCmd(msg.APIKey)
		} else {
			slog.Debug("anthropic_models_fetch_skipped", "reason", "missing_api_key")
		}
	case "google_model":
		if msg.APIKey != "" {
			cmd = fetchGoogleModelsCmd(msg.APIKey)
		} else {
			slog.Debug("google_models_fetch_skipped", "reason", "missing_api_key")
		}
	}
	return m, cmd
}

func (m Model) handleModelPickerSelect(msg picker.ModelPickerSelectMsg) (Model, tea.Cmd) {
	slog.Info("model_picker_select", "model", msg.ModelID, "field_key", msg.FieldKey)
	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		m.modelPicker.Hide()
	}
	if m.settingsPanel != nil {
		switch msg.FieldKey {
		case "model":
			m.settingsPanel.SetModelValue(msg.ModelID)
		case "openai_model":
			m.settingsPanel.SetOpenAIModelValue(msg.ModelID)
		case "copilot_model":
			m.settingsPanel.SetCopilotModelValue(msg.ModelID)
		case "anthropic_model":
			m.settingsPanel.SetAnthropicModelValue(msg.ModelID)
		case "google_model":
			m.settingsPanel.SetGoogleModelValue(msg.ModelID)
		default:
			// Fallback to OpenRouter model for backwards compatibility
			m.settingsPanel.SetModelValue(msg.ModelID)
		}
	}
	return m, nil
}

func (m Model) handleOpenOptionPicker(msg picker.OpenOptionPickerMsg) (Model, tea.Cmd) {
	slog.Info("option_picker_open", "field", msg.FieldKey, "current", msg.Current)
	if m.optionPicker != nil {
		m.optionPicker.SetSize(m.width, m.height)
		m.optionPicker.Show(msg.Title, msg.FieldKey, msg.Options, msg.Current)
	}
	return m, nil
}

func (m Model) handleOptionPickerSelect(msg picker.OptionPickerSelectMsg) (Model, tea.Cmd) {
	slog.Info("option_picker_select", "field", msg.FieldKey, "value", msg.Value)
	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		m.optionPicker.Hide()
	}
	if m.settingsPanel != nil {
		switch msg.FieldKey {
		case "llm_provider":
			m.settingsPanel.SetProviderValue(msg.Value)
			if msg.Value == "copilot" {
				return m, fetchCopilotAuthStatusCmd(false)
			}
		case "log_level":
			m.settingsPanel.SetLogLevelValue(msg.Value)
		case "log_format":
			m.settingsPanel.SetLogFormatValue(msg.Value)
		}
	}
	return m, nil
}

func (m Model) handleModelPickerRefresh(msg picker.ModelPickerRefreshMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		slog.Error("model_picker_refresh_error", "error", msg.Err)
		return m, nil
	}
	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		m.modelPicker.UpdateOptions(msg.Cache.Models)
	}
	if m.settingsPanel != nil {
		m.settingsPanel.SetModelCache(msg.Cache)
	}
	slog.Info("model_picker_refresh_done", "models", len(msg.Cache.Models))
	return m, nil
}

func (m Model) handleProviderModelsRefresh(msg providerModelsRefreshMsg) (Model, tea.Cmd) {
	if msg.Err != nil {
		slog.Error("provider_models_refresh_error", "field_key", msg.FieldKey, "error", msg.Err)
		return m, nil
	}
	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		m.modelPicker.UpdateOptions(msg.Models)
	}
	slog.Info("provider_models_refresh_done", "field_key", msg.FieldKey, "models", len(msg.Models))
	return m, nil
}

func refreshModelCacheCmd(apiURL string) tea.Cmd {
	trimmed := strings.TrimSpace(apiURL)
	if trimmed == "" {
		return nil
	}

	return func() tea.Msg {
		slog.Info("model_picker_refresh_start", "api_url", trimmed)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cache, err := ai.RefreshOpenRouterModelCache(ctx, trimmed, ai.DefaultModelCachePath())
		return picker.ModelPickerRefreshMsg{Cache: cache, Err: err}
	}
}

// providerModelsRefreshMsg is sent when dynamic model fetching completes
type providerModelsRefreshMsg struct {
	Models   []ai.ModelInfo
	FieldKey string
	Err      error
}

func fetchOpenAIModelsCmd(apiKey string) tea.Cmd {
	return fetchAPIKeyProviderModelsCmd("openai_model", "openai_models_fetch_start", apiKey, ai.FetchOpenAIModels)
}

func fetchAnthropicModelsCmd(apiKey string) tea.Cmd {
	return fetchAPIKeyProviderModelsCmd("anthropic_model", "anthropic_models_fetch_start", apiKey, ai.FetchAnthropicModels)
}

func fetchGoogleModelsCmd(apiKey string) tea.Cmd {
	return fetchAPIKeyProviderModelsCmd("google_model", "google_models_fetch_start", apiKey, ai.FetchGoogleModels)
}

func fetchAPIKeyProviderModelsCmd(fieldKey, logEvent, apiKey string, fetch func(context.Context, string) ([]ai.ModelInfo, error)) tea.Cmd {
	if apiKey == "" {
		return nil
	}

	return func() tea.Msg {
		slog.Info(logEvent)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		models, err := fetch(ctx, apiKey)
		return providerModelsRefreshMsg{Models: models, FieldKey: fieldKey, Err: err}
	}
}

func fetchCopilotModelsCmd() tea.Cmd {
	return func() tea.Msg {
		slog.Info("copilot_models_fetch_start")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		models, err := ai.FetchCopilotModels(ctx)
		return providerModelsRefreshMsg{Models: models, FieldKey: "copilot_model", Err: err}
	}
}

// Copilot auth status message type.
type copilotAuthStatusMsg struct {
	Status     ai.CopilotAuthStatus
	Err        error
	ShowPrompt bool
}

// fetchCopilotAuthStatusCmd queries the Copilot CLI auth status using the SDK.
func fetchCopilotAuthStatusCmd(showPrompt bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		slog.Info("copilot_auth_status_start")
		status, err := ai.FetchCopilotAuthStatus(ctx)
		if err != nil {
			slog.Error("copilot_auth_status_error", "error", err)
			return copilotAuthStatusMsg{Err: err, ShowPrompt: showPrompt}
		}

		slog.Info("copilot_auth_status_done", "authenticated", status.Authenticated)
		return copilotAuthStatusMsg{Status: status, ShowPrompt: showPrompt}
	}
}

func formatCopilotAuthStatus(status ai.CopilotAuthStatus, err error) (string, string, string) {
	summary := "Not connected"
	detail := "Not connected (Enter for details)"
	statusLabel := "Not connected"
	if err != nil {
		message := fmt.Sprintf("Status: %s\nError: %v", statusLabel, err)
		return summary, detail, message
	}

	if status.Authenticated {
		summary = "Connected"
		detail = "Connected (Enter for details)"
		statusLabel = "Connected"
	}

	lines := []string{fmt.Sprintf("Status: %s", statusLabel)}
	if status.Login != "" {
		lines = append(lines, fmt.Sprintf("User: %s", status.Login))
	}
	if status.AuthType != "" {
		lines = append(lines, fmt.Sprintf("Auth: %s", status.AuthType))
	}
	if status.Host != "" {
		lines = append(lines, fmt.Sprintf("Host: %s", status.Host))
	}
	if status.StatusMessage != "" {
		lines = append(lines, status.StatusMessage)
	}

	return summary, detail, strings.Join(lines, "\n")
}
