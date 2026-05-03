package providers

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
)

func TestToChatMessageParam_ToolMessageRequiresID(t *testing.T) {
	_, err := toChatMessageParam(ai.Message{Role: "tool", Content: "result"})
	if err == nil || !strings.Contains(err.Error(), "ToolCallID") {
		t.Fatalf("expected ToolCallID required error, got %v", err)
	}
}

func TestToChatMessageParam_ToolMessageRoundTrip(t *testing.T) {
	param, err := toChatMessageParam(ai.Message{
		Role:       "tool",
		ToolCallID: "call_1",
		Name:       "read_file",
		Content:    "hello",
	})
	if err != nil {
		t.Fatalf("toChatMessageParam: %v", err)
	}
	data, err := json.Marshal(param)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"role":"tool"`) {
		t.Errorf("payload missing role=tool: %s", got)
	}
	if !strings.Contains(got, `"tool_call_id":"call_1"`) {
		t.Errorf("payload missing tool_call_id: %s", got)
	}
	if !strings.Contains(got, `"content":"hello"`) {
		t.Errorf("payload missing content: %s", got)
	}
}

func TestToChatMessageParam_AssistantWithToolCalls(t *testing.T) {
	param, err := toChatMessageParam(ai.Message{
		Role:    "assistant",
		Content: "let me check",
		ToolCalls: []ai.ToolCall{
			{ID: "call_1", Name: "read_file", Arguments: json.RawMessage(`{"path":"foo"}`)},
		},
	})
	if err != nil {
		t.Fatalf("toChatMessageParam: %v", err)
	}
	data, err := json.Marshal(param)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"role":"assistant"`) {
		t.Errorf("payload missing role=assistant: %s", got)
	}
	if !strings.Contains(got, `"tool_calls"`) {
		t.Errorf("payload missing tool_calls: %s", got)
	}
	if !strings.Contains(got, `"id":"call_1"`) {
		t.Errorf("payload missing tool call id: %s", got)
	}
	if !strings.Contains(got, `"name":"read_file"`) {
		t.Errorf("payload missing tool call name: %s", got)
	}
	if !strings.Contains(got, `\"path\":\"foo\"`) {
		t.Errorf("payload missing tool call args (escaped): %s", got)
	}
}

func TestToChatMessageParam_AssistantWithoutToolCallsUsesPlainPath(t *testing.T) {
	param, err := toChatMessageParam(ai.Message{Role: "assistant", Content: "ok"})
	if err != nil {
		t.Fatalf("toChatMessageParam: %v", err)
	}
	data, _ := json.Marshal(param)
	if strings.Contains(string(data), "tool_calls") {
		t.Errorf("plain assistant message should not include tool_calls: %s", data)
	}
}

func TestToOpenAIToolUnionParams_Empty(t *testing.T) {
	out, err := toOpenAIToolUnionParams(nil)
	if err != nil {
		t.Fatalf("toOpenAIToolUnionParams: %v", err)
	}
	if out != nil {
		t.Fatalf("expected nil for empty input, got %d", len(out))
	}
}

