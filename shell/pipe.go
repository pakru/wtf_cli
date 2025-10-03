package shell

import (
	"fmt"
	"io"
	"os"
	"time"

	"wtf_cli/api"
	"wtf_cli/config"
	"wtf_cli/display"
	"wtf_cli/logger"
	"wtf_cli/system"
)

// PipeHandler handles pipe input processing for WTF CLI
type PipeHandler struct {
	config config.Config
}

// NewPipeHandler creates a new pipe handler instance
func NewPipeHandler(cfg config.Config) *PipeHandler {
	return &PipeHandler{
		config: cfg,
	}
}

// IsReadingFromPipe detects if WTF is receiving input via stdin
func IsReadingFromPipe() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

// HandlePipeInput reads input from stdin when WTF is used in a pipe
func (h *PipeHandler) HandlePipeInput() (string, error) {
	if !IsReadingFromPipe() {
		return "", nil
	}

	// Read all input from stdin
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		logger.Error("Failed to read pipe input", "error", err)
		return "", fmt.Errorf("failed to read pipe input: %w", err)
	}

	return string(data), nil
}

// ProcessPipeMode processes pipe input with specialized handling
func (h *PipeHandler) ProcessPipeMode(input string) error {
	logger.Debug("Processing pipe mode", "input_length", len(input))

	// Create command info for pipe input (no original command tracking)
	cmdInfo := CommandInfo{
		Command:  "[N/A]",
		Output:   input,
		ExitCode: 0,
		Source:   SourcePipe,
	}

	// Get system information
	osInfo, err := system.GetOSInfo()
	if err != nil {
		logger.Warn("Failed to get OS info", "error", err)
	}

	// Create display handler
	displayer := display.NewSuggestionDisplayer()

	if h.config.DryRun {
		// Convert shell.CommandInfo to display.CommandInfo
		displayCmdInfo := display.CommandInfo{
			Command:  cmdInfo.Command,
			Output:   cmdInfo.Output,
			ExitCode: cmdInfo.ExitCode,
		}
		displayer.DisplayDryRunPipe(displayCmdInfo, osInfo)
		return nil
	}

	// Get AI suggestion for pipe input
	suggestion, err := h.getAISuggestion(cmdInfo, osInfo)
	if err != nil {
		logger.Error("Failed to get AI suggestion for pipe input", "error", err)
		return fmt.Errorf("failed to get AI suggestion for pipe input: %w", err)
	}

	displayer.DisplayPipeSuggestion(len(input), suggestion)
	return nil
}

// getAISuggestion gets an AI-powered suggestion for pipe input
func (h *PipeHandler) getAISuggestion(cmdInfo CommandInfo, osInfo system.OSInfo) (string, error) {
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

func (h *PipeHandler) getCurrentWorkingDir() string {
	if wd, err := os.Getwd(); err == nil {
		return wd
	}
	logger.Warn("Failed to get current working directory")
	return ""
}

func (h *PipeHandler) getShellInfo() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	return "/bin/bash" // default fallback
}

func (h *PipeHandler) getUserInfo() string {
	if user := os.Getenv("USER"); user != "" {
		return user
	}
	return ""
}

func (h *PipeHandler) getHomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	return ""
}
