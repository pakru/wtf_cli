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

func TestWelcomeMessageWithUpdate_ContainsUpdateInfo(t *testing.T) {
	msg := WelcomeMessageWithUpdate(&UpdateNotice{
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		ReleaseURL:     "https://github.com/pakru/wtf_cli/releases",
		UpgradeCommand: "curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash",
	})

	checks := []string{"Update available:", "Current: v0.1.0", "Latest:  v0.2.0", "Releases:", "Upgrade:"}
	for _, check := range checks {
		if !strings.Contains(msg, check) {
			t.Fatalf("Expected welcome update message to contain %q", check)
		}
	}
}

func TestWelcomeMessage_NoUpdateSectionByDefault(t *testing.T) {
	msg := WelcomeMessage()
	if strings.Contains(msg, "Update available:") {
		t.Fatal("Expected default welcome message to not contain update section")
	}
}
