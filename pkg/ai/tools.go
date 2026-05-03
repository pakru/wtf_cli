package ai

import "encoding/json"

// ToolDefinition describes a tool that the LLM may call.
type ToolDefinition struct {
	Name        string
	Description string
	JSONSchema  json.RawMessage
}

// ToolCall is a request from the LLM to invoke a tool.
type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage

	// ThoughtSignature is an opaque blob returned by Google's thinking models
	// alongside function-call parts. It must be echoed back verbatim in the
	// conversation history on the next API call; other providers leave it nil.
	ThoughtSignature []byte
}

// ProviderCapabilities advertises what optional features a provider supports.
type ProviderCapabilities struct {
	Tools     bool
	Streaming bool
}
