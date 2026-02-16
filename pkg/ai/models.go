package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	copilot "github.com/github/copilot-sdk/go"
	"google.golang.org/genai"
)

const modelCacheFilename = "models_cache.json"

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

// ModelInfo captures the fields needed for model selection and display.
type ModelInfo struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	ContextLength int               `json:"context_length"`
	Pricing       map[string]string `json:"pricing"`
}

type modelListResponse struct {
	Data []ModelInfo `json:"data"`
}

// ModelCache stores the cached model list with a timestamp.
type ModelCache struct {
	UpdatedAt time.Time   `json:"updated_at"`
	Models    []ModelInfo `json:"models"`
}

// DefaultModelCachePath returns the default path for the model cache file.
func DefaultModelCachePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".wtf_cli", modelCacheFilename)
	}
	return filepath.Join(homeDir, ".wtf_cli", modelCacheFilename)
}

// FetchOpenRouterModels retrieves the OpenRouter model list from the API.
func FetchOpenRouterModels(ctx context.Context, apiURL string) ([]ModelInfo, error) {
	slog.Debug("openrouter_models_fetch_start", "api_url", apiURL)
	client := &http.Client{Timeout: 15 * time.Second}
	return fetchOpenRouterModels(ctx, apiURL, client)
}

func fetchOpenRouterModels(ctx context.Context, apiURL string, client httpDoer) ([]ModelInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("http client is required")
	}

	modelsURL, err := buildModelsURL(apiURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("openrouter_models_fetch_error", "api_url", modelsURL, "error", err)
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		slog.Debug("openrouter_models_fetch_failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("models request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload modelListResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	sort.Slice(payload.Data, func(i, j int) bool {
		return payload.Data[i].ID < payload.Data[j].ID
	})

	return payload.Data, nil
}

// RefreshOpenRouterModelCache fetches models and writes the cache to disk.
func RefreshOpenRouterModelCache(ctx context.Context, apiURL, cachePath string) (ModelCache, error) {
	models, err := FetchOpenRouterModels(ctx, apiURL)
	if err != nil {
		return ModelCache{}, err
	}

	cache := ModelCache{
		UpdatedAt: time.Now().UTC(),
		Models:    models,
	}
	if err := SaveModelCache(cachePath, cache); err != nil {
		return ModelCache{}, err
	}

	return cache, nil
}

// LoadModelCache loads the model cache from disk.
func LoadModelCache(path string) (ModelCache, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return ModelCache{}, err
	}

	var cache ModelCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return ModelCache{}, fmt.Errorf("parse model cache: %w", err)
	}

	return cache, nil
}

// SaveModelCache writes the model cache to disk.
func SaveModelCache(path string, cache ModelCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal model cache: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return fmt.Errorf("create model cache directory: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write model cache: %w", err)
	}

	return nil
}

// FetchOpenAIModels retrieves the model list from OpenAI API.
// Endpoint: GET https://api.openai.com/v1/models
func FetchOpenAIModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	slog.Debug("openai_models_fetch_start", "has_key", apiKey != "")
	client := &http.Client{Timeout: 15 * time.Second}
	return fetchOpenAIModels(ctx, apiKey, client)
}

func fetchOpenAIModels(ctx context.Context, apiKey string, client httpDoer) ([]ModelInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.openai.com/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("openai_models_fetch_error", "error", err)
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		slog.Debug("openai_models_fetch_failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("models request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID      string `json:"id"`
			OwnedBy string `json:"owned_by"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	// Filter to only include chat-capable models (gpt-*, o1-*, chatgpt-*)
	var models []ModelInfo
	for _, m := range payload.Data {
		if strings.HasPrefix(m.ID, "gpt-") || strings.HasPrefix(m.ID, "o1-") || strings.HasPrefix(m.ID, "chatgpt-") {
			models = append(models, ModelInfo{
				ID:   m.ID,
				Name: m.ID,
			})
		}
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	slog.Debug("openai_models_fetch_done", "models", len(models))
	return models, nil
}

// FetchAnthropicModels retrieves the model list from Anthropic API.
// Endpoint: GET https://api.anthropic.com/v1/models
func FetchAnthropicModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	slog.Debug("anthropic_models_fetch_start", "has_key", apiKey != "")
	client := &http.Client{Timeout: 15 * time.Second}
	return fetchAnthropicModels(ctx, apiKey, client)
}

func fetchAnthropicModels(ctx context.Context, apiKey string, client httpDoer) ([]ModelInfo, error) {
	if client == nil {
		return nil, fmt.Errorf("http client is required")
	}
	if apiKey == "" {
		return nil, fmt.Errorf("api key is required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.anthropic.com/v1/models", nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		slog.Debug("anthropic_models_fetch_error", "error", err)
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		slog.Debug("anthropic_models_fetch_failed", "status", resp.StatusCode)
		return nil, fmt.Errorf("models request failed (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var payload struct {
		Data []struct {
			ID          string `json:"id"`
			DisplayName string `json:"display_name"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}

	var models []ModelInfo
	for _, m := range payload.Data {
		name := m.DisplayName
		if name == "" {
			name = m.ID
		}
		models = append(models, ModelInfo{
			ID:   m.ID,
			Name: name,
		})
	}

	// Sort by ID (most recent models first since they have dates in IDs)
	sort.Slice(models, func(i, j int) bool {
		return models[i].ID > models[j].ID
	})

	slog.Debug("anthropic_models_fetch_done", "models", len(models))
	return models, nil
}

