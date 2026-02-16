package main

import (
	"fmt"
	"log/slog"
	"os"

	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/logging"
	"wtf_cli/pkg/pty"
	"wtf_cli/pkg/ui"

	// Import providers package to register all LLM providers via init()
	_ "wtf_cli/pkg/ai/providers"

	tea "charm.land/bubbletea/v2"
)

func main() {
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v") {
		printVersion()
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	if _, err := logging.Init(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing logger: %v\n", err)
	}
	slog.Info("app_start",
		"config_path", config.GetConfigPath(),
		"provider", cfg.LLMProvider,
		"model", getModelForProvider(cfg),
		"log_level", cfg.LogLevel,
		"log_format", cfg.LogFormat,
		"log_file", cfg.LogFile,
	)

	// Spawn the shell in a PTY with buffer
	wrapper, err := pty.SpawnShellWithBuffer(cfg.BufferSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error spawning shell: %v\n", err)
		os.Exit(1)
	}
	defer wrapper.Close()

	// Initialize session context
	session := capture.NewSessionContext()

	// Create Bubble Tea model with shell's cwd function
	model := ui.NewModel(wrapper.GetPTY(), wrapper.GetBuffer(), session, wrapper.GetCwd)

	// Create Bubble Tea program
	// Note: In v2, AltScreen is set via View.AltScreen in the Model's View() method
	// Note: Not using mouse motion to allow normal text selection
	p := tea.NewProgram(model)

	// Run the program
	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
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
