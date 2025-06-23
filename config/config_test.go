package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "wtf-config-test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test config path
	configPath := filepath.Join(tempDir, "config.json")

	// Test loading non-existent config (should create default)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify default values
	if cfg.LLMProvider != "openrouter" {
		t.Errorf("Expected default LLM provider to be 'openrouter', got '%s'", cfg.LLMProvider)
	}
	if cfg.OpenRouter.Model != "openai/gpt-4o" {
		t.Errorf("Expected default model to be 'openai/gpt-4o', got '%s'", cfg.OpenRouter.Model)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Modify and save config
	cfg.LLMProvider = "openrouter"
	cfg.OpenRouter.APIKey = "test-api-key"
	cfg.OpenRouter.Model = "anthropic/claude-2"
	if err := SaveConfig(configPath, cfg); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Load the modified config
	loadedCfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("Failed to load modified config: %v", err)
	}

	// Verify loaded values
	if loadedCfg.LLMProvider != "openrouter" {
		t.Errorf("Expected LLM provider to be 'openrouter', got '%s'", loadedCfg.LLMProvider)
	}
	if loadedCfg.OpenRouter.APIKey != "test-api-key" {
		t.Errorf("Expected API key to be 'test-api-key', got '%s'", loadedCfg.OpenRouter.APIKey)
	}
	if loadedCfg.OpenRouter.Model != "anthropic/claude-2" {
		t.Errorf("Expected model to be 'anthropic/claude-2', got '%s'", loadedCfg.OpenRouter.Model)
	}
}

func TestValidate(t *testing.T) {
	// Test valid config
	validCfg := Config{
		LLMProvider: "openrouter",
		OpenRouter: OpenRouterConfig{
			APIKey: "test-api-key",
			Model:  "openai/gpt-4o",
		},
	}
	if err := validCfg.Validate(); err != nil {
		t.Errorf("Valid config failed validation: %v", err)
	}

	// Test invalid config (missing API key)
	invalidCfg := DefaultConfig() // Default has empty API key
	if err := invalidCfg.Validate(); err == nil {
		t.Error("Invalid config passed validation, expected error")
	}
}