func TestToOpenAIToolUnionParams_RoundTrip(t *testing.T) {
	defs := []ai.ToolDefinition{
		{
			Name:        "read_file",
			Description: "reads a file",
			JSONSchema:  json.RawMessage(`{"type":"object","properties":{"path":{"type":"string"}}}`),
		},
	}
	out, err := toOpenAIToolUnionParams(defs)
	if err != nil {
		t.Fatalf("toOpenAIToolUnionParams: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("len(out) = %d, want 1", len(out))
	}
	data, err := json.Marshal(out[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got := string(data)
	if !strings.Contains(got, `"type":"function"`) {
		t.Errorf("missing type=function: %s", got)
	}
	if !strings.Contains(got, `"name":"read_file"`) {
		t.Errorf("missing function.name: %s", got)
	}
	if !strings.Contains(got, `"description":"reads a file"`) {
		t.Errorf("missing function.description: %s", got)
	}
}

func TestToOpenAIToolUnionParams_BadSchema(t *testing.T) {
	_, err := toOpenAIToolUnionParams([]ai.ToolDefinition{
		{Name: "bad", JSONSchema: json.RawMessage(`{not json`)},
	})
	if err == nil {
		t.Fatal("expected error for malformed schema")
	}
}

func TestToOpenAIToolChoice_Variants(t *testing.T) {
	cases := []struct {
		in  string
		key string // substring expected in JSON output
	}{
		{"", `"auto"`},
		{"auto", `"auto"`},
		{"none", `"none"`},
		{"required", `"required"`},
		{"my_tool", `"name":"my_tool"`},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			choice := toOpenAIToolChoice(c.in)
			data, _ := json.Marshal(choice)
			if !strings.Contains(string(data), c.key) {
				t.Errorf("toOpenAIToolChoice(%q) JSON %q missing %q", c.in, string(data), c.key)
			}
		})
	}
}

// TestOpenRouterProvider_PassesToolsAndToolChoice verifies a request with
// req.Tools set serializes them in the wire payload and triggers tool_choice.
func TestOpenRouterProvider_PassesToolsAndToolChoice(t *testing.T) {
	var gotPayload map[string]any
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		_ = json.NewDecoder(req.Body).Decode(&gotPayload)
		_ = req.Body.Close()
		// Minimal valid response.
		resp := map[string]any{
			"id": "x", "object": "chat.completion", "created": 1, "model": "test",
			"choices": []any{map[string]any{
				"index": 0, "message": map[string]any{"role": "assistant", "content": ""},
				"finish_reason": "stop",
			}},
		}
		return newJSONResponse(t, req, http.StatusOK, resp), nil
	})

	cfg := config.OpenRouterConfig{
		APIKey: "k", APIURL: "https://or.test", Model: "test",
		Temperature: 0.5, MaxTokens: 100, APITimeoutSeconds: 5,
	}
	provider, err := newOpenRouterProviderWithHTTPClient(cfg, client)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	_, err = provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "go"}},
		Tools: []ai.ToolDefinition{
			{Name: "echo", Description: "echo", JSONSchema: json.RawMessage(`{"type":"object"}`)},
		},
		ToolChoice: "auto",
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}

	if _, ok := gotPayload["tools"]; !ok {
		t.Fatalf("payload missing tools: %v", gotPayload)
	}
	if _, ok := gotPayload["tool_choice"]; !ok {
		t.Fatalf("payload missing tool_choice: %v", gotPayload)
	}
}

func TestOpenRouterProvider_OmitsToolsWhenAbsent(t *testing.T) {
	var gotPayload map[string]any
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		_ = json.NewDecoder(req.Body).Decode(&gotPayload)
		_ = req.Body.Close()
		resp := map[string]any{
			"id": "x", "object": "chat.completion", "created": 1, "model": "test",
			"choices": []any{map[string]any{
				"index": 0, "message": map[string]any{"role": "assistant", "content": "hi"},
				"finish_reason": "stop",
			}},
		}
		return newJSONResponse(t, req, http.StatusOK, resp), nil
	})

	cfg := config.OpenRouterConfig{
		APIKey: "k", APIURL: "https://or.test", Model: "test",
		Temperature: 0.5, MaxTokens: 100, APITimeoutSeconds: 5,
	}
	provider, err := newOpenRouterProviderWithHTTPClient(cfg, client)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	_, err = provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "go"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion: %v", err)
	}

	if _, ok := gotPayload["tools"]; ok {
		t.Errorf("expected no 'tools' field when none requested; got %v", gotPayload["tools"])
	}
	if _, ok := gotPayload["tool_choice"]; ok {
		t.Errorf("expected no 'tool_choice' field when no tools; got %v", gotPayload["tool_choice"])
	}
}

