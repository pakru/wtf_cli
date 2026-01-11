package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	LLMProvider   string           `json:"llm_provider"`
	OpenRouter    OpenRouterConfig `json:"openrouter"`
	BufferSize    int              `json:"buffer_size"`
	ContextWindow int              `json:"context_window"`
	StatusBar     StatusBarConfig  `json:"status_bar"`
	DryRun        bool             `json:"dry_run"`
	LogLevel      string           `json:"log_level"`
}

// OpenRouterConfig holds the OpenRouter API configuration
type OpenRouterConfig struct {
	APIKey            string  `json:"api_key"`
	Model             string  `json:"model"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"max_tokens"`
	APITimeoutSeconds int     `json:"api_timeout_seconds"`
}

// StatusBarConfig holds status bar UI configuration
type StatusBarConfig struct {
	Position string `json:"position"` // "bottom" (hardcoded for now)
	Colors   string `json:"colors"`   // "auto"
}

// Default returns a configuration with default values
func Default() Config {
	return Config{
		LLMProvider: "openrouter",
		OpenRouter: OpenRouterConfig{
			APIKey:            "",
			Model:             "google/gemini-3.0-flash",
			Temperature:       0.7,
			MaxTokens:         2000,
			APITimeoutSeconds: 30,
		},
		BufferSize:    2000,
		ContextWindow: 1000,
		StatusBar: StatusBarConfig{
			Position: "bottom",
			Colors:   "auto",
		},
		DryRun:   false,
		LogLevel: "info",
	}
}

// Load loads configuration from the specified path
// If the file doesn't exist, creates one with default values
func Load(configPath string) (Config, error) {
	// Ensure directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return Config{}, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Try to read existing config
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config
			cfg := Default()
			if err := Save(configPath, cfg); err != nil {
				return Config{}, fmt.Errorf("failed to create default config: %w", err)
			}
			return cfg, nil
		}
		return Config{}, fmt.Errorf("failed to read config: %w", err)
	}

	// Parse config
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config: %w", err)
	}

	return cfg, nil
}

// Save saves the configuration to the specified path
func Save(configPath string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	// Check LLM provider
	if c.LLMProvider != "openrouter" {
		return fmt.Errorf("unsupported LLM provider: %s", c.LLMProvider)
	}

	// API key required unless dry-run
	if !c.DryRun && c.OpenRouter.APIKey == "" {
		return fmt.Errorf("OpenRouter API key is required (set in config file)")
	}

	// Validate temperature
	if c.OpenRouter.Temperature < 0 || c.OpenRouter.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got: %f", c.OpenRouter.Temperature)
	}

	// Validate max tokens
	if c.OpenRouter.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive, got: %d", c.OpenRouter.MaxTokens)
	}

	// Validate API timeout
	if c.OpenRouter.APITimeoutSeconds <= 0 {
		return fmt.Errorf("api_timeout_seconds must be positive, got: %d", c.OpenRouter.APITimeoutSeconds)
	}

	// Validate buffer sizes
	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer_size must be positive, got: %d", c.BufferSize)
	}

	if c.ContextWindow <= 0 {
		return fmt.Errorf("context_window must be positive, got: %d", c.ContextWindow)
	}

	return nil
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".wtf_cli/config.json"
	}
	return filepath.Join(homeDir, ".wtf_cli", "config.json")
}
