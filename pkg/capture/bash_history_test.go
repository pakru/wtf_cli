package capture

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestReadBashHistory(t *testing.T) {
	// Create a temporary history file
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, ".bash_history")

	// Write test history
	content := `#1234567890
ls -la
cd /tmp
#1234567891
git status
echo "hello world"

git commit -m "test"
#another timestamp
pwd
`
	if err := os.WriteFile(histFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test history file: %v", err)
	}

	// Set HISTFILE to our test file
	originalHistFile := os.Getenv("HISTFILE")
	os.Setenv("HISTFILE", histFile)
	defer os.Setenv("HISTFILE", originalHistFile)

	// Test reading history
	history, err := ReadBashHistory(0)
	if err != nil {
		t.Fatalf("ReadBashHistory failed: %v", err)
	}

	// Expected commands in reverse order (most recent first)
	expected := []string{
		"pwd",
		"git commit -m \"test\"",
		"echo \"hello world\"",
		"git status",
		"cd /tmp",
		"ls -la",
	}

	if len(history) != len(expected) {
		t.Errorf("Expected %d commands, got %d", len(expected), len(history))
	}

	for i, cmd := range expected {
		if i >= len(history) {
			break
		}
		if history[i] != cmd {
			t.Errorf("Command[%d]: expected %q, got %q", i, cmd, history[i])
		}
	}
}

func TestReadBashHistory_WithMaxLines(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, ".bash_history")

	content := `cmd1
cmd2
cmd3
cmd4
cmd5
`
	if err := os.WriteFile(histFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create test history file: %v", err)
	}

	originalHistFile := os.Getenv("HISTFILE")
	os.Setenv("HISTFILE", histFile)
	defer os.Setenv("HISTFILE", originalHistFile)

	history, err := ReadBashHistory(3)
	if err != nil {
		t.Fatalf("ReadBashHistory failed: %v", err)
	}

	if len(history) != 3 {
		t.Errorf("Expected 3 commands with maxLines=3, got %d", len(history))
	}

	// Should get the 3 most recent
	expected := []string{"cmd5", "cmd4", "cmd3"}
	for i, cmd := range expected {
		if history[i] != cmd {
			t.Errorf("Command[%d]: expected %q, got %q", i, cmd, history[i])
		}
	}
}

func TestReadBashHistory_NonexistentFile(t *testing.T) {
	// Point to a file that doesn't exist
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, "nonexistent", ".bash_history")

	originalHistFile := os.Getenv("HISTFILE")
	os.Setenv("HISTFILE", histFile)
	defer os.Setenv("HISTFILE", originalHistFile)

	history, err := ReadBashHistory(0)
	if err != nil {
		t.Errorf("Expected no error for nonexistent file, got: %v", err)
	}

	if len(history) != 0 {
		t.Errorf("Expected empty history for nonexistent file, got %d items", len(history))
	}
}

func TestReadBashHistory_FallbackToDefault(t *testing.T) {
	// Unset HISTFILE to test fallback
	originalHistFile := os.Getenv("HISTFILE")
	os.Unsetenv("HISTFILE")
	defer os.Setenv("HISTFILE", originalHistFile)

	// This should fall back to ~/.bash_history
	// We can't guarantee it exists, so we just check it doesn't crash
	_, err := ReadBashHistory(10)
	if err != nil {
		// Only fail if it's not a "file doesn't exist" error
		if !os.IsNotExist(err) {
			t.Errorf("Unexpected error: %v", err)
		}
	}
}

func TestMergeHistory(t *testing.T) {
	bashHistory := []string{
		"ls -la", // most recent in bash history
		"cd /tmp",
		"pwd",
		"git status", // oldest in bash history
	}

	sessionHistory := []CommandRecord{
		{Command: "echo test", StartTime: time.Now().Add(-3 * time.Minute)},
		{Command: "ls -la", StartTime: time.Now().Add(-2 * time.Minute)}, // duplicate
		{Command: "vim file.txt", StartTime: time.Now().Add(-1 * time.Minute)},
	}

	merged := MergeHistory(bashHistory, sessionHistory)

	// Expected order: session history first (most recent to oldest), then bash history (deduplicated)
	expected := []string{
		"vim file.txt",
		"ls -la", // from session (dedup, won't appear from bash)
		"echo test",
		"cd /tmp",
		"pwd",
		"git status",
	}

	if len(merged) != len(expected) {
		t.Errorf("Expected %d merged commands, got %d", len(expected), len(merged))
	}

	for i, cmd := range expected {
		if i >= len(merged) {
			break
		}
		if merged[i] != cmd {
			t.Errorf("Merged[%d]: expected %q, got %q", i, cmd, merged[i])
		}
	}
}

func TestMergeHistory_EmptyInputs(t *testing.T) {
	// Test with empty bash history
	merged := MergeHistory([]string{}, []CommandRecord{
		{Command: "echo test"},
	})
	if len(merged) != 1 || merged[0] != "echo test" {
		t.Errorf("Failed to handle empty bash history")
	}

	// Test with empty session history
	merged = MergeHistory([]string{"ls"}, []CommandRecord{})
	if len(merged) != 1 || merged[0] != "ls" {
		t.Errorf("Failed to handle empty session history")
	}

	// Test with both empty
	merged = MergeHistory([]string{}, []CommandRecord{})
	if len(merged) != 0 {
		t.Errorf("Expected empty result for both empty inputs")
	}
}

func TestMergeHistory_DeduplicationCaseInsensitive(t *testing.T) {
	// Test that deduplication is case-sensitive (different casing = different commands)
	bashHistory := []string{"Ls -la", "ls -la"}
	sessionHistory := []CommandRecord{}

	merged := MergeHistory(bashHistory, sessionHistory)

	// Both should be present (case-sensitive dedup)
	if len(merged) != 2 {
		t.Errorf("Expected 2 commands (case-sensitive), got %d", len(merged))
	}
}

func TestMergeHistory_IgnoreEmptyCommands(t *testing.T) {
	bashHistory := []string{"", "  ", "ls", "", "pwd"}
	sessionHistory := []CommandRecord{
		{Command: ""},
		{Command: "  "},
		{Command: "echo test"},
	}

	merged := MergeHistory(bashHistory, sessionHistory)

	// Should only have non-empty commands
	expected := []string{"echo test", "ls", "pwd"}
	if len(merged) != len(expected) {
		t.Errorf("Expected %d non-empty commands, got %d", len(expected), len(merged))
	}

	for i, cmd := range expected {
		if merged[i] != cmd {
			t.Errorf("Merged[%d]: expected %q, got %q", i, cmd, merged[i])
		}
	}
}
