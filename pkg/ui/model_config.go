package ui

import (
	"os"
	"strings"

	"wtf_cli/pkg/config"
)

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "~"
	}
	return dir
}

func loadProviderAndModelFromConfig() (string, string) {
	path := config.GetConfigPath()
	if path == "" {
		return getProviderAndModel(config.Default())
	}
	if _, err := os.Stat(path); err != nil {
		return getProviderAndModel(config.Default())
	}
	cfg, err := config.Load(path)
	if err != nil {
		return getProviderAndModel(config.Default())
	}
	return getProviderAndModel(cfg)
}

func getProviderAndModel(cfg config.Config) (string, string) {
	provider := strings.TrimSpace(cfg.LLMProvider)
	if provider == "" {
		provider = "openrouter"
	}
	model := strings.TrimSpace(getModelForProvider(cfg))
	if model == "" {
		model = "unknown"
	}
	return provider, model
}

// getModelForProvider returns the model name for the currently selected provider
func getModelForProvider(cfg config.Config) string {
	switch cfg.LLMProvider {
	case "openai":
		if cfg.Providers.OpenAI.Model != "" {
			return cfg.Providers.OpenAI.Model
		}
		return "gpt-4o"
	case "copilot":
		if cfg.Providers.Copilot.Model != "" {
			return cfg.Providers.Copilot.Model
		}
		return "gpt-4o"
	case "anthropic":
		if cfg.Providers.Anthropic.Model != "" {
			return cfg.Providers.Anthropic.Model
		}
		return "claude-3-5-sonnet-20241022"
	case "google":
		if cfg.Providers.Google.Model != "" {
			return cfg.Providers.Google.Model
		}
		return "gemini-3-flash-preview"
	default: // openrouter or unknown
		if cfg.OpenRouter.Model != "" {
			return cfg.OpenRouter.Model
		}
		return config.Default().OpenRouter.Model
	}
}
