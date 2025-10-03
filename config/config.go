package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"wtf_cli/logger"
)

// Config represents the application configuration
type Config struct {
	LLMProvider string           `json:"llm_provider"`
	OpenRouter  OpenRouterConfig `json:"openrouter"`
	DryRun      bool             `json:"dry_run"`
	LogLevel    string           `json:"log_level"`
}

// OpenRouterConfig holds the OpenRouter API configuration
type OpenRouterConfig struct {
	APIKey            string  `json:"api_key"`
	Model             string  `json:"model"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"max_tokens"`
	APITimeoutSeconds int     `json:"api_timeout_seconds"`
}

// DefaultConfig returns a configuration with default values
func DefaultConfig() Config {
	return Config{
		LLMProvider: "openrouter",
		OpenRouter: OpenRouterConfig{
			APIKey:            "",
			Model:             "google/gemma-3-27b", // Default model
			Temperature:       0.7,
			MaxTokens:         1000,
			APITimeoutSeconds: 30,
		},
		DryRun:   false,
		LogLevel: "info",
	}
}

// LoadConfig loads the configuration from the specified path
// If the file doesn't exist, it creates one with default values
// Environment variables override config file values
func LoadConfig(configPath string) (Config, error) {
	logger.Debug("Loading configuration", "config_path", configPath)

	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	logger.Debug("Creating config directory if needed", "config_dir", configDir)
	if err := os.MkdirAll(configDir, 0700); err != nil {
		logger.Error("Failed to create config directory", "error", err, "config_dir", configDir)
		return Config{}, fmt.Errorf("failed to create config directory: %w", err)
	}

	var cfg Config

	// Try to read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			logger.Info("Config file not found, creating default config", "config_path", configPath)
			// File doesn't exist, create with default config
			cfg = DefaultConfig()
			if err := SaveConfig(configPath, cfg); err != nil {
				logger.Error("Failed to create default config", "error", err, "config_path", configPath)
				return Config{}, fmt.Errorf("failed to create default config: %w", err)
			}
			logger.Info("Default config created successfully", "config_path", configPath)
		} else {
			logger.Error("Failed to read config file", "error", err, "config_path", configPath)
			return Config{}, fmt.Errorf("failed to read config file: %w", err)
		}
	} else {
		logger.Debug("Config file found, parsing", "config_path", configPath, "size", len(data))
		// Parse the config file
		if err := json.Unmarshal(data, &cfg); err != nil {
			logger.Error("Failed to parse config file", "error", err, "config_path", configPath)
			return Config{}, fmt.Errorf("failed to parse config: %w", err)
		}
		logger.Debug("Config file parsed successfully", "llm_provider", cfg.LLMProvider, "model", cfg.OpenRouter.Model)
	}

	// Override with environment variables (applies to both default and loaded config)
	logger.Debug("Applying environment variable overrides")
	cfg = applyEnvironmentOverrides(cfg)

	logger.Debug("Configuration loaded successfully",
		"llm_provider", cfg.LLMProvider,
		"model", cfg.OpenRouter.Model,
		"dry_run", cfg.DryRun,
		"log_level", cfg.LogLevel)

	return cfg, nil
}

// applyEnvironmentOverrides applies environment variable overrides to the config
func applyEnvironmentOverrides(cfg Config) Config {
	logger.Debug("Checking for environment variable overrides")
	// Dry run mode
	if dryRunEnv := os.Getenv("WTF_DRY_RUN"); dryRunEnv != "" {
		if dryRun, err := strconv.ParseBool(dryRunEnv); err == nil {
			logger.Debug("Overriding dry run mode from environment", "WTF_DRY_RUN", dryRun)
			cfg.DryRun = dryRun
		}
	}

	// Log level
	if logLevel := os.Getenv("WTF_LOG_LEVEL"); logLevel != "" {
		validLevels := []string{"debug", "info", "warn", "error"}
		logLevel = strings.ToLower(logLevel)
		for _, valid := range validLevels {
			if logLevel == valid {
				logger.Debug("Overriding log level from environment", "WTF_LOG_LEVEL", logLevel)
				cfg.LogLevel = logLevel
				break
			}
		}
	}

	// API Key override
	if apiKey := os.Getenv("WTF_API_KEY"); apiKey != "" {
		logger.Debug("Overriding API key from environment", "has_api_key", len(apiKey) > 0)
		cfg.OpenRouter.APIKey = apiKey
	}

	// Model override
	if model := os.Getenv("WTF_MODEL"); model != "" {
		logger.Debug("Overriding model from environment", "WTF_MODEL", model)
		cfg.OpenRouter.Model = model
	}

	// Temperature override
	if tempStr := os.Getenv("WTF_TEMPERATURE"); tempStr != "" {
		if temp, err := strconv.ParseFloat(tempStr, 64); err == nil && temp >= 0 && temp <= 2 {
			logger.Debug("Overriding temperature from environment", "WTF_TEMPERATURE", temp)
			cfg.OpenRouter.Temperature = temp
		}
	}

	// Max tokens override
	if tokensStr := os.Getenv("WTF_MAX_TOKENS"); tokensStr != "" {
		if tokens, err := strconv.Atoi(tokensStr); err == nil && tokens > 0 {
			logger.Debug("Overriding max tokens from environment", "WTF_MAX_TOKENS", tokens)
			cfg.OpenRouter.MaxTokens = tokens
		}
	}

	// API timeout override
	if timeoutStr := os.Getenv("WTF_API_TIMEOUT"); timeoutStr != "" {
		if timeout, err := strconv.Atoi(timeoutStr); err == nil && timeout > 0 {
			logger.Debug("Overriding API timeout from environment", "WTF_API_TIMEOUT", timeout)
			cfg.OpenRouter.APITimeoutSeconds = timeout
		}
	}

	return cfg
}

// SaveConfig saves the configuration to the specified path
func SaveConfig(configPath string, cfg Config) error {
	logger.Debug("Saving configuration", "config_path", configPath)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		logger.Error("Failed to marshal config", "error", err)
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0600); err != nil {
		logger.Error("Failed to write config file", "error", err, "config_path", configPath)
		return fmt.Errorf("failed to write config file: %w", err)
	}

	logger.Debug("Configuration saved successfully", "config_path", configPath, "size", len(data))
	return nil
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	// Check if LLM provider is supported
	if c.LLMProvider != "openrouter" {
		return fmt.Errorf("unsupported LLM provider: %s", c.LLMProvider)
	}

	// Check if API key is provided (skip in dry-run mode)
	if !c.DryRun && c.OpenRouter.APIKey == "" {
		return errors.New("OpenRouter API key is required (set WTF_API_KEY environment variable or add to config file)")
	}

	// Validate temperature range
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

// GetSystemPromptPath returns the default path for the system prompt file
func GetSystemPromptPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Fallback to current directory if home directory can't be determined
		return ".wtf/system_prompt.md"
	}
	return filepath.Join(homeDir, ".wtf", "system_prompt.md")
}
