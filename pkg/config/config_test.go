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

	if cfg.Providers.Google.Model != "gemini-3-flash-preview" {
		t.Errorf("Expected Google model 'gemini-3-flash-preview', got %q", cfg.Providers.Google.Model)
	}

	if cfg.BufferSize != 2000 {
		t.Errorf("Expected BufferSize 2000, got %d", cfg.BufferSize)
	}

	if cfg.ContextWindow != 1000 {
		t.Errorf("Expected ContextWindow 1000, got %d", cfg.ContextWindow)
	}

	if !cfg.UpdateCheck.Enabled {
		t.Error("Expected update check enabled by default")
	}
	if cfg.UpdateCheck.IntervalHours != 1 {
		t.Errorf("Expected update check interval 1h, got %d", cfg.UpdateCheck.IntervalHours)
	}
}

func TestDefault_AgentTools(t *testing.T) {
	cfg := Default()

	if !cfg.Agent.Tools.ReadFile.Enabled {
		t.Error("Expected read_file enabled by default")
	}
	if cfg.Agent.Tools.ReadFile.MaxLines != defaultReadFileMaxLines {
		t.Errorf("Expected read_file max_lines %d, got %d", defaultReadFileMaxLines, cfg.Agent.Tools.ReadFile.MaxLines)
	}
	if cfg.Agent.Tools.ReadFile.MaxBytes != defaultReadFileMaxBytes {
		t.Errorf("Expected read_file max_bytes %d, got %d", defaultReadFileMaxBytes, cfg.Agent.Tools.ReadFile.MaxBytes)
	}

	if !cfg.Agent.Tools.ListDirectory.Enabled {
		t.Error("Expected list_directory enabled by default")
	}
	if cfg.Agent.Tools.ListDirectory.MaxEntries != defaultListDirectoryMaxEntries {
		t.Errorf("Expected list_directory max_entries %d, got %d", defaultListDirectoryMaxEntries, cfg.Agent.Tools.ListDirectory.MaxEntries)
	}
	if cfg.Agent.Tools.ListDirectory.MaxBytes != defaultListDirectoryMaxBytes {
		t.Errorf("Expected list_directory max_bytes %d, got %d", defaultListDirectoryMaxBytes, cfg.Agent.Tools.ListDirectory.MaxBytes)
	}
}

