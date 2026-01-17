package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.LLMProvider != "openrouter" {
		t.Errorf("Expected LLMProvider 'openrouter', got %q", cfg.LLMProvider)
	}

	if cfg.OpenRouter.Model != "google/gemini-3.0-flash" {
		t.Errorf("Expected model 'google/gemini-3.0-flash', got %q", cfg.OpenRouter.Model)
	}

	if cfg.OpenRouter.APIURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected API URL 'https://openrouter.ai/api/v1', got %q", cfg.OpenRouter.APIURL)
	}

	if cfg.BufferSize != 2000 {
		t.Errorf("Expected BufferSize 2000, got %d", cfg.BufferSize)
	}

	if cfg.ContextWindow != 1000 {
		t.Errorf("Expected ContextWindow 1000, got %d", cfg.ContextWindow)
	}
}

func TestLoad_CreateDefault(t *testing.T) {
	// Use temp directory
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, ".wtf_cli", "config.json")

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	// Should be default config
	if cfg.BufferSize != 2000 {
		t.Errorf("Expected default BufferSize 2000, got %d", cfg.BufferSize)
	}

	// File should exist now
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}
}

func TestLoad_ExistingConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Create initial config
	initialCfg := Default()
	initialCfg.BufferSize = 5000
	if err := Save(configPath, initialCfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Load it back
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.BufferSize != 5000 {
		t.Errorf("Expected BufferSize 5000, got %d", cfg.BufferSize)
	}
}

func TestLoad_MigrationDefaults(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Missing api_url and llm_provider, explicit temperature 0 should be preserved
	raw := `{
  "openrouter": {
    "api_key": "test-key",
    "model": "test-model",
    "temperature": 0
  },
  "buffer_size": 4000,
  "context_window": 900
}`
	if err := os.WriteFile(configPath, []byte(raw), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.LLMProvider != "openrouter" {
		t.Errorf("Expected LLMProvider 'openrouter', got %q", cfg.LLMProvider)
	}

	if cfg.OpenRouter.APIURL != "https://openrouter.ai/api/v1" {
		t.Errorf("Expected API URL default, got %q", cfg.OpenRouter.APIURL)
	}

	if cfg.OpenRouter.Temperature != 0 {
		t.Errorf("Expected temperature 0, got %f", cfg.OpenRouter.Temperature)
	}
}

func TestLoad_CorruptedJSON(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	// Write invalid JSON
	if err := os.WriteFile(configPath, []byte("{invalid json}"), 0600); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Should return error
	_, err := Load(configPath)
	if err == nil {
		t.Error("Expected error for corrupted JSON, got nil")
	}
}

func TestSave(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	cfg := Default()
	cfg.BufferSize = 3000

	if err := Save(configPath, cfg); err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify file was created
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("Config file was not created")
	}

	// Load and verify
	loadedCfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() after Save() failed: %v", err)
	}

	if loadedCfg.BufferSize != 3000 {
		t.Errorf("Expected BufferSize 3000, got %d", loadedCfg.BufferSize)
	}
}

func TestValidate_Success(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test-key"

	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() failed on valid config: %v", err)
	}
}

func TestValidate_MissingAPIKey(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for missing API key, got nil")
	}
}

func TestValidate_MissingAPIURL(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.OpenRouter.APIURL = ""

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for missing API URL, got nil")
	}
}

func TestValidate_InvalidAPIURL(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.OpenRouter.APIURL = "not a url"

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for invalid API URL, got nil")
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.LogLevel = "verbose"

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for invalid log level, got nil")
	}
}

func TestValidate_InvalidLogFormat(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.LogFormat = "xml"

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for invalid log format, got nil")
	}
}

func TestValidate_MissingModel(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.OpenRouter.Model = "   "

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for missing model, got nil")
	}
}

func TestValidate_InvalidTemperature(t *testing.T) {
	tests := []struct {
		temp  float64
		valid bool
	}{
		{-0.1, false},
		{0.0, true},
		{0.7, true},
		{2.0, true},
		{2.1, false},
	}

	for _, tt := range tests {
		cfg := Default()
		cfg.OpenRouter.APIKey = "test"
		cfg.OpenRouter.Temperature = tt.temp

		err := cfg.Validate()
		if tt.valid && err != nil {
			t.Errorf("Temperature %f should be valid, got error: %v", tt.temp, err)
		}
		if !tt.valid && err == nil {
			t.Errorf("Temperature %f should be invalid, got no error", tt.temp)
		}
	}
}

func TestValidate_InvalidBufferSize(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.BufferSize = -100

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for negative buffer size, got nil")
	}
}

func TestValidate_InvalidProvider(t *testing.T) {
	cfg := Default()
	cfg.LLMProvider = "unsupported"

	err := cfg.Validate()
	if err == nil {
		t.Error("Expected error for unsupported provider, got nil")
	}
}

func TestGetConfigPath(t *testing.T) {
	path := GetConfigPath()

	if path == "" {
		t.Error("GetConfigPath() returned empty string")
	}

	// Should contain .wtf_cli
	if !contains(path, ".wtf_cli") {
		t.Errorf("Expected path to contain '.wtf_cli', got %q", path)
	}
}

func contains(s, substr string) bool {
	return filepath.Base(filepath.Dir(s)) == ".wtf_cli" || filepath.Dir(s) == ".wtf_cli"
}
