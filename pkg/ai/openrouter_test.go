package ai

import (
	"context"
	"encoding/json"
	"math"
	"net/http"
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
