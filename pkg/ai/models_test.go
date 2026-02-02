package ai

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	copilot "github.com/github/copilot-sdk/go"
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

func TestFetchOpenAIModels(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		// Verify headers
		if got := req.Header.Get("Authorization"); got != "Bearer test-api-key" {
			t.Errorf("Expected Authorization header 'Bearer test-api-key', got %q", got)
		}
		payload := map[string]any{
			"data": []any{
				map[string]any{"id": "gpt-4o", "owned_by": "openai"},
				map[string]any{"id": "gpt-3.5-turbo", "owned_by": "openai"},
				map[string]any{"id": "o1-mini", "owned_by": "openai"},
				map[string]any{"id": "dall-e-3", "owned_by": "openai"},  // Should be filtered out
				map[string]any{"id": "whisper-1", "owned_by": "openai"}, // Should be filtered out
				map[string]any{"id": "chatgpt-4o-latest", "owned_by": "openai"},
			},
		}
		return newJSONResponse(t, req, http.StatusOK, payload), nil
	})

	models, err := fetchOpenAIModels(context.Background(), "test-api-key", client)
	if err != nil {
		t.Fatalf("fetchOpenAIModels() error: %v", err)
	}
	// Should only include gpt-*, o1-*, chatgpt-* models (4 models, not dall-e or whisper)
	if len(models) != 4 {
		t.Fatalf("Expected 4 models, got %d", len(models))
	}
	// Should be sorted alphabetically
	if models[0].ID != "chatgpt-4o-latest" {
		t.Errorf("Expected first model 'chatgpt-4o-latest', got %q", models[0].ID)
	}
}

func TestFetchOpenAIModels_EmptyAPIKey(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		t.Fatal("Should not make request with empty API key")
		return nil, nil
	})

	_, err := fetchOpenAIModels(context.Background(), "", client)
	if err == nil {
		t.Fatal("Expected error for empty API key")
	}
}

func TestFetchOpenAIModels_HTTPError(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return newJSONResponse(t, req, http.StatusUnauthorized, map[string]any{
			"error": map[string]any{"message": "Invalid API key"},
		}), nil
	})

	_, err := fetchOpenAIModels(context.Background(), "bad-key", client)
	if err == nil {
		t.Fatal("Expected error for HTTP error response")
	}
}

func TestFetchAnthropicModels(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		// Verify headers
		if got := req.Header.Get("x-api-key"); got != "test-api-key" {
			t.Errorf("Expected x-api-key header 'test-api-key', got %q", got)
		}
		if got := req.Header.Get("anthropic-version"); got != "2023-06-01" {
			t.Errorf("Expected anthropic-version header '2023-06-01', got %q", got)
		}
		payload := map[string]any{
			"data": []any{
				map[string]any{"id": "claude-3-opus-20240229", "display_name": "Claude 3 Opus"},
				map[string]any{"id": "claude-3-5-sonnet-20241022", "display_name": "Claude 3.5 Sonnet"},
				map[string]any{"id": "claude-3-haiku-20240307", "display_name": ""},
			},
		}
		return newJSONResponse(t, req, http.StatusOK, payload), nil
	})

	models, err := fetchAnthropicModels(context.Background(), "test-api-key", client)
	if err != nil {
		t.Fatalf("fetchAnthropicModels() error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(models))
	}
	// Should be sorted descending by ID (newest first based on date in ID)
	// claude-3-opus-20240229 > claude-3-haiku-20240307 > claude-3-5-sonnet-20241022 (alphabetically descending)
	// Actually: "claude-3-opus" > "claude-3-haiku" > "claude-3-5-sonnet" alphabetically descending
	if models[0].ID != "claude-3-opus-20240229" {
		t.Errorf("Expected first model 'claude-3-opus-20240229', got %q", models[0].ID)
	}
	// Display name should be used when available
	if models[0].Name != "Claude 3 Opus" {
		t.Errorf("Expected name 'Claude 3 Opus', got %q", models[0].Name)
	}
	// Find the model with empty display_name and verify ID is used as name
	var foundHaiku bool
	for _, m := range models {
		if m.ID == "claude-3-haiku-20240307" {
			foundHaiku = true
			if m.Name != "claude-3-haiku-20240307" {
				t.Errorf("Expected name to fall back to ID for haiku, got %q", m.Name)
			}
		}
	}
	if !foundHaiku {
		t.Error("Expected to find claude-3-haiku-20240307 in models")
	}
}

func TestFetchAnthropicModels_EmptyAPIKey(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		t.Fatal("Should not make request with empty API key")
		return nil, nil
	})

	_, err := fetchAnthropicModels(context.Background(), "", client)
	if err == nil {
		t.Fatal("Expected error for empty API key")
	}
}

type stubCopilotClient struct {
	startErr   error
	listErr    error
	statusErr  error
	models     []copilot.ModelInfo
	status     *copilot.GetAuthStatusResponse
	stopErrors []error
}

