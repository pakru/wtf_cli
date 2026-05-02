package ai

import "context"

// Message represents a single chat message for LLM requests.
type Message struct {
	Role    string
	Content string

	// ToolCalls is set on assistant messages when the model requested tool invocations.
	ToolCalls []ToolCall

	// ToolCallID links a role="tool" message back to the assistant's ToolCall.ID.
	ToolCallID string

	// Name is the tool name on role="tool" messages (required by some providers, e.g. OpenAI).
	Name string
}

// ChatRequest defines the input to an LLM chat completion.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature *float64
	MaxTokens   *int

	// Tools, when non-empty, are advertised to the model. Providers that do not
	// support tool calling (Capabilities().Tools == false) must ignore this field.
	Tools []ToolDefinition

	// ToolChoice controls tool selection. Empty string means "auto" when Tools is
	// non-empty. Other values: "none", "auto", or a specific tool name.
	ToolChoice string
}

// ChatResponse is a normalized response from an LLM.
type ChatResponse struct {
	Content    string
	Model      string
	ToolCalls  []ToolCall
	StopReason string
}

// ChatStream exposes a streaming response interface.
//
// Iteration contract:
//   - Next() advances to the next text delta. It returns false when the stream
//     is exhausted or after an error.
//   - Content() returns the most recent text delta (only valid after Next()
//     returned true).
//   - ToolCalls() returns the assembled list of tool calls accumulated during
//     the stream. It is only valid after Next() returns false. Streaming
//     implementations MUST accumulate tool-call deltas internally and only
//     expose the assembled list at end-of-stream — never partial.
//   - StopReason() returns the provider's stop reason (e.g. "stop",
//     "tool_calls", "length"), if known. Only valid after Next() returns false.
type ChatStream interface {
	Next() bool
	Content() string
	Err() error
	Close() error
	ToolCalls() []ToolCall
	StopReason() string
}

// Provider defines the LLM interface used by the app.
type Provider interface {
	CreateChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error)
	CreateChatCompletionStream(ctx context.Context, req ChatRequest) (ChatStream, error)
	Capabilities() ProviderCapabilities
}
