package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"wtf_cli/logger"
)

func TestNewClient(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")

	apiKey := "test-api-key"
	client := NewClient(apiKey)

	if client == nil {
		t.Fatal("NewClient returned nil")
	}

	// Test that client has proper timeout
	if client.HTTPClient.Timeout != 30*time.Second {
		t.Errorf("Expected timeout 30s, got %v", client.HTTPClient.Timeout)
	}

	// Test that API key is set
	if client.APIKey != apiKey {
		t.Errorf("Expected API key %s, got %s", apiKey, client.APIKey)
	}

	// Test default values
	if client.BaseURL != DefaultBaseURL {
		t.Errorf("Expected base URL %s, got %s", DefaultBaseURL, client.BaseURL)
	}
}

func TestClientChatCompletion_Success(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")

	// Mock successful response
	mockResponse := Response{
		ID:      "chatcmpl-test",
		Object:  "chat.completion",
		Created: 1677652288,
		Model:   "google/gemma-3-27b",
		Choices: []Choice{
			{
				Index: 0,
				Message: Message{
					Role:    "assistant",
					Content: "Here's how to fix your command...",
				},
				FinishReason: "stop",
			},
		},
		Usage: Usage{
			PromptTokens:     50,
			CompletionTokens: 100,
			TotalTokens:      150,
		},
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request method and headers
		if r.Method != "POST" {
			t.Errorf("Expected POST method, got %s", r.Method)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Expected Content-Type application/json, got %s", r.Header.Get("Content-Type"))
		}
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("Expected Authorization header with Bearer token")
		}

		// Send mock response
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(mockResponse)
	}))
	defer server.Close()

	client := &Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	request := Request{
		Model: "google/gemma-3-27b",
		Messages: []Message{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Help me debug this command."},
		},
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	response, err := client.ChatCompletion(request)
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	if response.ID != mockResponse.ID {
		t.Errorf("Expected ID %s, got %s", mockResponse.ID, response.ID)
	}
	if len(response.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Message.Content != mockResponse.Choices[0].Message.Content {
		t.Errorf("Expected content %s, got %s",
			mockResponse.Choices[0].Message.Content,
			response.Choices[0].Message.Content)
	}
}

func TestClientChatCompletion_DefaultValues(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := Response{
			ID: "test",
			Choices: []Choice{
				{Message: Message{Role: "assistant", Content: "test"}},
			},
		}
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	client := &Client{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}

	// Request with empty values that should get defaults
	request := Request{
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	_, err := client.ChatCompletion(request)
	if err != nil {
		t.Fatalf("ChatCompletion failed: %v", err)
	}

	// Note: The ChatCompletion method modifies the request struct internally
	// but doesn't return the modified values, so we can't test them here
}

func TestClientChatCompletion_NetworkError(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")

	client := &Client{
		APIKey:     "test-key",
		BaseURL:    "http://nonexistent-server.local",
		HTTPClient: &http.Client{Timeout: 1 * time.Millisecond}, // Very short timeout
	}

	request := Request{
		Model: "google/gemma-3-27b",
		Messages: []Message{
			{Role: "user", Content: "Test"},
		},
	}

	_, err := client.ChatCompletion(request)
	if err == nil {
		t.Fatal("Expected network error, got nil")
	}

	// Should be a network-related error
	if !strings.Contains(err.Error(), "failed to send request") && !strings.Contains(err.Error(), "timeout") {
		t.Errorf("Expected network error, got: %v", err)
	}
}
