package command

import (
	"fmt"
	"os"
	"time"

	"wtf_cli/api"
	"wtf_cli/config"
	"wtf_cli/display"
	"wtf_cli/logger"
	"wtf_cli/shell"
	"wtf_cli/system"
)

// CommandHandler handles command-based suggestion processing for WTF CLI
type CommandHandler struct {
	config config.Config
}

// NewCommandHandler creates a new command handler instance
func NewCommandHandler(cfg config.Config) *CommandHandler {
	return &CommandHandler{
		config: cfg,
	}
}

// ProcessCommandMode processes command information and provides suggestions
// This method handles the complete command mode workflow including validation,
// command retrieval, OS info gathering, and suggestion generation
func (h *CommandHandler) ProcessCommandMode() error {
	logger.Debug("Starting command mode processing")

	// Validate API key if not in dry-run mode
	if !h.config.DryRun {
		if err := h.config.Validate(); err != nil {
			logger.Error("Configuration validation failed", "error", err)
			return fmt.Errorf("configuration validation failed: %w", err)
		}
		logger.Debug("API key validation passed")
	}

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

	return h.processCommandWithInfo(cmdInfo, osInfo)
}

// processCommandWithInfo processes command information and provides suggestions
// This is the core processing logic separated for testability
func (h *CommandHandler) processCommandWithInfo(cmdInfo shell.CommandInfo, osInfo system.OSInfo) error {
	logger.Debug("Processing command mode",
		"command", cmdInfo.Command,
		"exit_code", cmdInfo.ExitCode,
		"source", cmdInfo.Source)

	// Create display handler
	displayer := display.NewSuggestionDisplayer()

	if h.config.DryRun {
		displayer.DisplayDryRunCommand(cmdInfo.Command, cmdInfo.ExitCode)
		return nil
	}

	// Get AI suggestion for command
	suggestion, err := h.getAISuggestion(cmdInfo, osInfo)
	if err != nil {
		logger.Error("Failed to get AI suggestion for command", "error", err)
		return fmt.Errorf("failed to get AI suggestion for command: %w", err)
	}

	displayer.DisplayCommandSuggestion(cmdInfo.Command, cmdInfo.ExitCode, suggestion)
	return nil
}

// getAISuggestion gets an AI-powered suggestion for the command
func (h *CommandHandler) getAISuggestion(cmdInfo shell.CommandInfo, osInfo system.OSInfo) (string, error) {
	logger.Debug("Creating API client")

	// Create API client
	client := api.NewClient(h.config.OpenRouter.APIKey)

	// Configure client with settings from config
	client.SetTimeout(time.Duration(h.config.OpenRouter.APITimeoutSeconds) * time.Second)

	logger.Debug("Converting system info to API format")

	// Convert shell.CommandInfo and system.OSInfo to API types
	apiCmdInfo := api.CommandInfo{
		Command:    cmdInfo.Command,
		ExitCode:   fmt.Sprintf("%d", cmdInfo.ExitCode),
		Output:     cmdInfo.Output,
		WorkingDir: h.getCurrentWorkingDir(),
		Duration:   "", // Not available in current shell.CommandInfo
	}

	apiSysInfo := api.SystemInfo{
		OS:           osInfo.Type,
		Distribution: osInfo.Distribution,
		Kernel:       osInfo.Kernel,
		Shell:        h.getShellInfo(),
		User:         h.getUserInfo(),
		Home:         h.getHomeDir(),
	}

	logger.Debug("Building API request")

	// Create the API request
	request, err := api.CreateChatRequest(apiCmdInfo, apiSysInfo)
	if err != nil {
		logger.Error("Failed to create API request", "error", err)
		return "", fmt.Errorf("failed to create API request: %w", err)
	}

	// Override model and parameters from config
	request.Model = h.config.OpenRouter.Model
	request.Temperature = h.config.OpenRouter.Temperature
	request.MaxTokens = h.config.OpenRouter.MaxTokens

	logger.Debug("Sending API request",
		"model", request.Model,
		"temperature", request.Temperature,
		"max_tokens", request.MaxTokens)

	// Make the API call
	response, err := client.ChatCompletion(request)
	if err != nil {
		logger.Error("API call failed", "error", err)
		return "", fmt.Errorf("API call failed: %w", err)
	}

	// Extract the suggestion from the response
	if len(response.Choices) == 0 {
		logger.Error("No response choices received from API")
		return "", fmt.Errorf("no response choices received from API")
	}

	suggestion := response.Choices[0].Message.Content
	if suggestion == "" {
		logger.Error("Empty suggestion received from API")
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

func (h *CommandHandler) getCurrentWorkingDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	logger.Warn("Failed to get current working directory")
	return ""
}

func (h *CommandHandler) getShellInfo() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash" // default fallback
}

func (h *CommandHandler) getUserInfo() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return ""
}

func (h *CommandHandler) getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return ""
}