// Each of these covers one cell of the agent.tools presence matrix: whether
// "agent" or "tools" is present at all, and whether each tool block is
// present, partially present, or absent. applyAgentToolsDefaults must default
// each tool independently — a config specifying only one tool must not zero
// out the other's fields.
func TestLoad_AgentToolsPresenceMatrix(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want func(t *testing.T, cfg Config)
	}{
		{
			name: "agent key absent entirely",
			raw:  `{"buffer_size": 2000, "context_window": 1000}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.Tools.ReadFile.MaxLines != defaultReadFileMaxLines {
					t.Errorf("read_file.max_lines = %d, want default %d", cfg.Agent.Tools.ReadFile.MaxLines, defaultReadFileMaxLines)
				}
				if cfg.Agent.Tools.ListDirectory.MaxEntries != defaultListDirectoryMaxEntries {
					t.Errorf("list_directory.max_entries = %d, want default %d", cfg.Agent.Tools.ListDirectory.MaxEntries, defaultListDirectoryMaxEntries)
				}
			},
		},
		{
			name: "tools key absent under agent",
			raw:  `{"agent": {"max_iterations": 3}}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.MaxIterations != 3 {
					t.Errorf("max_iterations = %d, want 3", cfg.Agent.MaxIterations)
				}
				if !cfg.Agent.Tools.ReadFile.Enabled || cfg.Agent.Tools.ReadFile.MaxLines != defaultReadFileMaxLines {
					t.Errorf("expected read_file defaults, got %+v", cfg.Agent.Tools.ReadFile)
				}
				if !cfg.Agent.Tools.ListDirectory.Enabled || cfg.Agent.Tools.ListDirectory.MaxEntries != defaultListDirectoryMaxEntries {
					t.Errorf("expected list_directory defaults, got %+v", cfg.Agent.Tools.ListDirectory)
				}
			},
		},
		{
			name: "only read_file present",
			raw:  `{"agent": {"tools": {"read_file": {"enabled": true, "max_lines": 42, "max_bytes": 4096}}}}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.Tools.ReadFile.MaxLines != 42 || cfg.Agent.Tools.ReadFile.MaxBytes != 4096 {
					t.Errorf("read_file config not applied: %+v", cfg.Agent.Tools.ReadFile)
				}
				if !cfg.Agent.Tools.ListDirectory.Enabled || cfg.Agent.Tools.ListDirectory.MaxEntries != defaultListDirectoryMaxEntries || cfg.Agent.Tools.ListDirectory.MaxBytes != defaultListDirectoryMaxBytes {
					t.Errorf("expected list_directory to fall back to full defaults, got %+v", cfg.Agent.Tools.ListDirectory)
				}
			},
		},
		{
			name: "only list_directory present",
			raw:  `{"agent": {"tools": {"list_directory": {"enabled": true, "max_entries": 10, "max_bytes": 2048}}}}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.Tools.ListDirectory.MaxEntries != 10 || cfg.Agent.Tools.ListDirectory.MaxBytes != 2048 {
					t.Errorf("list_directory config not applied: %+v", cfg.Agent.Tools.ListDirectory)
				}
				if !cfg.Agent.Tools.ReadFile.Enabled || cfg.Agent.Tools.ReadFile.MaxLines != defaultReadFileMaxLines || cfg.Agent.Tools.ReadFile.MaxBytes != defaultReadFileMaxBytes {
					t.Errorf("expected read_file to fall back to full defaults, got %+v", cfg.Agent.Tools.ReadFile)
				}
			},
		},
		{
			name: "list_directory partial fields fill remaining from defaults",
			raw:  `{"agent": {"tools": {"list_directory": {"max_entries": 25}}}}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.Tools.ListDirectory.MaxEntries != 25 {
					t.Errorf("max_entries = %d, want 25", cfg.Agent.Tools.ListDirectory.MaxEntries)
				}
				if cfg.Agent.Tools.ListDirectory.MaxBytes != defaultListDirectoryMaxBytes {
					t.Errorf("max_bytes = %d, want default %d", cfg.Agent.Tools.ListDirectory.MaxBytes, defaultListDirectoryMaxBytes)
				}
				if !cfg.Agent.Tools.ListDirectory.Enabled {
					t.Error("expected enabled to default to true when omitted")
				}
			},
		},
		{
			name: "list_directory explicitly disabled",
			raw:  `{"agent": {"tools": {"list_directory": {"enabled": false}}}}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.Tools.ListDirectory.Enabled {
					t.Error("expected list_directory to remain disabled")
				}
				// Caps still default even when disabled.
				if cfg.Agent.Tools.ListDirectory.MaxEntries != defaultListDirectoryMaxEntries {
					t.Errorf("max_entries = %d, want default %d", cfg.Agent.Tools.ListDirectory.MaxEntries, defaultListDirectoryMaxEntries)
				}
			},
		},
		{
			name: "list_directory non-positive caps replaced by defaults",
			raw:  `{"agent": {"tools": {"list_directory": {"max_entries": 0, "max_bytes": -5}}}}`,
			want: func(t *testing.T, cfg Config) {
				if cfg.Agent.Tools.ListDirectory.MaxEntries != defaultListDirectoryMaxEntries {
					t.Errorf("max_entries = %d, want default %d", cfg.Agent.Tools.ListDirectory.MaxEntries, defaultListDirectoryMaxEntries)
				}
				if cfg.Agent.Tools.ListDirectory.MaxBytes != defaultListDirectoryMaxBytes {
					t.Errorf("max_bytes = %d, want default %d", cfg.Agent.Tools.ListDirectory.MaxBytes, defaultListDirectoryMaxBytes)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.json")
			if err := os.WriteFile(configPath, []byte(tt.raw), 0600); err != nil {
				t.Fatalf("failed to write test config: %v", err)
			}

			cfg, err := Load(configPath)
			if err != nil {
				t.Fatalf("Load() failed: %v", err)
			}
			tt.want(t, cfg)
		})
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

func TestValidate_GoogleSuccess(t *testing.T) {
	cfg := Default()
	cfg.LLMProvider = "google"
	cfg.Providers.Google.APIKey = "test-google-key"

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() failed for valid Google config: %v", err)
	}
}

func TestValidate_GoogleMissingAPIKey(t *testing.T) {
	cfg := Default()
	cfg.LLMProvider = "google"
	cfg.Providers.Google.APIKey = ""

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for missing Google API key, got nil")
	}
}

func TestLoad_GoogleDefaultsApplied(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.json")

	raw := `{
  "llm_provider": "google",
  "providers": {
    "google": {
      "api_key": "test-key"
    }
  }
}`
	if err := os.WriteFile(configPath, []byte(raw), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	if cfg.Providers.Google.Model != "gemini-3-flash-preview" {
		t.Errorf("Expected default Google model, got %q", cfg.Providers.Google.Model)
	}
	if cfg.Providers.Google.Temperature != 0.7 {
		t.Errorf("Expected default Google temperature 0.7, got %f", cfg.Providers.Google.Temperature)
	}
	if cfg.Providers.Google.MaxTokens != 8192 {
		t.Errorf("Expected default Google max_tokens 8192, got %d", cfg.Providers.Google.MaxTokens)
	}
	if cfg.Providers.Google.APITimeoutSeconds != 60 {
		t.Errorf("Expected default Google timeout 60, got %d", cfg.Providers.Google.APITimeoutSeconds)
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

func TestValidate_InvalidUpdateCheckInterval(t *testing.T) {
	cfg := Default()
	cfg.OpenRouter.APIKey = "test"
	cfg.UpdateCheck.IntervalHours = 0

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected error for non-positive update_check.interval_hours, got nil")
	}
}
