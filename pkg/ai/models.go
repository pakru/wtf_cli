package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
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
		return nil, fmt.Errorf("fetch models: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
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

// GetProviderModels returns the available models for a given provider.
// For OpenRouter, this returns an empty list (use FetchOpenRouterModels instead).
// For other providers, this returns a static list of commonly available models.
func GetProviderModels(provider string) []ModelInfo {
	switch provider {
	case "openai":
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
		return []ModelInfo{
			{ID: "gpt-4o", Name: "GPT-4o", Description: "Default Copilot model"},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini", Description: "Faster Copilot model"},
			{ID: "gpt-4", Name: "GPT-4", Description: "GPT-4 via Copilot"},
			{ID: "gpt-3.5-turbo", Name: "GPT-3.5 Turbo", Description: "Fast model via Copilot"},
		}
	case "anthropic":
		return []ModelInfo{
			{ID: "claude-3-5-sonnet-20241022", Name: "Claude 3.5 Sonnet", Description: "Latest Claude 3.5 Sonnet"},
			{ID: "claude-3-5-haiku-20241022", Name: "Claude 3.5 Haiku", Description: "Fast Claude 3.5 model"},
			{ID: "claude-3-opus-20240229", Name: "Claude 3 Opus", Description: "Most capable Claude 3"},
			{ID: "claude-3-sonnet-20240229", Name: "Claude 3 Sonnet", Description: "Balanced Claude 3"},
			{ID: "claude-3-haiku-20240307", Name: "Claude 3 Haiku", Description: "Fast Claude 3 model"},
		}
	default:
		return nil
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
