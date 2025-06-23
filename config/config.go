package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the application configuration
type Config struct {
	LLMProvider string           `json:"llm_provider"`
	OpenRouter  OpenRouterConfig `json:"openrouter"`
}

// OpenRouterConfig holds the OpenRouter API configuration
type OpenRouterConfig struct {
	APIKey string `json:"api_key"`
	Model  string `json:"model"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() Config {
	return Config{
		LLMProvider: "openrouter",
		OpenRouter: OpenRouterConfig{
			APIKey: "",
			Model:  "openai/gpt-4o", // Default model
		},
	}
}

// LoadConfig loads the configuration from the specified path
// If the file doesn't exist, it creates one with default values
func LoadConfig(configPath string) (Config, error) {
	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return Config{}, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Try to read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create with default config
			cfg := DefaultConfig()
			if err := SaveConfig(configPath, cfg); err != nil {
				return Config{}, fmt.Errorf("failed to create default config: %w", err)
			}
			return cfg, nil
		}
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the config file
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// SaveConfig saves the configuration to the specified path
func SaveConfig(configPath string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	// Check if LLM provider is supported
	if c.LLMProvider != "openrouter" {
		return fmt.Errorf("unsupported LLM provider: %s", c.LLMProvider)
	}

	// Check if API key is provided
	if c.OpenRouter.APIKey == "" {
		return errors.New("OpenRouter API key is required")
	}
	return nil
}

// GetConfigPath returns the default path for the config file
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return ".wtf/config.json"
	}
	return filepath.Join(homeDir, ".wtf", "config.json")
}
