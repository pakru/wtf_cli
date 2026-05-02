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
}

// ProviderCapabilities advertises what optional features a provider supports.
type ProviderCapabilities struct {
	Tools     bool
	Streaming bool
}
