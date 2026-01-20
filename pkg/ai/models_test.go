package ai

import (
	"context"
	"net/http"
	"path/filepath"
	"testing"
	"time"
)

func TestFetchOpenRouterModels(t *testing.T) {
	var gotPath string
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		gotPath = req.URL.Path
		payload := map[string]any{
			"data": []any{
				map[string]any{
					"id":             "b-model",
					"name":           "B Model",
					"context_length": 2000,
					"pricing": map[string]any{
						"prompt":     "0.01",
						"completion": "0.02",
					},
				},
				map[string]any{
					"id":             "a-model",
					"name":           "A Model",
					"context_length": 1000,
					"pricing": map[string]any{
						"prompt":     "0.001",
						"completion": "0.002",
					},
				},
			},
		}
		return newJSONResponse(t, req, http.StatusOK, payload), nil
	})

	models, err := fetchOpenRouterModels(context.Background(), "https://openrouter.test/api/v1", client)
	if err != nil {
		t.Fatalf("FetchOpenRouterModels() error: %v", err)
	}
	if gotPath != "/api/v1/models" {
		t.Fatalf("Expected path /api/v1/models, got %q", gotPath)
	}
	if len(models) != 2 {
		t.Fatalf("Expected 2 models, got %d", len(models))
	}
	if models[0].ID != "a-model" {
		t.Fatalf("Expected models sorted by ID, got %q", models[0].ID)
	}
	if models[0].Pricing["prompt"] != "0.001" {
		t.Fatalf("Expected prompt pricing, got %q", models[0].Pricing["prompt"])
	}
}

func TestModelCacheReadWrite(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "models_cache.json")

	expected := ModelCache{
		UpdatedAt: time.Date(2025, 1, 15, 12, 30, 0, 0, time.UTC),
		Models: []ModelInfo{
			{
				ID:            "test-model",
				Name:          "Test Model",
				ContextLength: 1234,
				Pricing: map[string]string{
					"prompt":     "0.01",
					"completion": "0.02",
				},
			},
		},
	}

	if err := SaveModelCache(cachePath, expected); err != nil {
		t.Fatalf("SaveModelCache() error: %v", err)
	}

	cache, err := LoadModelCache(cachePath)
	if err != nil {
		t.Fatalf("LoadModelCache() error: %v", err)
	}

	if cache.UpdatedAt.Format(time.RFC3339) != expected.UpdatedAt.Format(time.RFC3339) {
		t.Fatalf("UpdatedAt mismatch: %v vs %v", cache.UpdatedAt, expected.UpdatedAt)
	}
	if len(cache.Models) != 1 {
		t.Fatalf("Expected 1 model, got %d", len(cache.Models))
	}
	if cache.Models[0].ID != "test-model" {
		t.Fatalf("Expected model ID 'test-model', got %q", cache.Models[0].ID)
	}
}
