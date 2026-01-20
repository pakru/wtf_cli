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

// normalizePath removes environment-specific paths to make tests portable
// Replaces absolute paths with a placeholder for consistent golden file comparison
func normalizePath(output string) string {
	output = strings.ReplaceAll(output, "\r", "")
	// Replace any absolute path to wtf_cli with a normalized placeholder
	// Matches both: /home/dev/project/wtf_cli/wtf_cli and /home/runner/work/wtf_cli/wtf_cli
	re := regexp.MustCompile(`/home/[^/]+/(project|work)/wtf_cli/wtf_cli`)
	return re.ReplaceAllString(output, "/path/to/wtf_cli")
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
	normalizedView := normalizePath(view)
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
	normalizedView := normalizePath(view)
	// Use a specific golden file name for palette state
	golden.RequireEqual(t, []byte(normalizedView))
}
