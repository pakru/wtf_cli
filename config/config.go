package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config represents the application configuration
type Config struct {
	LLMProvider string           `json:"llm_provider"`
	OpenRouter  OpenRouterConfig `json:"openrouter"`
	Debug       bool             `json:"debug"`
	DryRun      bool             `json:"dry_run"`
	LogLevel    string           `json:"log_level"`
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
		Debug:    false,
		DryRun:   false,
		LogLevel: "info",
	}
}

// LoadConfig loads the configuration from the specified path
// If the file doesn't exist, it creates one with default values
// Environment variables override config file values
func LoadConfig(configPath string) (Config, error) {
	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		return Config{}, fmt.Errorf("failed to create config directory: %w", err)
	}

	var cfg Config

	// Try to read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, create with default config
			cfg = DefaultConfig()
			if err := SaveConfig(configPath, cfg); err != nil {
				return Config{}, fmt.Errorf("failed to create default config: %w", err)
			}
		} else {
			return Config{}, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		// Parse the config file
		if err := json.Unmarshal(data, &cfg); err != nil {
			return Config{}, fmt.Errorf("failed to parse config: %w", err)
		}
	}

	// Override with environment variables (applies to both default and loaded config)
	cfg = applyEnvironmentOverrides(cfg)

	return cfg, nil
}

// applyEnvironmentOverrides applies environment variable overrides to the config
func applyEnvironmentOverrides(cfg Config) Config {
	// Debug mode
	if debugEnv := os.Getenv("WTF_DEBUG"); debugEnv != "" {
		if debug, err := strconv.ParseBool(debugEnv); err == nil {
			cfg.Debug = debug
		}
	}

	// Dry run mode
	if dryRunEnv := os.Getenv("WTF_DRY_RUN"); dryRunEnv != "" {
		if dryRun, err := strconv.ParseBool(dryRunEnv); err == nil {
			cfg.DryRun = dryRun
		}
	}

	// Log level
	if logLevel := os.Getenv("WTF_LOG_LEVEL"); logLevel != "" {
		validLevels := []string{"debug", "info", "warn", "error"}
		logLevel = strings.ToLower(logLevel)
		for _, valid := range validLevels {
			if logLevel == valid {
				cfg.LogLevel = logLevel
				break
			}
		}
	}

	// API Key override
	if apiKey := os.Getenv("WTF_API_KEY"); apiKey != "" {
		cfg.OpenRouter.APIKey = apiKey
	}

	// Model override
	if model := os.Getenv("WTF_MODEL"); model != "" {
		cfg.OpenRouter.Model = model
	}

	return cfg
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