func (s *stubCopilotClient) Start() error {
	return s.startErr
}

func (s *stubCopilotClient) Stop() []error {
	return s.stopErrors
}

func (s *stubCopilotClient) GetAuthStatus() (*copilot.GetAuthStatusResponse, error) {
	if s.statusErr != nil {
		return nil, s.statusErr
	}
	if s.status == nil {
		return &copilot.GetAuthStatusResponse{}, nil
	}
	return s.status, nil
}

func (s *stubCopilotClient) ListModels() ([]copilot.ModelInfo, error) {
	if s.listErr != nil {
		return nil, s.listErr
	}
	return s.models, nil
}

func TestFetchCopilotModels(t *testing.T) {
	origFactory := copilotClientFactory
	defer func() {
		copilotClientFactory = origFactory
	}()

	stub := &stubCopilotClient{
		models: []copilot.ModelInfo{
			{ID: "gpt-4o", Name: "GPT-4o"},
			{ID: "claude-3.5-sonnet", Name: ""},
			{ID: "gpt-4o-mini", Name: "GPT-4o Mini"},
		},
	}
	copilotClientFactory = func() copilotSDKClient { return stub }

	models, err := FetchCopilotModels(context.Background())
	if err != nil {
		t.Fatalf("FetchCopilotModels() error: %v", err)
	}
	if len(models) != 3 {
		t.Fatalf("Expected 3 models, got %d", len(models))
	}
	if models[0].ID != "claude-3.5-sonnet" {
		t.Errorf("Expected sorted models, got %q first", models[0].ID)
	}
	if models[0].Name != "claude-3.5-sonnet" {
		t.Errorf("Expected fallback name to ID, got %q", models[0].Name)
	}
}

func TestFetchCopilotModels_StartError(t *testing.T) {
	origFactory := copilotClientFactory
	defer func() {
		copilotClientFactory = origFactory
	}()

	stub := &stubCopilotClient{startErr: errors.New("start failed")}
	copilotClientFactory = func() copilotSDKClient { return stub }

	_, err := FetchCopilotModels(context.Background())
	if err == nil {
		t.Fatal("Expected error for Copilot client start failure")
	}
}

func TestFetchCopilotAuthStatus(t *testing.T) {
	origFactory := copilotClientFactory
	defer func() {
		copilotClientFactory = origFactory
	}()

	authType := "oauth"
	host := "github.com"
	login := "octocat"
	message := "Authenticated"
	stub := &stubCopilotClient{
		status: &copilot.GetAuthStatusResponse{
			IsAuthenticated: true,
			AuthType:        &authType,
			Host:            &host,
			Login:           &login,
			StatusMessage:   &message,
		},
	}
	copilotClientFactory = func() copilotSDKClient { return stub }

	status, err := FetchCopilotAuthStatus(context.Background())
	if err != nil {
		t.Fatalf("FetchCopilotAuthStatus() error: %v", err)
	}
	if !status.Authenticated {
		t.Fatal("Expected authenticated status")
	}
	if status.Login != "octocat" {
		t.Fatalf("Expected login 'octocat', got %q", status.Login)
	}
	if status.StatusMessage != "Authenticated" {
		t.Fatalf("Expected status message, got %q", status.StatusMessage)
	}
}

func TestFetchCopilotAuthStatus_Error(t *testing.T) {
	origFactory := copilotClientFactory
	defer func() {
		copilotClientFactory = origFactory
	}()

	stub := &stubCopilotClient{statusErr: errors.New("status error")}
	copilotClientFactory = func() copilotSDKClient { return stub }

	_, err := FetchCopilotAuthStatus(context.Background())
	if err == nil {
		t.Fatal("Expected error for auth status failure")
	}
}

func TestGetCopilotModels(t *testing.T) {
	models := GetCopilotModels()
	if len(models) == 0 {
		t.Fatal("Expected non-empty model list")
	}
	// Verify some expected models are present
	found := make(map[string]bool)
	for _, m := range models {
		found[m.ID] = true
	}
	expectedModels := []string{"gpt-4o", "gpt-4o-mini", "gpt-4", "gpt-3.5-turbo"}
	for _, id := range expectedModels {
		if !found[id] {
			t.Errorf("Expected model %q in static list", id)
		}
	}
}

func TestGetProviderModels(t *testing.T) {
	tests := []struct {
		provider string
		wantLen  int
	}{
		{"openai", 7},
		{"copilot", 7},
		{"anthropic", 5},
		{"unknown", 0},
	}

	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			models := GetProviderModels(tt.provider)
			if tt.wantLen == 0 {
				if models != nil {
					t.Errorf("Expected nil for unknown provider, got %d models", len(models))
				}
			} else if len(models) != tt.wantLen {
				t.Errorf("Expected %d models for %s, got %d", tt.wantLen, tt.provider, len(models))
			}
		})
	}
}
