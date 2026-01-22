package ui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/config"

	"github.com/charmbracelet/x/exp/golden"
)

var (
	pathPattern = regexp.MustCompile(`/home/[^/]+/(project|work)/wtf_cli/wtf_cli`)
	ansiPattern = regexp.MustCompile("\x1b\\[[0-9;]*m")
)

// normalizeOutput removes environment-specific paths and collapses shortcut lines
// to keep golden comparisons stable when shortcuts change.
func normalizeOutput(output string) string {
	output = strings.ReplaceAll(output, "\r", "")
	output = pathPattern.ReplaceAllString(output, "/path/to/wtf_cli")
	return normalizeWelcomeShortcuts(output)
}

func normalizeWelcomeShortcuts(output string) string {
	lines := strings.Split(output, "\n")
	normalized := make([]string, 0, len(lines))
	inShortcuts := false

	for _, line := range lines {
		if inShortcuts {
			if isWelcomeEmptyLine(line) {
				normalized = append(normalized, line)
				inShortcuts = false
			}
			continue
		}

		normalized = append(normalized, line)
		if isWelcomeShortcutsHeader(line) {
			inShortcuts = true
		}
	}

	return strings.Join(normalized, "\n")
}

func isWelcomeShortcutsHeader(line string) bool {
	return strings.Contains(stripANSI(line), "Shortcuts:")
}

func isWelcomeEmptyLine(line string) bool {
	stripped := strings.TrimSpace(stripANSI(line))
	if stripped == "" {
		return true
	}
	stripped = strings.TrimPrefix(stripped, "\u2502")
	stripped = strings.TrimSuffix(stripped, "\u2502")
	stripped = strings.TrimSpace(stripped)
	return stripped == ""
}

func stripANSI(line string) string {
	return ansiPattern.ReplaceAllString(line, "")
}

func TestModelViewGolden(t *testing.T) {
	// Setup a model in a deterministic state
	cwdFunc := func() (string, error) {
		return "/path/to/wtf_cli/pkg/ui", nil
	}
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), cwdFunc)
	m.ready = true
	m.width = 80
	m.height = 24
	m.statusBar.SetModel(config.Default().OpenRouter.Model)

	// Initialize viewport with fixed size
	m.viewport.SetSize(80, 23) // Height - 1 for status bar
	m.viewport.AppendOutput([]byte("Welcome to WTF CLI\nGolden Test Environment\n"))

	// Force resize time to be zero to avoid "recent resize" logic affecting output
	m.resizeTime = time.Time{}

	// Verify standard view
	view, _ := m.Render()
	normalizedView := normalizeOutput(view)
	golden.RequireEqual(t, []byte(normalizedView))
}

func TestModelViewGolden_Palette(t *testing.T) {
	cwdFunc := func() (string, error) {
		return "/path/to/wtf_cli/pkg/ui", nil
	}
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), cwdFunc)
	m.ready = true
	m.width = 80
	m.height = 24
	m.statusBar.SetModel(config.Default().OpenRouter.Model)
	m.viewport.SetSize(80, 23)

	// Open palette
	m.palette.Show()

	view, _ := m.Render()
	normalizedView := normalizeOutput(view)
	// Use a specific golden file name for palette state
	golden.RequireEqual(t, []byte(normalizedView))
}
