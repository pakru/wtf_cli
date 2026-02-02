package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
)

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func newTestClient(rt roundTripperFunc) *http.Client {
	return &http.Client{Transport: rt}
}

func newHTTPResponse(req *http.Request, status int, contentType string, body []byte) *http.Response {
	resp := &http.Response{
		Status:     fmt.Sprintf("%d %s", status, http.StatusText(status)),
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(body)),
		Request:    req,
	}
	if contentType != "" {
		resp.Header.Set("Content-Type", contentType)
	}
	return resp
}

func newJSONResponse(t *testing.T, req *http.Request, status int, payload any) *http.Response {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal response: %v", err)
	}
	return newHTTPResponse(req, status, "application/json", data)
}

func TestOpenRouterProvider_CreateChatCompletion(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotReferer string
	var gotTitle string
	var gotPayload map[string]any

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotAuth = req.Header.Get("Authorization")
		gotReferer = req.Header.Get("HTTP-Referer")
		gotTitle = req.Header.Get("X-Title")

		if req.Body == nil {
			t.Fatalf("expected request body")
		}
		if err := json.NewDecoder(req.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_ = req.Body.Close()

		resp := map[string]any{
			"id":      "chatcmpl-1",
			"object":  "chat.completion",
			"created": 1,
			"model":   "test-model",
			"choices": []any{
				map[string]any{
					"index": 0,
					"message": map[string]any{
						"role":    "assistant",
						"content": "ok",
					},
					"finish_reason": "stop",
				},
			},
		}
		return newJSONResponse(t, req, http.StatusOK, resp), nil
	})

	cfg := config.OpenRouterConfig{
		APIKey:            "test-key",
		APIURL:            "https://openrouter.test",
		HTTPReferer:       "https://example.com",
		XTitle:            "wtf-cli",
		Model:             "test-model",
		Temperature:       0.4,
		MaxTokens:         55,
		APITimeoutSeconds: 5,
	}

	provider, err := newOpenRouterProviderWithHTTPClient(cfg, client)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error: %v", err)
	}

	resp, err := provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}

	if resp.Content != "ok" {
		t.Fatalf("Expected response content 'ok', got %q", resp.Content)
	}

	if gotPath != "/chat/completions" {
		t.Fatalf("Expected path '/chat/completions', got %q", gotPath)
	}
	if gotAuth != "Bearer test-key" {
		t.Fatalf("Expected Authorization header, got %q", gotAuth)
	}
	if gotReferer != "https://example.com" {
		t.Fatalf("Expected HTTP-Referer header, got %q", gotReferer)
	}
	if gotTitle != "wtf-cli" {
		t.Fatalf("Expected X-Title header, got %q", gotTitle)
	}

	model, _ := gotPayload["model"].(string)
	if model != "test-model" {
		t.Fatalf("Expected model 'test-model', got %q", model)
	}

	messages, ok := gotPayload["messages"].([]any)
	if !ok || len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %v", gotPayload["messages"])
	}
	first, ok := messages[0].(map[string]any)
	if !ok {
		t.Fatalf("Expected message object, got %T", messages[0])
	}
	if first["role"] != "user" {
		t.Fatalf("Expected role 'user', got %v", first["role"])
	}
	if first["content"] != "hello" {
		t.Fatalf("Expected content 'hello', got %v", first["content"])
	}

	temp, _ := gotPayload["temperature"].(float64)
	if math.Abs(temp-0.4) > 0.0001 {
		t.Fatalf("Expected temperature 0.4, got %v", gotPayload["temperature"])
	}

	maxTokens, _ := gotPayload["max_tokens"].(float64)
	if int(maxTokens) != 55 {
		t.Fatalf("Expected max_tokens 55, got %v", gotPayload["max_tokens"])
	}
}

