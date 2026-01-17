package ai

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"wtf_cli/pkg/config"
)

func TestOpenRouterProvider_CreateChatCompletion(t *testing.T) {
	var gotPath string
	var gotAuth string
	var gotReferer string
	var gotTitle string
	var gotPayload map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotReferer = r.Header.Get("HTTP-Referer")
		gotTitle = r.Header.Get("X-Title")

		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
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
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("failed to encode response: %v", err)
		}
	}))
	defer server.Close()

	cfg := config.OpenRouterConfig{
		APIKey:            "test-key",
		APIURL:            server.URL,
		HTTPReferer:       "https://example.com",
		XTitle:            "wtf-cli",
		Model:             "test-model",
		Temperature:       0.4,
		MaxTokens:         55,
		APITimeoutSeconds: 5,
	}

	provider, err := NewOpenRouterProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error: %v", err)
	}

	resp, err := provider.CreateChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hello"}},
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

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := json.NewDecoder(r.Body).Decode(&gotPayload); err != nil {
			t.Errorf("failed to decode request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)

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

		writeChunk := func(payload map[string]any) {
			data, err := json.Marshal(payload)
			if err != nil {
				t.Errorf("failed to marshal chunk: %v", err)
				return
			}
			_, _ = w.Write([]byte("data: "))
			_, _ = w.Write(data)
			_, _ = w.Write([]byte("\n\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}

		writeChunk(chunk1)
		writeChunk(chunk2)

		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		if flusher != nil {
			flusher.Flush()
		}
	}))
	defer server.Close()

	cfg := config.OpenRouterConfig{
		APIKey:            "test-key",
		APIURL:            server.URL,
		Model:             "default-model",
		Temperature:       0.7,
		MaxTokens:         100,
		APITimeoutSeconds: 5,
	}

	provider, err := NewOpenRouterProvider(cfg)
	if err != nil {
		t.Fatalf("NewOpenRouterProvider() error: %v", err)
	}

	temp := 0.2
	maxTokens := 10
	stream, err := provider.CreateChatCompletionStream(context.Background(), ChatRequest{
		Model:       "override-model",
		Messages:    []Message{{Role: "user", Content: "stream"}},
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