// GetCopilotModels returns the static fallback list of models for GitHub Copilot.
// Use FetchCopilotModels for dynamic list when authenticated.
func GetCopilotModels() []ModelInfo {
	return []ModelInfo{
		{ID: "gpt-4o", Name: "GPT-4o", Description: "Default Copilot model"},
		{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Description: "Faster Copilot model"},
		{ID: "gpt-4", Name: "GPT-4", Description: "GPT-4 via Copilot"},
		{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Description: "Fast model via Copilot"},
		{ID: "claude-3.5-sonnet", Name: "Claude 3.5 Sonnet", Description: "Anthropic model via Copilot"},
		{ID: "o1-preview", Name: "o1 Preview", Description: "OpenAI reasoning model"},
		{ID: "o1-mini", Name: "o1 Mini", Description: "Smaller reasoning model"},
	}
}

// CopilotAuthStatus captures the Copilot CLI authentication state.
type CopilotAuthStatus struct {
	Authenticated bool
	AuthType      string
	Host          string
	Login         string
	StatusMessage string
}

type copilotSDKClient interface {
	Start() error
	Stop() []error
	GetAuthStatus() (*copilot.GetAuthStatusResponse, error)
	ListModels() ([]copilot.ModelInfo, error)
}

type copilotSDKClientWrapper struct {
	client *copilot.Client
}

func (c *copilotSDKClientWrapper) Start() error {
	return c.client.Start()
}

func (c *copilotSDKClientWrapper) Stop() []error {
	return c.client.Stop()
}

func (c *copilotSDKClientWrapper) GetAuthStatus() (*copilot.GetAuthStatusResponse, error) {
	return c.client.GetAuthStatus()
}

func (c *copilotSDKClientWrapper) ListModels() ([]copilot.ModelInfo, error) {
	return c.client.ListModels()
}

var copilotClientFactory = func() copilotSDKClient {
	return &copilotSDKClientWrapper{client: copilot.NewClient(nil)}
}

var fetchGoogleModelList = func(ctx context.Context, apiKey string) ([]*genai.Model, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create google client: %w", err)
	}

	models := make([]*genai.Model, 0, 16)
	for model, err := range client.Models.All(ctx) {
		if err != nil {
			return nil, fmt.Errorf("iterate google models: %w", err)
		}
		if model != nil {
			models = append(models, model)
		}
	}
	return models, nil
}

// FetchCopilotAuthStatus queries the Copilot CLI auth status via the SDK.
func FetchCopilotAuthStatus(ctx context.Context) (CopilotAuthStatus, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return CopilotAuthStatus{}, err
	}

	slog.Debug("copilot_auth_status_start")
	client := copilotClientFactory()
	if err := client.Start(); err != nil {
		return CopilotAuthStatus{}, fmt.Errorf("start Copilot client: %w", err)
	}
	defer func() {
		logCopilotStopErrors(client.Stop())
	}()

	status, err := client.GetAuthStatus()
	if err != nil {
		return CopilotAuthStatus{}, fmt.Errorf("get Copilot auth status: %w", err)
	}

	result := CopilotAuthStatus{Authenticated: status.IsAuthenticated}
	if status.AuthType != nil {
		result.AuthType = strings.TrimSpace(*status.AuthType)
	}
	if status.Host != nil {
		result.Host = strings.TrimSpace(*status.Host)
	}
	if status.Login != nil {
		result.Login = strings.TrimSpace(*status.Login)
	}
	if status.StatusMessage != nil {
		result.StatusMessage = strings.TrimSpace(*status.StatusMessage)
	}

	slog.Debug("copilot_auth_status_done",
		"authenticated", result.Authenticated,
		"login", result.Login,
	)
	return result, nil
}

