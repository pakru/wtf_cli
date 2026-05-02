// Package tools provides the agentic tool registry and tool implementations
// (read_file, etc.) callable by the LLM during a /explain or /chat run.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"sync"

	"wtf_cli/pkg/ai"
)

// Result is what a tool returns to the agent loop.
//
// IsError discriminates *recoverable* failures (the model should see the error
// message and retry or recover) from hard errors (returned via the Go error
// path; the agent loop must abort). Bad arguments, missing files, and policy
// rejections are recoverable. Context cancellation is not.
type Result struct {
	Content string
	IsError bool
}

// Tool is implemented by anything callable from the agent loop.
//
// Execute receives the raw JSON arguments emitted by the model. The
// implementation is responsible for decoding them, validating, and returning a
// Result. Returning a non-nil error aborts the loop.
type Tool interface {
	Name() string
	Definition() ai.ToolDefinition
	Execute(ctx context.Context, args json.RawMessage) (Result, error)
}

// Registry holds a set of tools keyed by name.
type Registry struct {
	mu    sync.RWMutex
	tools map[string]Tool
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{tools: make(map[string]Tool)}
}

// Register adds a tool to the registry. It panics if a tool with the same name
// is already registered — registration happens at startup, not at runtime, so
// duplicates indicate a programming error.
func (r *Registry) Register(t Tool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := t.Name()
	if _, exists := r.tools[name]; exists {
		panic(fmt.Sprintf("tools: duplicate registration for %q", name))
	}
	r.tools[name] = t
}

// Get returns the tool registered under the given name.
func (r *Registry) Get(name string) (Tool, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.tools[name]
	return t, ok
}

// Definitions returns the tool definitions to advertise to the LLM, in
// alphabetical order by name for deterministic prompt construction.
func (r *Registry) Definitions() []ai.ToolDefinition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(r.tools) == 0 {
		return nil
	}
	names := make([]string, 0, len(r.tools))
	for name := range r.tools {
		names = append(names, name)
	}
	sort.Strings(names)
	defs := make([]ai.ToolDefinition, 0, len(names))
	for _, name := range names {
		defs = append(defs, r.tools[name].Definition())
	}
	return defs
}

// Len returns the number of registered tools.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.tools)
}
