package ui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// PTYViewport wraps Bubble Tea's viewport for displaying PTY output
type PTYViewport struct {
	viewport      viewport.Model
	content       string
	cursorTracker *CursorTracker
	ready         bool
	pendingCR     bool
}

// NewPTYViewport creates a new PTY viewport
func NewPTYViewport() PTYViewport {
	return PTYViewport{
		viewport:      viewport.New(),
		content:       "",
		cursorTracker: NewCursorTracker(),
	}
}

// SetSize updates the viewport dimensions
func (v *PTYViewport) SetSize(width, height int) {
	v.viewport.SetWidth(width)
	v.viewport.SetHeight(height)
	v.ready = true
}

// AppendOutput adds new output to the viewport
func (v *PTYViewport) AppendOutput(data []byte) {
	v.content = appendPTYContent(v.content, data, &v.pendingCR)

	// Track cursor position from ANSI codes
	v.cursorTracker.UpdateFromOutput(data)

	// Set content with cursor overlay
	contentWithCursor := v.cursorTracker.RenderCursorOverlay(v.content, "█")
	v.viewport.SetContent(contentWithCursor)

	// Auto-scroll to bottom when new content arrives
	v.viewport.GotoBottom()
}

// GetContent returns the current viewport content
func (v *PTYViewport) GetContent() string {
	return v.content
}

// Clear empties the viewport
func (v *PTYViewport) Clear() {
	v.content = ""
	v.viewport.SetContent("")
}

// Update handles viewport updates (scrolling, etc)
func (v *PTYViewport) Update(msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	v.viewport, cmd = v.viewport.Update(msg)
	return cmd
}

// View renders the viewport
func (v *PTYViewport) View() string {
	if !v.ready {
		return "Loading..."
	}

	view := v.viewport.View()
	if v.viewport.Width() <= 0 {
		return view
	}

	lines := strings.Split(view, "\n")
	for i := range lines {
		lines[i] = padStyled(lines[i], v.viewport.Width())
	}
	return strings.Join(lines, "\n")
}

// Scrolling helpers

// ScrollUp scrolls the viewport up
func (v *PTYViewport) ScrollUp() {
	v.viewport.ScrollUp(1)
}

// ScrollDown scrolls the viewport down
func (v *PTYViewport) ScrollDown() {
	v.viewport.ScrollDown(1)
}

// PageUp scrolls up one page
func (v *PTYViewport) PageUp() {
	v.viewport.PageUp()
}

// PageDown scrolls down one page
func (v *PTYViewport) PageDown() {
	v.viewport.PageDown()
}

// IsAtBottom returns true if scrolled to bottom
func (v *PTYViewport) IsAtBottom() bool {
	return v.viewport.AtBottom()
}

// Stats returns viewport statistics
func (v *PTYViewport) Stats() (totalLines, visibleLines, scrollPercent int) {
	// Count total lines
	lines := strings.Split(v.content, "\n")
	totalLines = len(lines)
	visibleLines = v.viewport.Height()

	// Calculate scroll percentage
	if totalLines <= visibleLines {
		scrollPercent = 100
	} else {
		scrollPercent = int(v.viewport.ScrollPercent() * 100)
	}

	return
}

// Style helpers for rendering

var viewportStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.RoundedBorder()).
	BorderForeground(lipgloss.Color("62"))
