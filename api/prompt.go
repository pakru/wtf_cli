package api

import (
	"fmt"
	"strings"
)

const SystemPrompt = `You are a command-line troubleshooting expert. Your job is to analyze failed shell commands and provide clear, actionable solutions.

RESPONSE GUIDELINES:
- Start with suggestion for next command to run
- Next include brief explanation of what likely went wrong
- Provide specific, copy-pasteable commands to fix the issue
- Include relevant context about why the error occurred
- Keep explanations concise but thorough
- Use code blocks for commands
- If multiple solutions exist, prioritize the most common/likely fix first
- Keep in mind that you are running in cli, so output should be copy-pasteable, and not much text should be included

FORMAT YOUR RESPONSE:
1. Suggest next command to run
2. Brief problem summary
3. Root cause explanation
4. Optional: Prevention tips for the future`

// BuildPrompt creates the user prompt from command and system information
func BuildPrompt(cmdInfo CommandInfo, sysInfo SystemInfo) string {
	var builder strings.Builder

	builder.WriteString("FAILED COMMAND ANALYSIS:\n\n")

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

	builder.WriteString("\nPlease analyze what went wrong and provide a solution.")

	return builder.String()
}

// CreateChatRequest builds a complete API request
func CreateChatRequest(cmdInfo CommandInfo, sysInfo SystemInfo) Request {
	userPrompt := BuildPrompt(cmdInfo, sysInfo)

	return Request{
		Model: DefaultModel,
		Messages: []Message{
			{
				Role:    "system",
				Content: SystemPrompt,
			},
			{
				Role:    "user",
				Content: userPrompt,
			},
		},
		Temperature: DefaultTemperature,
		MaxTokens:   DefaultMaxTokens,
		Stream:      false,
	}
}