// TestOpenRouterProvider_StreamAccumulatesToolCalls verifies that streaming
// tool-call deltas (the OpenAI SSE pattern of partial id/name/arguments per
// chunk) are assembled into a complete ToolCall list at end-of-stream.
func TestOpenRouterProvider_StreamAccumulatesToolCalls(t *testing.T) {
	client := newTestClient(func(req *http.Request) (*http.Response, error) {
		// Three chunks: first declares the tool call (id+name+partial args),
		// second adds more args, third sets finish_reason=tool_calls.
		chunks := []map[string]any{
			{
				"id": "s", "object": "chat.completion.chunk", "created": 1, "model": "m",
				"choices": []any{map[string]any{
					"index": 0,
					"delta": map[string]any{
						"role": "assistant",
						"tool_calls": []any{map[string]any{
							"index": 0,
							"id":    "call_a",
							"type":  "function",
							"function": map[string]any{
								"name":      "echo",
								"arguments": `{"x":`,
							},
						}},
					},
					"finish_reason": "",
				}},
			},
			{
				"id": "s", "object": "chat.completion.chunk", "created": 1, "model": "m",
				"choices": []any{map[string]any{
					"index": 0,
					"delta": map[string]any{
						"tool_calls": []any{map[string]any{
							"index": 0,
							"function": map[string]any{
								"arguments": `1}`,
							},
						}},
					},
					"finish_reason": "",
				}},
			},
			{
				"id": "s", "object": "chat.completion.chunk", "created": 1, "model": "m",
				"choices": []any{map[string]any{
					"index":         0,
					"delta":         map[string]any{},
					"finish_reason": "tool_calls",
				}},
			},
		}

		var sb strings.Builder
		for _, c := range chunks {
			b, _ := json.Marshal(c)
			sb.WriteString("data: ")
			sb.Write(b)
			sb.WriteString("\n\n")
		}
		sb.WriteString("data: [DONE]\n\n")
		return newHTTPResponse(req, http.StatusOK, "text/event-stream", []byte(sb.String())), nil
	})

	cfg := config.OpenRouterConfig{
		APIKey: "k", APIURL: "https://or.test", Model: "test",
		Temperature: 0.5, MaxTokens: 100, APITimeoutSeconds: 5,
	}
	provider, err := newOpenRouterProviderWithHTTPClient(cfg, client)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	stream, err := provider.CreateChatCompletionStream(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "go"}},
		Tools:    []ai.ToolDefinition{{Name: "echo", JSONSchema: json.RawMessage(`{}`)}},
	})
	if err != nil {
		t.Fatalf("stream: %v", err)
	}
	// Drain text deltas (none expected here).
	for stream.Next() {
		t.Logf("unexpected text delta: %q", stream.Content())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("stream err: %v", err)
	}

	calls := stream.ToolCalls()
	if len(calls) != 1 {
		t.Fatalf("ToolCalls() = %d, want 1", len(calls))
	}
	if calls[0].ID != "call_a" {
		t.Errorf("call.ID = %q, want call_a", calls[0].ID)
	}
	if calls[0].Name != "echo" {
		t.Errorf("call.Name = %q, want echo", calls[0].Name)
	}
	if string(calls[0].Arguments) != `{"x":1}` {
		t.Errorf("call.Arguments = %q, want %q", calls[0].Arguments, `{"x":1}`)
	}
	if stream.StopReason() != "tool_calls" {
		t.Errorf("StopReason() = %q, want tool_calls", stream.StopReason())
	}
}

func TestOpenRouterProvider_CapabilitiesReportsTools(t *testing.T) {
	cfg := config.OpenRouterConfig{
		APIKey: "k", APIURL: "https://or.test", Model: "test",
		Temperature: 0.5, MaxTokens: 100, APITimeoutSeconds: 5,
	}
	provider, err := newOpenRouterProviderWithHTTPClient(cfg, nil)
	if err != nil {
		t.Fatalf("provider: %v", err)
	}
	caps := provider.Capabilities()
	if !caps.Tools {
		t.Error("expected Capabilities().Tools = true")
	}
	if !caps.Streaming {
		t.Error("expected Capabilities().Streaming = true")
	}
}
