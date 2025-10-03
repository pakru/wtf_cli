package display

import (
	"fmt"
	"strings"

	"wtf_cli/logger"
	"wtf_cli/system"
)

// CommandInfo represents command information for display purposes
type CommandInfo struct {
	Command  string
	Output   string
	ExitCode int
}

// SuggestionDisplayer handles formatting and displaying AI suggestions
type SuggestionDisplayer struct{}

// NewSuggestionDisplayer creates a new suggestion displayer
func NewSuggestionDisplayer() *SuggestionDisplayer {
	return &SuggestionDisplayer{}
}

// DisplayCommandSuggestion shows AI suggestion for a command with exit code
func (d *SuggestionDisplayer) DisplayCommandSuggestion(command string, exitCode int, suggestion string) {
	logger.Info("Displaying command suggestion",
		"command", command,
		"exit_code", exitCode,
		"suggestion_length", len(suggestion))

	headerText := fmt.Sprintf(" < Explanation of the command `%s`, exit code: %d >", command, exitCode)
	d.displayWithBorder(headerText, suggestion)
}

// DisplayPipeSuggestion shows AI suggestion for piped input
func (d *SuggestionDisplayer) DisplayPipeSuggestion(inputSize int, suggestion string) {
	logger.Info("Displaying pipe suggestion",
		"input_size", inputSize,
		"suggestion_length", len(suggestion))

	headerText := fmt.Sprintf(" < Analysis of piped input (%d bytes) >", inputSize)
	d.displayWithBorder(headerText, suggestion)
}

// DisplayDryRunCommand shows dry run information for command mode
func (d *SuggestionDisplayer) DisplayDryRunCommand(command string, exitCode int) {
	logger.Info("Displaying command dry run",
		"command", command,
		"exit_code", exitCode)

	fmt.Println("🧪 Dry Run Mode")
	fmt.Println("═══════════════")
	fmt.Println("No API calls will be made")
	fmt.Println()

	if exitCode == 0 {
		fmt.Println("✅ Mock Response:")
		fmt.Printf("   Command '%s' completed successfully\n", command)
		fmt.Println("   • Command executed without errors")
		fmt.Println("   • Check output for expected results")
		fmt.Println("   • Consider next steps in your workflow")
	} else {
		fmt.Println("❌ Mock Response:")
		fmt.Printf("   Command '%s' failed with exit code %d\n", command, exitCode)
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

// DisplayDryRunPipe shows dry run information for pipe mode
func (d *SuggestionDisplayer) DisplayDryRunPipe(cmdInfo CommandInfo, osInfo system.OSInfo) {
	logger.Info("Displaying pipe mode dry run", "input_size", len(cmdInfo.Output))

	fmt.Println("🧪 Pipe Mode - Dry Run")
	fmt.Println("═══════════════════════")
	fmt.Printf("Input size: %d bytes\n", len(cmdInfo.Output))
	fmt.Printf("Input preview: %s\n", d.truncateString(cmdInfo.Output, 100))

	fmt.Printf("OS: %s %s\n", osInfo.Type, osInfo.Version)
	fmt.Println()
	fmt.Println("💡 Mock Response:")
	fmt.Println("   • Analyzing piped input")
	fmt.Println("   • Providing contextual suggestions")
	fmt.Println("   • No API calls made in dry-run mode")
	fmt.Println()
	fmt.Println("🔧 To use real AI suggestions, set your API key and remove WTF_DRY_RUN")
}

// displayWithBorder shows content with a decorative border
func (d *SuggestionDisplayer) displayWithBorder(headerText, content string) {
	fmt.Println(headerText)
	fmt.Println(strings.Repeat("═", len(headerText)))
	fmt.Println(content)
	fmt.Println(strings.Repeat("═", len(headerText)))
}

// truncateString truncates a string to the specified maximum length
func (d *SuggestionDisplayer) truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
