package main

import (
	"fmt"
	"os"
	"time"

	"wtf_cli/api"
	"wtf_cli/config"
	"wtf_cli/logger"
	"wtf_cli/shell"
	"wtf_cli/system"
)

func main() {
	// Initialize logger with default settings first
	logger.InitLogger(false, "info")
	
	logger.Info("wtf CLI utility - Go implementation started")

	// Load configuration from ~/.wtf/config.json
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		logger.Error("Failed to load configuration", "error", err, "config_path", configPath)
		os.Exit(1)
	}

	// Re-initialize logger with configuration settings
	logger.InitLogger(cfg.Debug, cfg.LogLevel)

	logger.Debug("Debug mode enabled")
	logger.Info("Configuration loaded", "config_path", configPath, "debug", cfg.Debug, "dry_run", cfg.DryRun)

	// Check if API key is configured (skip in dry-run mode)
	if !cfg.DryRun {
		if err := cfg.Validate(); err != nil {
			logger.Error("Configuration validation failed", "error", err, "config_path", configPath)
			logger.Info("Please set your OpenRouter API key in the configuration file or WTF_OPENROUTER_API_KEY environment variable", "config_path", configPath)
			os.Exit(1)
		}
		logger.Debug("API key validation passed")
	}

	logger.Info("Configuration loaded successfully")

	// Get last command, output, and exit code
	logger.Debug("Retrieving last command information")
	cmdInfo, err := shell.GetLastCommand()
	if err != nil {
		logger.Warn("Failed to get last command", "error", err)
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
	} else {
		logger.Debug("System information retrieved", "os_type", osInfo.Type, "distribution", osInfo.Distribution, "version", osInfo.Version)
	}

	if cfg.DryRun {
		displayDryRunMode(cmdInfo)
	} else {
		logger.Debug("Preparing to make API call")
		
		// Get AI-powered suggestion
		suggestion, err := getAISuggestion(cfg, cmdInfo, osInfo)
		if err != nil {
			logger.Error("Failed to get AI suggestion", "error", err)
			logger.Info("You can run in dry-run mode with: export WTF_DRY_RUN=true")
			os.Exit(1)
		}

		// Display the AI suggestion to the user
		displayAISuggestion(suggestion)
		
		logger.Debug("AI suggestion displayed successfully")
	}
}

// getAISuggestion gets an AI-powered suggestion for the failed command
func getAISuggestion(cfg config.Config, cmdInfo shell.CommandInfo, osInfo system.OSInfo) (string, error) {
	logger.Debug("Creating API client")
	
	// Create API client
	client := api.NewClient(cfg.OpenRouter.APIKey)
	
	// Configure client with settings from config
	client.SetTimeout(time.Duration(cfg.OpenRouter.APITimeoutSeconds) * time.Second)
	
	logger.Debug("Converting system info to API format")
	
	// Convert shell.CommandInfo and system.OSInfo to API types
	apiCmdInfo := api.CommandInfo{
		Command:    cmdInfo.Command,
		ExitCode:   fmt.Sprintf("%d", cmdInfo.ExitCode),
		Output:     cmdInfo.Output,
		WorkingDir: getCurrentWorkingDir(),
		Duration:   "", // Not available in current shell.CommandInfo
	}
	
	apiSysInfo := api.SystemInfo{
		OS:           osInfo.Type,
		Distribution: osInfo.Distribution,
		Kernel:       osInfo.Kernel,
		Shell:        getShellInfo(),
		User:         getUserInfo(),
		Home:         getHomeDir(),
	}
	
	logger.Debug("Building API request")
	
	// Create the API request
	request := api.CreateChatRequest(apiCmdInfo, apiSysInfo)
	
	// Override model and parameters from config
	request.Model = cfg.OpenRouter.Model
	request.Temperature = cfg.OpenRouter.Temperature
	request.MaxTokens = cfg.OpenRouter.MaxTokens
	
	logger.Debug("Sending API request", 
		"model", request.Model,
		"temperature", request.Temperature,
		"max_tokens", request.MaxTokens)
	
	// Make the API call
	response, err := client.ChatCompletion(request)
	if err != nil {
		return "", fmt.Errorf("API call failed: %w", err)
	}
	
	// Extract the suggestion from the response
	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response choices received from API")
	}
	
	suggestion := response.Choices[0].Message.Content
	if suggestion == "" {
		return "", fmt.Errorf("empty suggestion received from API")
	}
	
	logger.Debug("API suggestion received successfully", 
		"response_id", response.ID,
		"model", response.Model,
		"total_tokens", response.Usage.TotalTokens,
		"suggestion_length", len(suggestion))
	
	return suggestion, nil
}

// Helper functions to gather additional system information

func getCurrentWorkingDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	return ""
}

func getShellInfo() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash" // default fallback
}

func getUserInfo() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return ""
}

func getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return ""
}

// displayAISuggestion shows the AI suggestion in a beautiful format
func displayAISuggestion(suggestion string) {
	fmt.Println("══════════════════════════════")
	fmt.Println(suggestion)
	fmt.Println("══════════════════════════════")
}

// displayDryRunMode shows dry run information in a beautiful format
func displayDryRunMode(cmdInfo shell.CommandInfo) {
	fmt.Println("🧪 Dry Run Mode")
	fmt.Println("═══════════════")
	fmt.Println("No API calls will be made")
	fmt.Println()
	
	if cmdInfo.ExitCode == 0 {
		fmt.Println("✅ Mock Response:")
		fmt.Printf("   Command '%s' completed successfully\n", cmdInfo.Command)
	} else {
		fmt.Println("❌ Mock Response:")
		fmt.Printf("   Command '%s' failed with exit code %d\n", cmdInfo.Command, cmdInfo.ExitCode)
		fmt.Println()
		fmt.Println("💡 Mock Suggestions:")
		fmt.Println("   • Check command syntax")
		fmt.Println("   • Verify file permissions")
		fmt.Println("   • Check dependencies")
		fmt.Println("   • Review error messages")
	}
	fmt.Println()
	fmt.Println("─────────────────────────────────────────────")
	fmt.Println("🔧 To use real AI suggestions, set your API key and remove WTF_DRY_RUN")
	fmt.Println()
}

