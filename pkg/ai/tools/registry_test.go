package tools

import (
	"context"
	"encoding/json"
	"testing"

	"wtf_cli/pkg/ai"
)

type fakeTool struct {
	name string
	desc string
}

func (f *fakeTool) Name() string { return f.name }
func (f *fakeTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{Name: f.name, Description: f.desc, JSONSchema: json.RawMessage(`{}`)}
}
func (f *fakeTool) Execute(_ context.Context, _ json.RawMessage) (Result, error) {
	return Result{Content: "ok"}, nil
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	tool := &fakeTool{name: "ping", desc: "ping"}

	r.Register(tool)

	got, ok := r.Get("ping")
	if !ok {
		t.Fatal("expected to find registered tool")
	}
	if got.Name() != "ping" {
		t.Fatalf("got name %q, want %q", got.Name(), "ping")
	}
	if r.Len() != 1 {
		t.Fatalf("got Len=%d, want 1", r.Len())
	}
}

func TestRegistry_GetUnknownReturnsFalse(t *testing.T) {
	r := NewRegistry()
	if _, ok := r.Get("missing"); ok {
		t.Fatal("expected ok=false for unknown tool")
	}
}

func TestRegistry_DefinitionsSortedByName(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "zebra", desc: "z"})
	r.Register(&fakeTool{name: "alpha", desc: "a"})
	r.Register(&fakeTool{name: "mango", desc: "m"})

	defs := r.Definitions()
	if len(defs) != 3 {
		t.Fatalf("got %d definitions, want 3", len(defs))
	}
	want := []string{"alpha", "mango", "zebra"}
	for i, d := range defs {
		if d.Name != want[i] {
			t.Errorf("defs[%d].Name = %q, want %q", i, d.Name, want[i])
		}
	}
}

func TestRegistry_DefinitionsEmpty(t *testing.T) {
	r := NewRegistry()
	if defs := r.Definitions(); defs != nil {
		t.Fatalf("got %v, want nil", defs)
	}
}

func TestRegistry_DuplicateRegistrationPanics(t *testing.T) {
	r := NewRegistry()
	r.Register(&fakeTool{name: "dup"})

	defer func() {
		if recover() == nil {
			t.Fatal("expected panic on duplicate registration")
		}
	}()
	r.Register(&fakeTool{name: "dup"})
}
