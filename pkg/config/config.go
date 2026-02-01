package config

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// Config represents the application configuration
type Config struct {
	LLMProvider   string           `json:"llm_provider"`
	OpenRouter    OpenRouterConfig `json:"openrouter"`
	Providers     ProvidersConfig  `json:"providers"`
	BufferSize    int              `json:"buffer_size"`
	ContextWindow int              `json:"context_window"`
	StatusBar     StatusBarConfig  `json:"status_bar"`
	LogFile       string           `json:"log_file"`
	LogFormat     string           `json:"log_format"`
	LogLevel      string           `json:"log_level"`
}

// ProvidersConfig holds configuration for all LLM providers.
type ProvidersConfig struct {
	OpenAI    OpenAIConfig    `json:"openai"`
	Copilot   CopilotConfig   `json:"copilot"`
	Anthropic AnthropicConfig `json:"anthropic"`
}

// OpenAIConfig holds OpenAI API configuration.
type OpenAIConfig struct {
	APIKey            string  `json:"api_key"`
	APIURL            string  `json:"api_url"`
	Model             string  `json:"model"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"max_tokens"`
	APITimeoutSeconds int     `json:"api_timeout_seconds"`
}

// CopilotConfig holds GitHub Copilot configuration.
type CopilotConfig struct {
	Model             string  `json:"model"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"max_tokens"`
	APITimeoutSeconds int     `json:"api_timeout_seconds"`
}

// AnthropicConfig holds Anthropic API configuration.
type AnthropicConfig struct {
	APIKey            string  `json:"api_key"`
	APIURL            string  `json:"api_url"`
	Model             string  `json:"model"`
	Temperature       float64 `json:"temperature"`
	MaxTokens         int     `json:"max_tokens"`
	APITimeoutSeconds int     `json:"api_timeout_seconds"`
}

// OpenRouterConfig holds the OpenRouter API configuration
type OpenRouterConfig struct {
	APIKey            string  `json:"api_key"`
	APIURL            string  `json:"api_url"`
	HTTPReferer       string  `json:"http_referer"`
	XTitle            string  `json:"x_title"`
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
			APIURL:            "https://openrouter.ai/api/v1",
			HTTPReferer:       "",
			XTitle:            "",
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
		LogFile:   defaultLogFilePath(),
		LogFormat: "json",
		LogLevel:  "info",
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

	cfg = applyDefaults(cfg, data)

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

// SupportedProviders returns a list of supported LLM provider names.
func SupportedProviders() []string {
	return []string{"openrouter", "openai", "copilot", "anthropic"}
}

// IsValidProvider checks if a provider name is supported.
func IsValidProvider(provider string) bool {
	for _, p := range SupportedProviders() {
		if p == provider {
			return true
		}
	}
	return false
}

// Validate checks if the configuration is valid
func (c Config) Validate() error {
	// Check LLM provider
	if !IsValidProvider(c.LLMProvider) {
		return fmt.Errorf("unsupported LLM provider: %s (supported: %v)", c.LLMProvider, SupportedProviders())
	}

	// Validate provider-specific config based on selected provider
	switch c.LLMProvider {
	case "openrouter":
		if err := c.validateOpenRouter(); err != nil {
			return err
		}
	case "openai":
		if err := c.validateOpenAI(); err != nil {
			return err
		}
	case "copilot":
		// Copilot uses OAuth, no API key validation needed here
	case "anthropic":
		if err := c.validateAnthropic(); err != nil {
			return err
		}
	}

	// Validate buffer sizes
	if c.BufferSize <= 0 {
		return fmt.Errorf("buffer_size must be positive, got: %d", c.BufferSize)
	}

	if c.ContextWindow <= 0 {
		return fmt.Errorf("context_window must be positive, got: %d", c.ContextWindow)
	}

	if strings.TrimSpace(c.LogLevel) != "" {
		switch strings.ToLower(strings.TrimSpace(c.LogLevel)) {
		case "trace", "debug", "info", "warn", "warning", "error":
		default:
			return fmt.Errorf("log_level must be one of trace, debug, info, warn, error, got: %s", c.LogLevel)
		}
	}

	if strings.TrimSpace(c.LogFormat) != "" {
		switch strings.ToLower(strings.TrimSpace(c.LogFormat)) {
		case "json", "text":
		default:
			return fmt.Errorf("log_format must be json or text, got: %s", c.LogFormat)
		}
	}

	return nil
}

func (c Config) validateOpenRouter() error {
	if c.OpenRouter.APIKey == "" {
		return fmt.Errorf("OpenRouter API key is required (set in config file)")
	}

	apiURL := strings.TrimSpace(c.OpenRouter.APIURL)
	if apiURL == "" {
		return fmt.Errorf("OpenRouter api_url is required (set in config file)")
	}
	parsedURL, err := url.Parse(apiURL)
	if err != nil || parsedURL.Scheme == "" || parsedURL.Host == "" {
		return fmt.Errorf("OpenRouter api_url must be a valid URL, got: %s", c.OpenRouter.APIURL)
	}

	if strings.TrimSpace(c.OpenRouter.Model) == "" {
		return fmt.Errorf("OpenRouter model is required (set in config file)")
	}

	if c.OpenRouter.Temperature < 0 || c.OpenRouter.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2, got: %f", c.OpenRouter.Temperature)
	}

	if c.OpenRouter.MaxTokens <= 0 {
		return fmt.Errorf("max_tokens must be positive, got: %d", c.OpenRouter.MaxTokens)
	}

	if c.OpenRouter.APITimeoutSeconds <= 0 {
		return fmt.Errorf("api_timeout_seconds must be positive, got: %d", c.OpenRouter.APITimeoutSeconds)
	}

	return nil
}

