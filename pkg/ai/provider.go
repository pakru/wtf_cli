package ai

import "context"

// Message represents a single chat message for LLM requests.
type Message struct {
	Role    string
	Content string
}

// ChatRequest defines the input to an LLM chat completion.
type ChatRequest struct {
	Model       string
	Messages    []Message
	Temperature *float64
	MaxTokens   *int
}

// ChatResponse is a normalized response from an LLM.
type ChatResponse struct {
	Content string
	Model   string
}

// ChatStream exposes a streaming response interface.
type ChatStream interface {
	Next() bool
	Content() string
	Err() error
	Close() error
}

// Provider defines the LLM interface used by the app.
type Provider interface {
	CreateChatCompletion(ctx context.Context, req ChatRequest) (ChatResponse, error)
	CreateChatCompletionStream(ctx context.Context, req ChatRequest) (ChatStream, error)
}
