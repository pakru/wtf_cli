package config

import (
	"os"
	"path/filepath"
	"testing"

	"wtf_cli/logger"
)

func TestLoadConfig(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")
	
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
	if cfg.OpenRouter.Model != "google/gemma-3-27b" {
		t.Errorf("Expected default model to be 'google/gemma-3-27b', got '%s'", cfg.OpenRouter.Model)
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
			APIKey:      "test-api-key",
			Model:       "google/gemma-3-27b",
			Temperature: 0.7,
			MaxTokens:   1000,
			APITimeoutSeconds: 30,
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

func TestEnvironmentVariableOverrides(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("error")
	
	// Set environment variables
	os.Setenv("WTF_API_KEY", "env-api-key")
	os.Setenv("WTF_MODEL", "env-model")
	os.Setenv("WTF_TEMPERATURE", "0.8")
	os.Setenv("WTF_MAX_TOKENS", "2000")
	os.Setenv("WTF_DRY_RUN", "true")
	defer func() {
		os.Unsetenv("WTF_API_KEY")
		os.Unsetenv("WTF_MODEL")
		os.Unsetenv("WTF_TEMPERATURE")
		os.Unsetenv("WTF_MAX_TOKENS")
		os.Unsetenv("WTF_DRY_RUN")
	}()

	cfg := DefaultConfig()
	// Apply environment overrides manually since DefaultConfig doesn't do it
	cfg = applyEnvironmentOverrides(cfg)

	if cfg.OpenRouter.APIKey != "env-api-key" {
		t.Errorf("Expected API key from env var, got '%s'", cfg.OpenRouter.APIKey)
	}
	if cfg.OpenRouter.Model != "env-model" {
		t.Errorf("Expected model from env var, got '%s'", cfg.OpenRouter.Model)
	}
	if cfg.OpenRouter.Temperature != 0.8 {
		t.Errorf("Expected temperature from env var, got %f", cfg.OpenRouter.Temperature)
	}
	if cfg.OpenRouter.MaxTokens != 2000 {
		t.Errorf("Expected max tokens from env var, got %d", cfg.OpenRouter.MaxTokens)
	}
	// Debug field removed - now controlled by log_level
	if !cfg.DryRun {
		t.Error("Expected dry run to be true from env var")
	}
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config",
			config: Config{
				LLMProvider: "openrouter",
				OpenRouter: OpenRouterConfig{
					APIKey:      "valid-key",
					Model:       "google/gemma-3-27b",
					Temperature: 0.7,
					MaxTokens:   1000,
					APITimeoutSeconds: 30,
				},
			},
			wantErr: false,
		},
		{
			name: "missing API key",
			config: Config{
				LLMProvider: "openrouter",
				OpenRouter: OpenRouterConfig{
					Model:       "google/gemma-3-27b",
					Temperature: 0.7,
					MaxTokens:   1000,
					APITimeoutSeconds: 30,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid temperature too low",
			config: Config{
				LLMProvider: "openrouter",
				OpenRouter: OpenRouterConfig{
					APIKey:      "valid-key",
					Model:       "google/gemma-3-27b",
					Temperature: -0.1,
					MaxTokens:   1000,
					APITimeoutSeconds: 30,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid temperature too high",
			config: Config{
				LLMProvider: "openrouter",
				OpenRouter: OpenRouterConfig{
					APIKey:      "valid-key",
					Model:       "google/gemma-3-27b",
					Temperature: 2.1,
					MaxTokens:   1000,
					APITimeoutSeconds: 30,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid max tokens",
			config: Config{
				LLMProvider: "openrouter",
				OpenRouter: OpenRouterConfig{
					APIKey:      "valid-key",
					Model:       "google/gemma-3-27b",
					Temperature: 0.7,
					MaxTokens:   -1,
					APITimeoutSeconds: 30,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
