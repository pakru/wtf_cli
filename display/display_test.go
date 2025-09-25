package display

import (
	"testing"

	"wtf_cli/logger"
	"wtf_cli/system"
)

func TestNewSuggestionDisplayer(t *testing.T) {
	displayer := NewSuggestionDisplayer()
	if displayer == nil {
		t.Error("NewSuggestionDisplayer returned nil")
	}
}

func TestTruncateString(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	displayer := NewSuggestionDisplayer()

	tests := []struct {
		name     string
		input    string
		maxLen   int
		expected string
	}{
		{
			name:     "short string no truncation",
			input:    "short",
			maxLen:   10,
			expected: "short",
		},
		{
			name:     "exact length no truncation",
			input:    "exactly",
			maxLen:   7,
			expected: "exactly",
		},
		{
			name:     "long string with truncation",
			input:    "this is a very long string that should be truncated",
			maxLen:   20,
			expected: "this is a very long ...",
		},
		{
			name:     "empty string",
			input:    "",
			maxLen:   10,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := displayer.truncateString(tt.input, tt.maxLen)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestDisplayCommandSuggestion(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	displayer := NewSuggestionDisplayer()

	// Test that the function doesn't panic
	displayer.DisplayCommandSuggestion("ls /missing", 2, "This is a test suggestion")
}

func TestDisplayPipeSuggestion(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	displayer := NewSuggestionDisplayer()

	// Test that the function doesn't panic
	displayer.DisplayPipeSuggestion(100, "This is a test pipe suggestion")
}

func TestDisplayDryRunCommand(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	displayer := NewSuggestionDisplayer()

	// Test successful command
	displayer.DisplayDryRunCommand("ls", 0)

	// Test failed command
	displayer.DisplayDryRunCommand("ls /missing", 2)
}

func TestDisplayDryRunPipe(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	displayer := NewSuggestionDisplayer()

	// Test pipe dry run
	cmdInfo := CommandInfo{
		Command:  "echo test",
		Output:   "test input data",
		ExitCode: 0,
	}
	osInfo := system.OSInfo{
		Type:    "linux",
		Version: "22.04",
	}
	displayer.DisplayDryRunPipe(cmdInfo, osInfo)
}
