package ai

import (
	"testing"
)

func TestNewRegistry(t *testing.T) {
	r := NewRegistry()
	if r == nil {
		t.Fatal("expected registry, got nil")
	}
	if r.factories == nil {
		t.Fatal("expected factories map, got nil")
	}
	if r.info == nil {
		t.Fatal("expected info map, got nil")
	}
}

func TestRegistry_Register(t *testing.T) {
	r := NewRegistry()

	info := ProviderInfo{
		Type:        "test-provider",
		Name:        "Test Provider",
		Description: "A test provider",
		AuthMethod:  "api_key",
		RequiresKey: true,
	}

	factory := func(cfg ProviderConfig) (Provider, error) {
		return nil, nil
	}

	r.Register(info, factory)

	if !r.IsRegistered("test-provider") {
		t.Fatal("expected provider to be registered")
	}

	gotInfo, ok := r.GetProviderInfo("test-provider")
	if !ok {
		t.Fatal("expected to find provider info")
	}
	if gotInfo.Name != "Test Provider" {
		t.Fatalf("expected name 'Test Provider', got %q", gotInfo.Name)
	}
}

func TestRegistry_GetProvider_UnknownType(t *testing.T) {
	r := NewRegistry()

	_, err := r.GetProvider(ProviderConfig{Type: "unknown"})
	if err == nil {
		t.Fatal("expected error for unknown provider type")
	}
}

func TestRegistry_ListProviders(t *testing.T) {
	r := NewRegistry()

	info1 := ProviderInfo{Type: "provider1", Name: "Provider 1"}
	info2 := ProviderInfo{Type: "provider2", Name: "Provider 2"}

	r.Register(info1, func(cfg ProviderConfig) (Provider, error) { return nil, nil })
	r.Register(info2, func(cfg ProviderConfig) (Provider, error) { return nil, nil })

	providers := r.ListProviders()
	if len(providers) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(providers))
	}
}

func TestRegistry_IsRegistered(t *testing.T) {
	r := NewRegistry()

	if r.IsRegistered("nonexistent") {
		t.Fatal("expected false for nonexistent provider")
	}

	r.Register(ProviderInfo{Type: "exists"}, func(cfg ProviderConfig) (Provider, error) { return nil, nil })

	if !r.IsRegistered("exists") {
		t.Fatal("expected true for registered provider")
	}
}

func TestSupportedProviders(t *testing.T) {
	providers := SupportedProviders()
	if len(providers) != 4 {
		t.Fatalf("expected 4 supported providers, got %d", len(providers))
	}

	expected := map[ProviderType]bool{
		ProviderOpenRouter: true,
		ProviderOpenAI:     true,
		ProviderCopilot:    true,
		ProviderAnthropic:  true,
	}

	for _, p := range providers {
		if !expected[p] {
			t.Fatalf("unexpected provider type: %s", p)
		}
	}
}

func TestValidateProviderType(t *testing.T) {
	tests := []struct {
		input    string
		wantType ProviderType
		wantOK   bool
	}{
		{"openrouter", ProviderOpenRouter, true},
		{"openai", ProviderOpenAI, true},
		{"copilot", ProviderCopilot, true},
		{"anthropic", ProviderAnthropic, true},
		{"invalid", "", false},
		{"", "", false},
		{"OPENROUTER", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotType, gotOK := ValidateProviderType(tt.input)
			if gotType != tt.wantType {
				t.Errorf("ValidateProviderType(%q) type = %q, want %q", tt.input, gotType, tt.wantType)
			}
			if gotOK != tt.wantOK {
				t.Errorf("ValidateProviderType(%q) ok = %v, want %v", tt.input, gotOK, tt.wantOK)
			}
		})
	}
}

func TestProviderTypeConstants(t *testing.T) {
	if ProviderOpenRouter != "openrouter" {
		t.Errorf("ProviderOpenRouter = %q, want 'openrouter'", ProviderOpenRouter)
	}
	if ProviderOpenAI != "openai" {
		t.Errorf("ProviderOpenAI = %q, want 'openai'", ProviderOpenAI)
	}
	if ProviderCopilot != "copilot" {
		t.Errorf("ProviderCopilot = %q, want 'copilot'", ProviderCopilot)
	}
	if ProviderAnthropic != "anthropic" {
		t.Errorf("ProviderAnthropic = %q, want 'anthropic'", ProviderAnthropic)
	}
}