func TestOpenRouterProvider_CreateChatCompletionStream(t *testing.T) {
	var gotPayload map[string]any

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		if req.Body == nil {
			t.Fatalf("expected request body")
		}
		if err := json.NewDecoder(req.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_ = req.Body.Close()

		chunk1 := map[string]any{
			"id":      "stream-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "override-model",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"content": "Hello",
					},
					"finish_reason": "",
				},
			},
		}
		chunk2 := map[string]any{
			"id":      "stream-1",
			"object":  "chat.completion.chunk",
			"created": 1,
			"model":   "override-model",
			"choices": []any{
				map[string]any{
					"index": 0,
					"delta": map[string]any{
						"content": " world",
					},
					"finish_reason": "stop",
				},
			},
		}

		chunk1Data, err := json.Marshal(chunk1)
		if err != nil {
			t.Fatalf("failed to marshal chunk: %v", err)
		}
		chunk2Data, err := json.Marshal(chunk2)
		if err != nil {
			t.Fatalf("failed to marshal chunk: %v", err)
		}

		var stream strings.Builder
		stream.WriteString("data: ")
		_, _ = stream.Write(chunk1Data)
		stream.WriteString("\n\n")
		stream.WriteString("data: ")
		_, _ = stream.Write(chunk2Data)
		stream.WriteString("\n\n")
		stream.WriteString("data: [DONE]\n\n")

		return newHTTPResponse(req, http.StatusOK, "text/event-stream", []byte(stream.String())), nil
	})

	cfg := config.OpenRouterConfig{
		APIKey:            "test-key",
		APIURL:            "https://openrouter.test",
		Model:             "default-model",
		Temperature:       0.7,
		MaxTokens:         100,
		APITimeoutSeconds: 5,
	}

	provider, err := newOpenRouterProviderWithHTTPClient(cfg, client)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error: %v", err)
	}

	temp := 0.2
	maxTokens := 10
	stream, err := provider.CreateChatCompletionStream(context.Background(), ai.ChatRequest{
		Model:       "override-model",
		Messages:    []ai.Message{{Role: "user", Content: "stream"}},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error: %v", err)
	}
	defer stream.Close()

	var output strings.Builder
	for stream.Next() {
		output.WriteString(stream.Content())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if output.String() != "Hello world" {
		t.Fatalf("Expected stream output 'Hello world', got %q", output.String())
	}

	model, _ := gotPayload["model"].(string)
	if model != "override-model" {
		t.Fatalf("Expected model 'override-model', got %q", model)
	}

	streamFlag, _ := gotPayload["stream"].(bool)
	if !streamFlag {
		t.Fatalf("Expected stream=true, got %v", gotPayload["stream"])
	}

	tempValue, _ := gotPayload["temperature"].(float64)
	if math.Abs(tempValue-0.2) > 0.0001 {
		t.Fatalf("Expected temperature 0.2, got %v", gotPayload["temperature"])
	}

	maxTokensValue, _ := gotPayload["max_tokens"].(float64)
	if int(maxTokensValue) != 10 {
		t.Fatalf("Expected max_tokens 10, got %v", gotPayload["max_tokens"])
	}
}

func TestOpenRouterProvider_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.OpenRouterConfig
		wantErr string
	}{
		{
			name:    "missing api_key",
			cfg:     config.OpenRouterConfig{APIURL: "https://test.com", Model: "test", APITimeoutSeconds: 5},
			wantErr: "api_key is required",
		},
		{
			name:    "missing api_url",
			cfg:     config.OpenRouterConfig{APIKey: "key", Model: "test", APITimeoutSeconds: 5},
			wantErr: "api_url is required",
		},
		{
			name:    "missing model",
			cfg:     config.OpenRouterConfig{APIKey: "key", APIURL: "https://test.com", APITimeoutSeconds: 5},
			wantErr: "model is required",
		},
		{
			name:    "invalid timeout",
			cfg:     config.OpenRouterConfig{APIKey: "key", APIURL: "https://test.com", Model: "test", APITimeoutSeconds: 0},
			wantErr: "api_timeout_seconds must be positive",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := newOpenRouterProviderWithHTTPClient(tt.cfg, nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestAnthropicProvider_CreateChatCompletion(t *testing.T) {
	var gotPath string
	var gotAPIKey string
	var gotVersion string
	var gotPayload anthropicRequest

	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		gotPath = req.URL.Path
		gotAPIKey = req.Header.Get("x-api-key")
		gotVersion = req.Header.Get("anthropic-version")

		if req.Body == nil {
			t.Fatalf("expected request body")
		}
		if err := json.NewDecoder(req.Body).Decode(&gotPayload); err != nil {
			t.Fatalf("failed to decode request body: %v", err)
		}
		_ = req.Body.Close()

		resp := anthropicResponse{
			ID:    "msg-1",
			Type:  "message",
			Role:  "assistant",
			Model: "claude-3-5-sonnet-20241022",
			Content: []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			}{
				{Type: "text", Text: "Hello from Claude"},
			},
			StopReason: "end_turn",
		}
		return newJSONResponse(t, req, http.StatusOK, resp), nil
	})

	provider := &AnthropicProvider{
		apiKey:             "test-anthropic-key",
		apiURL:             "https://api.anthropic.test/v1",
		httpClient:         client,
		defaultModel:       "claude-3-5-sonnet-20241022",
		defaultTemperature: 0.5,
		defaultMaxTokens:   1000,
	}

	resp, err := provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: "You are helpful"},
			{Role: "user", Content: "Hello"},
		},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}

	if resp.Content != "Hello from Claude" {
		t.Fatalf("Expected response content 'Hello from Claude', got %q", resp.Content)
	}

	if gotPath != "/v1/messages" {
		t.Fatalf("Expected path '/v1/messages', got %q", gotPath)
	}
	if gotAPIKey != "test-anthropic-key" {
		t.Fatalf("Expected x-api-key header, got %q", gotAPIKey)
	}
	if gotVersion != "2023-06-01" {
		t.Fatalf("Expected anthropic-version header, got %q", gotVersion)
	}

	if gotPayload.Model != "claude-3-5-sonnet-20241022" {
		t.Fatalf("Expected model 'claude-3-5-sonnet-20241022', got %q", gotPayload.Model)
	}
	if gotPayload.System != "You are helpful" {
		t.Fatalf("Expected system prompt 'You are helpful', got %q", gotPayload.System)
	}
	if len(gotPayload.Messages) != 1 {
		t.Fatalf("Expected 1 message (system extracted), got %d", len(gotPayload.Messages))
	}
	if gotPayload.Messages[0].Role != "user" {
		t.Fatalf("Expected role 'user', got %q", gotPayload.Messages[0].Role)
	}
}

