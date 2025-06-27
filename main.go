package main

import (
	"fmt"
	"log"
	"os"

	"wtf_cli/config"
	"wtf_cli/logger"
	"wtf_cli/shell"
	"wtf_cli/system"
)

func main() {
	fmt.Println("wtf CLI utility - Go implementation started.")

	// Load configuration from ~/.wtf/config.json
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Initialize logger based on configuration
	logger.InitLogger(cfg.Debug, cfg.LogLevel)

	logger.Debug("Debug mode enabled")
	logger.Info("Configuration loaded", "config_path", configPath, "debug", cfg.Debug, "dry_run", cfg.DryRun)

	// Check if API key is configured (skip in dry-run mode)
	if !cfg.DryRun {
		if err := cfg.Validate(); err != nil {
			fmt.Printf("Configuration error: %v\n", err)
			fmt.Println("Please set your OpenRouter API key in the configuration file:")
			fmt.Printf("  %s\n", configPath)
			fmt.Println("\nOr use environment variable: export WTF_API_KEY=your_api_key")
			fmt.Println("Or run in dry-run mode: export WTF_DRY_RUN=true")
			os.Exit(1)
		}
		logger.Debug("API key validation passed")
	} else {
		logger.Info("Running in dry-run mode - skipping API key validation")
	}

	fmt.Println("Configuration loaded successfully.")

	// Get last command, output, and exit code
	logger.Debug("Retrieving last command information")
	cmdInfo, err := shell.GetLastCommand()
	if err != nil {
		logger.Warn("Failed to get last command", "error", err)
		log.Printf("Warning: Failed to get last command: %v", err)
	} else {
		logger.Debug("Last command retrieved", "command", cmdInfo.Command, "exit_code", cmdInfo.ExitCode)
	}

	// Get exit code separately if not available from GetLastCommand
	if cmdInfo.ExitCode == 0 {
		logger.Debug("Attempting to get exit code separately")
		exitCode, err := shell.GetLastExitCode()
		if err == nil {
			cmdInfo.ExitCode = exitCode
			logger.Debug("Exit code retrieved separately", "exit_code", exitCode)
		} else {
			logger.Debug("Failed to get exit code separately", "error", err)
		}
	}

	// Get current OS information
	logger.Debug("Retrieving system information")
	osInfo, err := system.GetOSInfo()
	if err != nil {
		logger.Warn("Failed to get OS info", "error", err)
		log.Printf("Warning: Failed to get OS info: %v", err)
	} else {
		logger.Debug("System information retrieved", "os_type", osInfo.Type, "distribution", osInfo.Distribution, "version", osInfo.Version)
	}

	// Display the collected information
	fmt.Println("\nLast Command Information:")
	fmt.Printf("  Command: %s\n", cmdInfo.Command)
	fmt.Printf("  Exit Code: %d\n", cmdInfo.ExitCode)

	fmt.Println("\nSystem Information:")
	fmt.Printf("  OS Type: %s\n", osInfo.Type)
	if osInfo.Distribution != "" {
		fmt.Printf("  Distribution: %s\n", osInfo.Distribution)
	}
	fmt.Printf("  Version: %s\n", osInfo.Version)
	fmt.Printf("  Kernel: %s\n", osInfo.Kernel)

	if cfg.DryRun {
		fmt.Println("\n DRY RUN MODE - No API calls will be made")
		fmt.Println("Mock LLM Response:")
		fmt.Println("---")
		fmt.Printf("It looks like the command '%s' ", cmdInfo.Command)
		if cmdInfo.ExitCode == 0 {
			fmt.Println("completed successfully!")
			fmt.Println("No issues detected.")
		} else {
			fmt.Printf("failed with exit code %d.\n", cmdInfo.ExitCode)
			fmt.Println("Common solutions:")
			fmt.Println("1. Check if the command syntax is correct")
			fmt.Println("2. Verify you have the necessary permissions")
			fmt.Println("3. Ensure all required dependencies are installed")
		}
		fmt.Println("---")
		logger.Info("Dry run completed successfully")
	} else {
		logger.Debug("Preparing to make API call")
		// TODO: Prepare payload for LLM
		// TODO: Call OpenRouter.ai API
		// TODO: Display suggestions
		fmt.Println("\n LLM API integration not yet implemented")
		fmt.Println("Set WTF_DRY_RUN=true to see mock responses")
	}
}
