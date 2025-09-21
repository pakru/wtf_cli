package testing

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestPipeIntegrationEndToEnd tests the complete pipe flow
func TestPipeIntegrationEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pipe integration test in short mode")
	}

	// Build the wtf binary for testing
	binaryPath := "../build/wtf"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("WTF binary not found at %s. Please run 'make build' first.", binaryPath)
	}

	tests := []struct {
		name        string
		input       string
		expectError bool
		description string
	}{
		{
			name:        "error_output_analysis",
			input:       "ls: cannot access '/nonexistent': No such file or directory",
			expectError: false,
			description: "Should analyze file not found error",
		},
		{
			name:        "successful_command_output",
			input:       "file1.txt\nfile2.txt\nfile3.txt",
			expectError: false,
			description: "Should analyze successful directory listing",
		},
		{
			name:        "empty_input",
			input:       "",
			expectError: false,
			description: "Should handle empty input gracefully",
		},
		{
			name:        "multiline_error",
			input:       "error: failed to connect\ndetails: connection timeout\nretry: attempt 3/5",
			expectError: false,
			description: "Should analyze multiline error output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create the command to pipe input to wtf
			cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | %s", strings.ReplaceAll(tt.input, "'", "'\"'\"'"), binaryPath))
			cmd.Env = append(os.Environ(), "WTF_DRY_RUN=true")

			// Execute the command
			output, err := cmd.CombinedOutput()
			outputStr := string(output)

			// Check for unexpected errors
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
				t.Logf("Output: %s", outputStr)
				return
			}

			// Verify pipe mode was detected
			if !strings.Contains(outputStr, "Pipe mode detected") {
				t.Errorf("Expected pipe mode detection in output")
			}

			// Verify dry run mode indication
			if !strings.Contains(outputStr, "Pipe Mode - Dry Run") {
				t.Errorf("Expected pipe mode dry run indication in output")
			}

			// Verify input size is reported correctly (account for newline from echo)
			expectedSize := len(tt.input)
			if tt.input != "" {
				expectedSize++ // Add 1 for newline from echo command
			}
			if expectedSize > 0 && !strings.Contains(outputStr, fmt.Sprintf("Input size: %d bytes", expectedSize)) {
				t.Errorf("Expected input size %d to be reported in output", expectedSize)
			}

			// Verify input preview is shown (for non-empty input)
			if len(tt.input) > 0 && !strings.Contains(outputStr, "Input preview:") {
				t.Errorf("Expected input preview to be shown for non-empty input")
			}

			// Verify mock response is shown
			if !strings.Contains(outputStr, "Mock Response:") {
				t.Errorf("Expected mock response in output")
			}

			t.Logf("Test case: %s", tt.description)
			t.Logf("Input: %q", tt.input)
			t.Logf("Output length: %d bytes", len(outputStr))
		})
	}
}

// TestPipeVsNormalMode tests that pipe mode and normal mode work correctly
func TestPipeVsNormalMode(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pipe vs normal mode test in short mode")
	}

	binaryPath := "../build/wtf"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("WTF binary not found at %s. Please run 'make build' first.", binaryPath)
	}

	t.Run("pipe_mode", func(t *testing.T) {
		// Test pipe mode
		cmd := exec.Command("bash", "-c", fmt.Sprintf("echo 'test pipe input' | %s", binaryPath))
		cmd.Env = append(os.Environ(), "WTF_DRY_RUN=true")

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Pipe mode failed: %v", err)
		}

		outputStr := string(output)
		if !strings.Contains(outputStr, "Pipe mode detected") {
			t.Errorf("Expected pipe mode to be detected")
		}
		if !strings.Contains(outputStr, "Pipe Mode - Dry Run") {
			t.Errorf("Expected pipe mode dry run display")
		}
	})

	t.Run("normal_mode", func(t *testing.T) {
		// Test normal mode (no pipe)
		cmd := exec.Command(binaryPath)
		cmd.Env = append(os.Environ(), "WTF_DRY_RUN=true")

		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("Normal mode failed: %v", err)
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "Pipe mode detected") {
			t.Errorf("Pipe mode should not be detected in normal mode")
		}
		if !strings.Contains(outputStr, "Dry Run Mode") {
			t.Errorf("Expected normal dry run mode display")
		}
	})
}

