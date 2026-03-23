package welcome

import (
	"regexp"
	"strings"
	"testing"
)

var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return ansiRegex.ReplaceAllString(s, "")
}

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

func TestWelcomeMessage_NoUpdateSectionByDefault(t *testing.T) {
	msg := WelcomeMessage()
	if strings.Contains(msg, "Update available") {
		t.Fatal("Expected default welcome message to not contain update section")
	}
}

func TestWelcomeMessageWithUpdate_DoesNotContainUpdateSection(t *testing.T) {
	// WelcomeMessageWithUpdate now delegates to buildWelcomeBox (no update info).
	// Update info is rendered separately by UpdateBanner.
	msg := WelcomeMessageWithUpdate(&UpdateNotice{
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		ReleaseURL:     "https://github.com/pakru/wtf_cli/releases",
		UpgradeCommand: "curl -fsSL https://example.com/install.sh | bash",
	})
	if strings.Contains(msg, "Update available") {
		t.Fatal("WelcomeMessageWithUpdate should no longer embed update info in the welcome box")
	}
}

func TestUpdateBanner_ContainsVersionInfo(t *testing.T) {
	banner := UpdateBanner(&UpdateNotice{
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		ReleaseURL:     "https://github.com/pakru/wtf_cli/releases",
		UpgradeCommand: "curl -fsSL https://example.com/install.sh | bash",
	})

	checks := []string{
		"Update available",
		"v0.1.0",
		"v0.2.0",
		"→",
	}
	plain := stripANSI(banner)
	for _, check := range checks {
		if !strings.Contains(plain, check) {
			t.Errorf("Expected update banner to contain %q", check)
		}
	}
}

func TestUpdateBanner_ContainsFullURL(t *testing.T) {
	fullURL := "https://github.com/pakru/wtf_cli/releases"
	banner := UpdateBanner(&UpdateNotice{
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		ReleaseURL:     fullURL,
		UpgradeCommand: "curl -fsSL https://example.com/install.sh | bash",
	})

	// The URL should NOT be truncated (strip ANSI since underline wraps each char)
	plain := stripANSI(banner)
	if !strings.Contains(plain, fullURL) {
		t.Fatalf("Expected update banner to contain full URL %q, got:\n%s", fullURL, plain)
	}
}

func TestUpdateBanner_ContainsFullCommand(t *testing.T) {
	fullCmd := "curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash"
	banner := UpdateBanner(&UpdateNotice{
		CurrentVersion: "v0.1.0",
		LatestVersion:  "v0.2.0",
		ReleaseURL:     "https://github.com/pakru/wtf_cli/releases",
		UpgradeCommand: fullCmd,
	})

	// The command should NOT be truncated (strip ANSI for bold-styled text)
	plain := stripANSI(banner)
	if !strings.Contains(plain, fullCmd) {
		t.Fatalf("Expected update banner to contain full command %q, got:\n%s", fullCmd, plain)
	}
}

func TestUpdateBanner_NilReturnsEmpty(t *testing.T) {
	banner := UpdateBanner(nil)
	if banner != "" {
		t.Fatalf("Expected empty string for nil notice, got: %q", banner)
	}
}
