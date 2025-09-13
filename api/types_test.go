package api

import (
	"encoding/json"
	"testing"
)

func TestRequestSerialization(t *testing.T) {
	request := Request{
		Model:       "google/gemma-3-27b",
		Messages:    []Message{{Role: "user", Content: "test"}},
		Temperature: 0.7,
		MaxTokens:   1000,
	}

	data, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("Failed to marshal request: %v", err)
	}

	var unmarshaled Request
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal request: %v", err)
	}

	if unmarshaled.Model != request.Model {
		t.Errorf("Expected model %s, got %s", request.Model, unmarshaled.Model)
	}
	if unmarshaled.Temperature != request.Temperature {
		t.Errorf("Expected temperature %f, got %f", request.Temperature, unmarshaled.Temperature)
	}
	if unmarshaled.MaxTokens != request.MaxTokens {
		t.Errorf("Expected max_tokens %d, got %d", request.MaxTokens, unmarshaled.MaxTokens)
	}
	if len(unmarshaled.Messages) != 1 {
		t.Errorf("Expected 1 message, got %d", len(unmarshaled.Messages))
	}
}

func TestResponseDeserialization(t *testing.T) {
	jsonResponse := `{
		"id": "chatcmpl-123",
		"object": "chat.completion",
		"created": 1677652288,
		"model": "google/gemma-3-27b",
		"choices": [{
			"index": 0,
			"message": {
				"role": "assistant",
				"content": "Test response"
			},
			"finish_reason": "stop"
		}],
		"usage": {
			"prompt_tokens": 10,
			"completion_tokens": 20,
			"total_tokens": 30
		}
	}`

	var response Response
	err := json.Unmarshal([]byte(jsonResponse), &response)
	if err != nil {
		t.Fatalf("Failed to unmarshal response: %v", err)
	}

	if response.ID != "chatcmpl-123" {
		t.Errorf("Expected ID 'chatcmpl-123', got '%s'", response.ID)
	}
	if response.Model != "google/gemma-3-27b" {
		t.Errorf("Expected model 'google/gemma-3-27b', got '%s'", response.Model)
	}
	if len(response.Choices) != 1 {
		t.Errorf("Expected 1 choice, got %d", len(response.Choices))
	}
	if response.Choices[0].Message.Content != "Test response" {
		t.Errorf("Expected content 'Test response', got '%s'", response.Choices[0].Message.Content)
	}
	if response.Usage.TotalTokens != 30 {
		t.Errorf("Expected total tokens 30, got %d", response.Usage.TotalTokens)
	}
}

func TestAPIErrorDeserialization(t *testing.T) {
	jsonError := `{
		"error": {
			"message": "Invalid API key",
			"type": "authentication_error",
			"code": "invalid_api_key"
		}
	}`

	var apiError APIError
	err := json.Unmarshal([]byte(jsonError), &apiError)
	if err != nil {
		t.Fatalf("Failed to unmarshal API error: %v", err)
	}

	if apiError.Error.Message != "Invalid API key" {
		t.Errorf("Expected message 'Invalid API key', got '%s'", apiError.Error.Message)
	}
	if apiError.Error.Type != "authentication_error" {
		t.Errorf("Expected type 'authentication_error', got '%s'", apiError.Error.Type)
	}
	if apiError.Error.Code != "invalid_api_key" {
		t.Errorf("Expected code 'invalid_api_key', got '%s'", apiError.Error.Code)
	}
}

func TestCommandInfoValidation(t *testing.T) {
	tests := []struct {
		name    string
		cmdInfo CommandInfo
		valid   bool
	}{
		{
			name: "valid command info",
			cmdInfo: CommandInfo{
				Command:    "git push",
				ExitCode:   "1",
				Output:     "Permission denied",
				WorkingDir: "/home/user/project",
				Duration:   "0.5s",
			},
			valid: true,
		},
		{
			name: "empty command",
			cmdInfo: CommandInfo{
				Command:    "",
				ExitCode:   "1",
				Output:     "Error",
				WorkingDir: "/home/user",
				Duration:   "0.1s",
			},
			valid: false,
		},
		{
			name: "minimal valid command",
			cmdInfo: CommandInfo{
				Command:  "ls",
				ExitCode: "0",
			},
			valid: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test JSON serialization
			data, err := json.Marshal(tt.cmdInfo)
			if err != nil {
				t.Fatalf("Failed to marshal CommandInfo: %v", err)
			}

			var unmarshaled CommandInfo
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal CommandInfo: %v", err)
			}

			if unmarshaled.Command != tt.cmdInfo.Command {
				t.Errorf("Expected command '%s', got '%s'", tt.cmdInfo.Command, unmarshaled.Command)
			}
		})
	}
}

func TestSystemInfoValidation(t *testing.T) {
	sysInfo := SystemInfo{
		OS:           "linux",
		Distribution: "Ubuntu",
		Kernel:       "5.4.0",
		Shell:        "/bin/bash",
		User:         "testuser",
		Home:         "/home/testuser",
	}

	// Test JSON serialization
	data, err := json.Marshal(sysInfo)
	if err != nil {
		t.Fatalf("Failed to marshal SystemInfo: %v", err)
	}

	var unmarshaled SystemInfo
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("Failed to unmarshal SystemInfo: %v", err)
	}

	if unmarshaled.OS != sysInfo.OS {
		t.Errorf("Expected OS '%s', got '%s'", sysInfo.OS, unmarshaled.OS)
	}
	if unmarshaled.Distribution != sysInfo.Distribution {
		t.Errorf("Expected distribution '%s', got '%s'", sysInfo.Distribution, unmarshaled.Distribution)
	}
	if unmarshaled.User != sysInfo.User {
		t.Errorf("Expected user '%s', got '%s'", sysInfo.User, unmarshaled.User)
	}
}

func TestMessageTypes(t *testing.T) {
	tests := []struct {
		name    string
		message Message
	}{
		{
			name: "system message",
			message: Message{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
		},
		{
			name: "user message",
			message: Message{
				Role:    "user",
				Content: "Help me debug this command.",
			},
		},
		{
			name: "assistant message",
			message: Message{
				Role:    "assistant",
				Content: "Here's what went wrong...",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.message)
			if err != nil {
				t.Fatalf("Failed to marshal message: %v", err)
			}

			var unmarshaled Message
			err = json.Unmarshal(data, &unmarshaled)
			if err != nil {
				t.Fatalf("Failed to unmarshal message: %v", err)
			}

			if unmarshaled.Role != tt.message.Role {
				t.Errorf("Expected role '%s', got '%s'", tt.message.Role, unmarshaled.Role)
			}
			if unmarshaled.Content != tt.message.Content {
				t.Errorf("Expected content '%s', got '%s'", tt.message.Content, unmarshaled.Content)
			}
		})
	}
}