// FetchCopilotModels retrieves the model list via the Copilot SDK.
func FetchCopilotModels(ctx context.Context) ([]ModelInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	slog.Debug("copilot_models_fetch_start")
	client := copilotClientFactory()
	if err := client.Start(); err != nil {
		return nil, fmt.Errorf("start Copilot client: %w", err)
	}
	defer func() {
		logCopilotStopErrors(client.Stop())
	}()

	models, err := client.ListModels()
	if err != nil {
		return nil, fmt.Errorf("list Copilot models: %w", err)
	}

	result := make([]ModelInfo, 0, len(models))
	for _, model := range models {
		name := strings.TrimSpace(model.Name)
		if name == "" {
			name = model.ID
		}
		result = append(result, ModelInfo{
			ID:   model.ID,
			Name: name,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})

	slog.Debug("copilot_models_fetch_done", "models", len(result))
	return result, nil
}

// FetchGoogleModels retrieves the model list via the Google AI SDK.
func FetchGoogleModels(ctx context.Context, apiKey string) ([]ModelInfo, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("api key is required")
	}

	slog.Debug("google_models_fetch_start", "has_key", apiKey != "")
	rawModels, err := fetchGoogleModelList(ctx, apiKey)
	if err != nil {
		slog.Debug("google_models_fetch_error", "error", err)
		return nil, err
	}

	models := make([]ModelInfo, 0, len(rawModels))
	for _, model := range rawModels {
		id := normalizeGoogleModelID(model.Name)
		if !strings.HasPrefix(id, "gemini-") {
			continue
		}

		name := strings.TrimSpace(model.DisplayName)
		if name == "" {
			name = id
		}

		models = append(models, ModelInfo{
			ID:            id,
			Name:          name,
			Description:   strings.TrimSpace(model.Description),
			ContextLength: int(model.InputTokenLimit),
		})
	}

	sort.Slice(models, func(i, j int) bool {
		return models[i].ID < models[j].ID
	})

	slog.Debug("google_models_fetch_done", "models", len(models))
	return models, nil
}

// GetProviderModels returns a static fallback list of models for a given provider.
// Use FetchOpenAIModels or FetchAnthropicModels for dynamic lists when API keys are available.
func GetProviderModels(provider string) []ModelInfo {
	switch provider {
	case "openai":
		// Fallback static list when API key is not available
		return []ModelInfo{
			{ID: "gpt-4o", Name: "GPT-4o", Description: "Most capable GPT-4 model"},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Description: "Smaller, faster GPT-4o"},
			{ID: "gpt-4-turbo", Name: "GPT-4 Turbo", Description: "GPT-4 Turbo with vision"},
			{ID: "gpt-4", Name: "GPT-4", Description: "Original GPT-4 model"},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Description: "Fast and cost-effective"},
			{ID: "o1-preview", Name: "o1 Preview", Description: "Reasoning model preview"},
			{ID: "o1-mini", Name: "o1 Mini", Description: "Smaller reasoning model"},
		}
	case "copilot":
		return GetCopilotModels()
	case "anthropic":
		// Fallback static list when API key is not available
		return []ModelInfo{
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Description: "Latest Claude 3.5 Sonnet"},
			{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Description: "Fast Claude 3.5 model"},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Description: "Most capable Claude 3"},
			{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Description: "Balanced Claude 3"},
			{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Description: "Fast Claude 3 model"},
		}
	case "google":
		return []ModelInfo{
			{ID: "gemini-3-flash-preview", Name: "Gemini 3 Flash (Preview)", Description: "Latest generation flash"},
			{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash", Description: "Best price-performance"},
			{ID: "gemini-2.5-pro", Name: "Gemini 2.5 Pro", Description: "Advanced reasoning and coding"},
			{ID: "gemini-2.5-flash-lite", Name: "Gemini 2.5 Flash Lite", Description: "Lightweight, low latency"},
			{ID: "gemini-3-pro-preview", Name: "Gemini 3 Pro (Preview)", Description: "Most capable model"},
		}
	default:
		return nil
	}
}

func logCopilotStopErrors(errors []error) {
	for _, err := range errors {
		if err != nil {
			slog.Debug("copilot_client_stop_error", "error", err)
		}
	}
}

func buildModelsURL(apiURL string) (string, error) {
	trimmed := strings.TrimSpace(apiURL)
	if trimmed == "" {
		return "", fmt.Errorf("api_url is required")
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "", fmt.Errorf("invalid api_url: %w", err)
	}
	if parsed.Scheme == "" || parsed.Host == "" {
		return "", fmt.Errorf("api_url must include scheme and host")
	}

	basePath := strings.TrimRight(parsed.Path, "/")
	parsed.Path = basePath + "/models"

	return parsed.String(), nil
}

func normalizeGoogleModelID(name string) string {
	id := strings.TrimSpace(name)
	if id == "" {
		return ""
	}

	// Gemini API typically returns "models/<id>".
	id = strings.TrimPrefix(id, "models/")

	// Be tolerant of fully-qualified names such as ".../models/<id>".
	if idx := strings.LastIndex(id, "/models/"); idx >= 0 {
		id = id[idx+len("/models/"):]
	}

	return strings.TrimSpace(id)
}
