package main

import (
	"fmt"
	"os"

	"wtf_cli/command"
	"wtf_cli/config"
	"wtf_cli/logger"
	"wtf_cli/shell"
)

func main() {
	// Check for version flag
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "-v" || os.Args[1] == "version") {
		fmt.Println(VersionInfo())
		os.Exit(0)
	}

	// Initialize logger with default settings first
	logger.InitLogger("error")

	// Load configuration from ~/.wtf/config.json
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err, "config_path", configPath)
		os.Exit(1)
	}

	// Re-initialize logger with configuration settings
	logger.InitLogger(cfg.LogLevel)
	logger.Info("Configuration loaded", "config_path", configPath, "dry_run", cfg.DryRun, "log_level", cfg.LogLevel)

	// Determine input mode and process accordingly
	pipeHandler := shell.NewPipeHandler(cfg)
	if pipeInput, err := pipeHandler.HandlePipeInput(); err == nil && pipeInput != "" {
		logger.Info("MODE: Pipe mode detected", "input_length", len(pipeInput))
		if err := processPipeModeWithInput(pipeHandler, pipeInput); err != nil {
			logger.Error("Failed to process pipe input", "error", err)
			os.Exit(1)
		}
	} else {
		logger.Info("MODE: Command mode detected - processing last command")
		if err := processCommandMode(cfg); err != nil {
			logger.Error("Failed to process command mode", "error", err)
			os.Exit(1)
		}
	}
}

// processPipeModeWithInput handles pipe input processing with pre-read input
func processPipeModeWithInput(pipeHandler *shell.PipeHandler, pipeInput string) error {
	return pipeHandler.ProcessPipeMode(pipeInput)
}

// processCommandMode handles command-based processing
func processCommandMode(cfg config.Config) error {
	commandHandler := command.NewCommandHandler(cfg)
	return commandHandler.ProcessCommandMode()
}
