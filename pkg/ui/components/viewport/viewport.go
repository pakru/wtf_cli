package viewport

import (
	"strings"

	"wtf_cli/pkg/ui/terminal"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PTYViewport wraps Bubble Tea's viewport for displaying PTY output
type PTYViewport struct {
	Viewport      viewport.Model
	content       string
	cursorTracker *terminal.CursorTracker
	ready         bool
	pendingCR     bool
	dirty         bool // True if content changed since last View()
}

// NewPTYViewport creates a new PTY viewport
func NewPTYViewport() PTYViewport {
	return PTYViewport{
		Viewport:      viewport.New(),
		content:       "",
		cursorTracker: terminal.NewCursorTracker(),
	}
}

// SetSize updates the viewport dimensions
func (v *PTYViewport) SetSize(width, height int) {
	v.Viewport.SetWidth(width)
	v.Viewport.SetHeight(height)
	v.ready = true
}

// AppendOutput adds new output to the viewport
func (v *PTYViewport) AppendOutput(data []byte) {
	if len(data) == 0 {
		return
	}

	v.content = terminal.AppendPTYContent(v.content, data, &v.pendingCR)
	v.dirty = true // Mark content as changed

	// Track cursor position from ANSI codes
	v.cursorTracker.UpdateFromOutput(data)

	// Set content with cursor overlay
	contentWithCursor := v.cursorTracker.RenderCursorOverlay(v.content, "█")
	v.Viewport.SetContent(contentWithCursor)

	// Auto-scroll to bottom when new content arrives
	v.Viewport.GotoBottom()
}

// GetContent returns the current viewport content
func (v *PTYViewport) GetContent() string {
	return v.content
}

// Clear empties the viewport
func (v *PTYViewport) Clear() {
	v.content = ""
	v.Viewport.SetContent("")
	v.dirty = true // Mark as changed
}

// Update handles viewport updates (scrolling, etc)
func (v *PTYViewport) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.Viewport, cmd = v.Viewport.Update(msg)
	return cmd
}

// View renders the viewport
func (v *PTYViewport) View() string {
	v.dirty = false // Clear dirty flag on render

	if !v.ready {
		return "Loading..."
	}

	return v.Viewport.View()
}

// Scrolling helpers

// ScrollUp scrolls the viewport up
func (v *PTYViewport) ScrollUp() {
	v.Viewport.ScrollUp(1)
}

// ScrollDown scrolls the viewport down
func (v *PTYViewport) ScrollDown() {
	v.Viewport.ScrollDown(1)
}

// PageUp scrolls up one page
func (v *PTYViewport) PageUp() {
	v.Viewport.PageUp()
}

// PageDown scrolls down one page
func (v *PTYViewport) PageDown() {
	v.Viewport.PageDown()
}

// IsAtBottom returns true if scrolled to bottom
func (v *PTYViewport) IsAtBottom() bool {
	return v.Viewport.AtBottom()
}

// Stats returns viewport statistics
func (v *PTYViewport) Stats() (totalLines, visibleLines, scrollPercent int) {
	// Count total lines
	lines := strings.Split(v.content, "\n")
	totalLines = len(lines)
	visibleLines = v.Viewport.Height()

	// Calculate scroll percentage
	if totalLines <= visibleLines {
		scrollPercent = 100
	} else {
		scrollPercent = int(v.Viewport.ScrollPercent() * 100)
	}

	return
}

// Style helpers for rendering

var viewportStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62"))
