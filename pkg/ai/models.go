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
	modelsURL, err := buildModelsURL(apiURL)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, modelsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("create models request: %w", err)
	}
	req.Header.Set("Accept", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
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