func TestAnthropicProvider_CreateChatCompletionStream(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		var stream strings.Builder
		stream.WriteString("event: message_start\n")
		stream.WriteString("data: {\"type\":\"message_start\"}\n\n")
		stream.WriteString("event: content_block_delta\n")
		stream.WriteString("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"Hello\"}}\n\n")
		stream.WriteString("event: content_block_delta\n")
		stream.WriteString("data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\" world\"}}\n\n")
		stream.WriteString("event: message_stop\n")
		stream.WriteString("data: {\"type\":\"message_stop\"}\n\n")

		return newHTTPResponse(req, http.StatusOK, "text/event-stream", []byte(stream.String())), nil
	})

	provider := &AnthropicProvider{
		apiKey:             "test-key",
		apiURL:             "https://api.anthropic.test/v1",
		httpClient:         client,
		defaultModel:       "claude-3-5-sonnet-20241022",
		defaultTemperature: 0.5,
		defaultMaxTokens:   1000,
	}

	stream, err := provider.CreateChatCompletionStream(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "Hello"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error: %v", err)
	}
	defer stream.Close()

	var output strings.Builder
	for stream.Next() {
		output.WriteString(stream.Content())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream error: %v", err)
	}

	if output.String() != "Hello world" {
		t.Fatalf("Expected stream output 'Hello world', got %q", output.String())
	}
}

