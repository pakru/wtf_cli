package api

import (
	"fmt"
	"os"
	"strings"

	"wtf_cli/config"
	"wtf_cli/logger"
)

// LoadSystemPrompt reads the system prompt from the markdown file
func LoadSystemPrompt() (string, error) {
	systemPromptPath := config.GetSystemPromptPath()
	logger.Debug("Loading system prompt", "path", systemPromptPath)
	
	data, err := os.ReadFile(systemPromptPath)
	if err != nil {
		logger.Error("Failed to read system prompt file", "error", err, "path", systemPromptPath)
		return "", fmt.Errorf("failed to read system prompt file: %w", err)
	}
	
	content := string(data)
	
	logger.Debug("System prompt loaded successfully", "length", len(content))
	return content, nil
}

// BuildPrompt creates the user prompt from command and system information
func BuildPrompt(cmdInfo CommandInfo, sysInfo SystemInfo) string {
	var builder strings.Builder

	if cmdInfo.ExitCode != "0" {
		builder.WriteString("FAILED COMMAND ANALYSIS:\n\n")
	} else {
		builder.WriteString("SUCCESSFUL COMMAND ANALYSIS:\n\n")
	}

	// Command details
	builder.WriteString("COMMAND DETAILS:\n")
	builder.WriteString(fmt.Sprintf("- Command: %s\n", cmdInfo.Command))
	builder.WriteString(fmt.Sprintf("- Exit Code: %s\n", cmdInfo.ExitCode))
	if cmdInfo.Output != "" {
		builder.WriteString(fmt.Sprintf("- Output: %s\n", cmdInfo.Output))
	}
	if cmdInfo.WorkingDir != "" {
		builder.WriteString(fmt.Sprintf("- Working Directory: %s\n", cmdInfo.WorkingDir))
	}
	if cmdInfo.Duration != "" {
		builder.WriteString(fmt.Sprintf("- Duration: %s\n", cmdInfo.Duration))
	}

	builder.WriteString("\nSYSTEM ENVIRONMENT:\n")
	if sysInfo.OS != "" {
		builder.WriteString(fmt.Sprintf("- OS: %s\n", sysInfo.OS))
	}
	if sysInfo.Distribution != "" {
		builder.WriteString(fmt.Sprintf("- Distribution: %s\n", sysInfo.Distribution))
	}
	if sysInfo.Kernel != "" {
		builder.WriteString(fmt.Sprintf("- Kernel: %s\n", sysInfo.Kernel))
	}
	if sysInfo.Shell != "" {
		builder.WriteString(fmt.Sprintf("- Shell: %s\n", sysInfo.Shell))
	}
	if sysInfo.User != "" {
		builder.WriteString(fmt.Sprintf("- User: %s\n", sysInfo.User))
	}

	if cmdInfo.ExitCode != "0" {
		builder.WriteString("\nPlease analyze what went wrong and provide a solution.")
	} else {
		builder.WriteString("\nPlease explain what the command accomplished.")
	}

	return builder.String()
}

// CreateChatRequest builds a complete API request
func CreateChatRequest(cmdInfo CommandInfo, sysInfo SystemInfo) (Request, error) {
	userPrompt := BuildPrompt(cmdInfo, sysInfo)
	
	systemPrompt, err := LoadSystemPrompt()
	if err != nil {
		return Request{}, fmt.Errorf("failed to load system prompt: %w", err)
	}

	return Request{
		Model: DefaultModel,
		Messages: []Message{
			{
				Role:    "system",
				Content: systemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: DefaultTemperature,
		MaxTokens:   DefaultMaxTokens,
		Stream:      false,
	}, nil
}