func (c Config) validateOpenAI() error {
	// API key can be empty if using OAuth
	if strings.TrimSpace(c.Providers.OpenAI.Model) == "" {
		// Will use default model
	}
	return nil
}

func (c Config) validateAnthropic() error {
	if c.Providers.Anthropic.APIKey == "" {
		return fmt.Errorf("Anthropic API key is required (set in config file)")
	}
	return nil
}

type configPresence struct {
	LLMProvider *string `json:"llm_provider"`
	OpenRouter  *struct {
		APIKey            *string  `json:"api_key"`
		APIURL            *string  `json:"api_url"`
		HTTPReferer       *string  `json:"http_referer"`
		XTitle            *string  `json:"x_title"`
		Model             *string  `json:"model"`
		Temperature       *float64 `json:"temperature"`
		MaxTokens         *int     `json:"max_tokens"`
		APITimeoutSeconds *int     `json:"api_timeout_seconds"`
	} `json:"openrouter"`
	BufferSize    *int `json:"buffer_size"`
	ContextWindow *int `json:"context_window"`
	StatusBar     *struct {
		Position *string `json:"position"`
		Colors   *string `json:"colors"`
	} `json:"status_bar"`
	LogFile   *string `json:"log_file"`
	LogFormat *string `json:"log_format"`
	LogLevel  *string `json:"log_level"`
}

func applyDefaults(cfg Config, data []byte) Config {
	defaults := Default()
	var presence configPresence
	if err := json.Unmarshal(data, &presence); err != nil {
		return cfg
	}

	if presence.LLMProvider == nil || strings.TrimSpace(cfg.LLMProvider) == "" {
		cfg.LLMProvider = defaults.LLMProvider
	}

	if presence.OpenRouter == nil {
		cfg.OpenRouter = defaults.OpenRouter
	} else {
		if presence.OpenRouter.APIKey == nil {
			cfg.OpenRouter.APIKey = defaults.OpenRouter.APIKey
		}
		if presence.OpenRouter.APIURL == nil || strings.TrimSpace(cfg.OpenRouter.APIURL) == "" {
			cfg.OpenRouter.APIURL = defaults.OpenRouter.APIURL
		}
		if presence.OpenRouter.HTTPReferer == nil {
			cfg.OpenRouter.HTTPReferer = defaults.OpenRouter.HTTPReferer
		}
		if presence.OpenRouter.XTitle == nil {
			cfg.OpenRouter.XTitle = defaults.OpenRouter.XTitle
		}
		if presence.OpenRouter.Model == nil || strings.TrimSpace(cfg.OpenRouter.Model) == "" {
			cfg.OpenRouter.Model = defaults.OpenRouter.Model
		}
		if presence.OpenRouter.Temperature == nil {
			cfg.OpenRouter.Temperature = defaults.OpenRouter.Temperature
		}
		if presence.OpenRouter.MaxTokens == nil || cfg.OpenRouter.MaxTokens <= 0 {
			cfg.OpenRouter.MaxTokens = defaults.OpenRouter.MaxTokens
		}
		if presence.OpenRouter.APITimeoutSeconds == nil || cfg.OpenRouter.APITimeoutSeconds <= 0 {
			cfg.OpenRouter.APITimeoutSeconds = defaults.OpenRouter.APITimeoutSeconds
		}
	}

	if presence.BufferSize == nil || cfg.BufferSize <= 0 {
		cfg.BufferSize = defaults.BufferSize
	}

	if presence.ContextWindow == nil || cfg.ContextWindow <= 0 {
		cfg.ContextWindow = defaults.ContextWindow
	}

	if presence.StatusBar == nil {
		cfg.StatusBar = defaults.StatusBar
	} else {
		if presence.StatusBar.Position == nil || strings.TrimSpace(cfg.StatusBar.Position) == "" {
			cfg.StatusBar.Position = defaults.StatusBar.Position
		}
		if presence.StatusBar.Colors == nil || strings.TrimSpace(cfg.StatusBar.Colors) == "" {
			cfg.StatusBar.Colors = defaults.StatusBar.Colors
		}
	}

	if presence.LogFile == nil || strings.TrimSpace(cfg.LogFile) == "" {
		cfg.LogFile = defaults.LogFile
	}

	if presence.LogFormat == nil || strings.TrimSpace(cfg.LogFormat) == "" {
		cfg.LogFormat = defaults.LogFormat
	}

	if presence.LogLevel == nil || strings.TrimSpace(cfg.LogLevel) == "" {
		cfg.LogLevel = defaults.LogLevel
	}

	return cfg
}

func defaultLogFilePath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return filepath.Join(".wtf_cli", "logs", "wtf_cli.log")
	}
	return filepath.Join(homeDir, ".wtf_cli", "logs", "wtf_cli.log")
}

// GetConfigPath returns the default configuration file path
func GetConfigPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return ".wtf_cli/config.json"
	}
	return filepath.Join(homeDir, ".wtf_cli", "config.json")
}