func TestAnthropicProvider_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.AnthropicConfig
		wantErr string
	}{
		{
			name:    "missing api_key",
			cfg:     config.AnthropicConfig{Model: "claude-3-5-sonnet-20241022"},
			wantErr: "api_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerCfg := ai.ProviderConfig{
				Type: ai.ProviderAnthropic,
				Config: config.Config{
					Providers: config.ProvidersConfig{
						Anthropic: tt.cfg,
					},
				},
			}
			_, err := NewAnthropicProvider(providerCfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestAnthropicProvider_BuildRequest(t *testing.T) {
	provider := &AnthropicProvider{
		defaultModel:       "claude-3-5-sonnet-20241022",
		defaultTemperature: 0.5,
		defaultMaxTokens:   1000,
	}

	tests := []struct {
		name    string
		req     ai.ChatRequest
		wantErr string
	}{
		{
			name:    "empty messages",
			req:     ai.ChatRequest{Messages: []ai.Message{}},
			wantErr: "messages are required",
		},
		{
			name:    "only system message",
			req:     ai.ChatRequest{Messages: []ai.Message{{Role: "system", Content: "test"}}},
			wantErr: "at least one user or assistant message is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.buildRequest(tt.req, false)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestOpenAIProvider_ValidationErrors(t *testing.T) {
	tests := []struct {
		name    string
		cfg     config.OpenAIConfig
		wantErr string
	}{
		{
			name:    "missing api_key",
			cfg:     config.OpenAIConfig{Model: "gpt-4o"},
			wantErr: "api_key is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerCfg := ai.ProviderConfig{
				Type: ai.ProviderOpenAI,
				Config: config.Config{
					Providers: config.ProvidersConfig{
						OpenAI: tt.cfg,
					},
				},
			}
			_, err := NewOpenAIProvider(providerCfg)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestCopilotProvider_BuildCopilotPrompt(t *testing.T) {
	req := ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: "system rules"},
			{Role: "developer", Content: "developer notes"},
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "hi"},
		},
	}

	systemMsg, prompt, err := buildCopilotPrompt(req)
	if err != nil {
		t.Fatalf("buildCopilotPrompt() error: %v", err)
	}
	if !strings.Contains(systemMsg, "SYSTEM") || !strings.Contains(systemMsg, "DEVELOPER") {
		t.Fatalf("expected system content to include roles, got %q", systemMsg)
	}
	if !strings.Contains(prompt, "User: hello") || !strings.Contains(prompt, "Assistant: hi") {
		t.Fatalf("expected prompt to include conversation, got %q", prompt)
	}
}

func TestCopilotProvider_BuildCopilotPrompt_EmptyMessages(t *testing.T) {
	_, _, err := buildCopilotPrompt(ai.ChatRequest{})
	if err == nil {
		t.Fatal("expected error for empty messages")
	}
}

func TestToChatMessageParam(t *testing.T) {
	tests := []struct {
		name    string
		msg     ai.Message
		wantErr bool
	}{
		{name: "system", msg: ai.Message{Role: "system", Content: "test"}, wantErr: false},
		{name: "user", msg: ai.Message{Role: "user", Content: "test"}, wantErr: false},
		{name: "assistant", msg: ai.Message{Role: "assistant", Content: "test"}, wantErr: false},
		{name: "developer", msg: ai.Message{Role: "developer", Content: "test"}, wantErr: false},
		{name: "invalid", msg: ai.Message{Role: "invalid", Content: "test"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := toChatMessageParam(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("toChatMessageParam() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenRouterProvider_BuildChatParams_Validation(t *testing.T) {
	provider := &OpenRouterProvider{
		defaultModel:       "",
		defaultTemperature: 0.5,
		defaultMaxTokens:   100,
	}

	tests := []struct {
		name    string
		req     ai.ChatRequest
		wantErr string
	}{
		{
			name:    "empty model",
			req:     ai.ChatRequest{Model: "", Messages: []ai.Message{{Role: "user", Content: "test"}}},
			wantErr: "model is required",
		},
		{
			name:    "empty messages",
			req:     ai.ChatRequest{Model: "test-model", Messages: []ai.Message{}},
			wantErr: "messages are required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := provider.buildChatParams(tt.req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestNewOpenRouterProviderFromConfig(t *testing.T) {
	cfg := config.OpenRouterConfig{
		APIKey:            "test-key",
		APIURL:            "https://openrouter.test",
		Model:             "test-model",
		Temperature:       0.5,
		MaxTokens:         100,
		APITimeoutSeconds: 30,
	}

	provider, err := NewOpenRouterProviderFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewOpenRouterProviderFromConfig() error: %v", err)
	}
	if provider == nil {
		t.Fatal("expected provider, got nil")
	}
	if provider.defaultModel != "test-model" {
		t.Fatalf("expected model 'test-model', got %q", provider.defaultModel)
	}
}

func withTempHome(t *testing.T) {
	t.Helper()
	tmpDir := t.TempDir()
	oldHome := os.Getenv("HOME")
	t.Setenv("HOME", tmpDir)
	t.Cleanup(func() {
		os.Setenv("HOME", oldHome)
	})
}
