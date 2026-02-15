package welcome

import (
	"strings"
	"testing"
)

func TestWelcomeMessage_ContainsShortcuts(t *testing.T) {
	msg := WelcomeMessage()

	shortcuts := []string{
		"Ctrl+D",
		"Ctrl+T",
		"Ctrl+R",
		"Shift+Tab",
		"/",
	}
	for _, shortcut := range shortcuts {
		if !strings.Contains(msg, shortcut) {
			t.Errorf("Expected welcome message to contain shortcut %q", shortcut)
		}
	}
}

func TestWelcomeMessage_ContainsTitle(t *testing.T) {
	msg := WelcomeMessage()
	if !strings.Contains(msg, "Welcome to WTF CLI") {
		t.Error("Expected welcome message to contain title")
	}
}

func TestWelcomeMessage_ContainsBorder(t *testing.T) {
	msg := WelcomeMessage()
	if !strings.Contains(msg, "╭") || !strings.Contains(msg, "╰") {
		t.Error("Expected welcome message to contain box border characters")
	}
}