// TestPipeInputSizes tests handling of different input sizes
func TestPipeInputSizes(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping pipe input size test in short mode")
	}

	binaryPath := "../build/wtf"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("WTF binary not found at %s. Please run 'make build' first.", binaryPath)
	}

	sizes := []int{0, 1, 100, 1000, 10000}

	for _, size := range sizes {
		t.Run(fmt.Sprintf("size_%d", size), func(t *testing.T) {
			// Generate input of specified size
			input := strings.Repeat("a", size)

			// Test with pipe
			cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | %s", strings.ReplaceAll(input, "'", "'\"'\"'"), binaryPath))
			cmd.Env = append(os.Environ(), "WTF_DRY_RUN=true")

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Pipe test failed for size %d: %v", size, err)
			}

			outputStr := string(output)

			// Verify pipe mode was detected
			if !strings.Contains(outputStr, "Pipe mode detected") {
				t.Errorf("Expected pipe mode detection for size %d", size)
			}

			// Verify input size is reported correctly (account for newline from echo)
			expectedSize := size
			if size > 0 {
				expectedSize++ // Add 1 for newline from echo command
			}
			if expectedSize > 0 && !strings.Contains(outputStr, fmt.Sprintf("Input size: %d bytes", expectedSize)) {
				t.Errorf("Expected input size %d to be reported", expectedSize)
			}

			t.Logf("Successfully handled input size: %d bytes", size)
		})
	}
}

// TestPipeWithSpecialCharacters tests handling of special characters in pipe input
func TestPipeWithSpecialCharacters(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping special characters test in short mode")
	}

	binaryPath := "../build/wtf"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Fatalf("WTF binary not found at %s. Please run 'make build' first.", binaryPath)
	}

	tests := []struct {
		name  string
		input string
	}{
		{
			name:  "quotes_and_apostrophes",
			input: "error: 'file not found' or \"path invalid\"",
		},
		{
			name:  "newlines_and_tabs",
			input: "line1\nline2\twith tab\nline3",
		},
		{
			name:  "unicode_characters",
			input: "error: 文件未找到 or файл не найден",
		},
		{
			name:  "special_symbols",
			input: "error: $HOME not set, PATH=/usr/bin:$PATH, user@host:~/dir$",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Use printf to handle special characters properly
			cmd := exec.Command("bash", "-c", fmt.Sprintf("printf '%%s' '%s' | %s", strings.ReplaceAll(tt.input, "'", "'\"'\"'"), binaryPath))
			cmd.Env = append(os.Environ(), "WTF_DRY_RUN=true")

			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Special character test failed for %s: %v", tt.name, err)
			}

			outputStr := string(output)

			// Verify pipe mode was detected
			if !strings.Contains(outputStr, "Pipe mode detected") {
				t.Errorf("Expected pipe mode detection for %s", tt.name)
			}

			t.Logf("Successfully handled special characters: %s", tt.name)
		})
	}
}

// BenchmarkPipeProcessing benchmarks pipe input processing
func BenchmarkPipeProcessing(b *testing.B) {
	binaryPath := "../build/wtf"
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		b.Fatalf("WTF binary not found at %s. Please run 'make build' first.", binaryPath)
	}

	testInput := "ls: cannot access '/nonexistent': No such file or directory"

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | %s", testInput, binaryPath))
		cmd.Env = append(os.Environ(), "WTF_DRY_RUN=true")

		_, err := cmd.Output()
		if err != nil {
			b.Errorf("Benchmark failed: %v", err)
		}
	}
}
