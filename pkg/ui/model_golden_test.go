package ui

import (
	"testing"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"

	"github.com/charmbracelet/x/exp/golden"
)

func TestModelViewGolden(t *testing.T) {
	// Setup a model in a deterministic state
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.width = 80
	m.height = 24

	// Initialize viewport with fixed size
	m.viewport.SetSize(80, 23) // Height - 1 for status bar
	m.viewport.AppendOutput([]byte("Welcome to WTF CLI\nGolden Test Environment\n"))

	// Force resize time to be zero to avoid "recent resize" logic affecting output
	m.resizeTime = time.Time{}

	// Verify standard view
	// Verify standard view
	view, _ := m.Render()
	golden.RequireEqual(t, []byte(view))
}

func TestModelViewGolden_Palette(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.width = 80
	m.height = 24
	m.viewport.SetSize(80, 23)

	// Open palette
	m.palette.Show()

	view, _ := m.Render()
	// Use a specific golden file name for palette state
	golden.RequireEqual(t, []byte(view))
}
