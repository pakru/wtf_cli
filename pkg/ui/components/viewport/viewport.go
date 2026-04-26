package viewport

import (
	"strings"

	"wtf_cli/pkg/ui/components/selection"
	"wtf_cli/pkg/ui/terminal"

	"charm.land/bubbles/v2/viewport"
	tea "charm.land/bubbletea/v2"
)

// PTYViewport wraps Bubble Tea's viewport for displaying PTY output
type PTYViewport struct {
	Viewport        viewport.Model
	content         string
	cursorTracker   *terminal.CursorTracker
	lineRenderer    *terminal.LineRenderer
	showCursor      bool
	ready           bool
	dirty           bool // True if content changed since last View()
	pauseAutoScroll bool // When true, AppendOutput does not auto-scroll to bottom
	sel             selection.Selection
}

// NewPTYViewport creates a new PTY viewport
func NewPTYViewport() PTYViewport {
	return PTYViewport{
		Viewport:      viewport.New(),
		content:       "",
		cursorTracker: terminal.NewCursorTracker(),
		lineRenderer:  terminal.NewLineRenderer(),
		showCursor:    true,
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
	if !v.sel.IsEmpty() || v.sel.Active {
		v.sel.Clear()
	}

	if v.lineRenderer != nil {
		v.lineRenderer.Append(data)
		v.content = v.lineRenderer.Content()
		if v.cursorTracker != nil {
			row, col := v.lineRenderer.CursorPosition()
			v.cursorTracker.SetPosition(row, col)
		}
	} else {
		v.content = terminal.AppendPTYContent(v.content, data, nil)
		if v.cursorTracker != nil {
			v.cursorTracker.UpdateFromOutput(data)
		}
	}
	v.dirty = true // Mark content as changed

	// Set content with cursor overlay
	v.renderContent()

	// Auto-scroll to bottom when new content arrives (suppressed in scroll mode)
	if !v.pauseAutoScroll {
		v.Viewport.GotoBottom()
	}
}

// SetCursorVisible toggles cursor overlay visibility and re-renders content.
func (v *PTYViewport) SetCursorVisible(visible bool) {
	if v.showCursor == visible {
		return
	}
	v.showCursor = visible
	v.renderContent()
	v.dirty = true
}

// GetContent returns the current viewport content
func (v *PTYViewport) GetContent() string {
	return v.content
}

// Clear empties the viewport
func (v *PTYViewport) Clear() {
	v.content = ""
	v.sel.Clear()
	if v.lineRenderer != nil {
		v.lineRenderer.Reset()
	}
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

// SetAutoScroll enables or disables auto-scrolling on new output.
// When disabled, AppendOutput will not move the viewport to the bottom.
// Re-enabling also snaps the viewport to the bottom immediately.
func (v *PTYViewport) SetAutoScroll(enabled bool) {
	v.pauseAutoScroll = !enabled
	if enabled {
		v.Viewport.GotoBottom()
	}
}

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

	// Visible lines equals viewport height
	visibleLines = v.Viewport.Height()

	// Scroll percent
	scrollPercent = int(v.Viewport.ScrollPercent() * 100)

	return
}

// RenderLines renders viewport content and returns it, without outer whitespace.
// Useful in tests.
func (v *PTYViewport) RenderLines() string {
	view := v.View()
	return strings.TrimSpace(view)
}

// StartSelection begins a mouse text selection from viewport-local screen
// coordinates.
func (v *PTYViewport) StartSelection(screenRow, screenCol int) {
	row, col, ok := v.selectionContentPoint(screenRow, screenCol, false)
	if !ok {
		return
	}
	v.sel.Start(row, col)
	v.renderContent()
	v.dirty = true
}

// UpdateSelection moves the active selection endpoint.
func (v *PTYViewport) UpdateSelection(screenRow, screenCol int) {
	if !v.sel.Active {
		return
	}
	row, col, ok := v.selectionContentPoint(screenRow, screenCol, true)
	if !ok {
		return
	}
	v.sel.Update(row, col)
	v.renderContent()
	v.dirty = true
}

// FinishSelection returns the selected text and clears the visible highlight.
func (v *PTYViewport) FinishSelection() string {
	if !v.sel.Active && v.sel.IsEmpty() {
		return ""
	}
	v.sel.Finish()
	text := selection.ExtractText(strings.Split(v.content, "\n"), v.sel)
	v.sel.Clear()
	v.renderContent()
	v.dirty = true
	return text
}

// ClearSelection removes the current selection.
func (v *PTYViewport) ClearSelection() {
	if !v.sel.Active && v.sel.IsEmpty() {
		return
	}
	v.sel.Clear()
	v.renderContent()
	v.dirty = true
}

// HasActiveSelection reports whether a drag selection is in progress.
func (v *PTYViewport) HasActiveSelection() bool {
	return v.sel.Active
}

// HasSelection reports whether a non-empty selection range exists.
func (v *PTYViewport) HasSelection() bool {
	return !v.sel.IsEmpty()
}

func (v *PTYViewport) renderContent() {
	content := v.content
	if !v.sel.IsEmpty() {
		content = selection.ApplyHighlight(content, v.sel)
	}
	if v.cursorTracker == nil {
		v.Viewport.SetContent(content)
		return
	}
	cursorChar := ""
	if v.showCursor {
		cursorChar = "█"
	}
	v.Viewport.SetContent(v.cursorTracker.RenderCursorOverlay(content, cursorChar))
}

func (v *PTYViewport) selectionContentPoint(screenRow, screenCol int, clamp bool) (int, int, bool) {
	height := v.Viewport.Height()
	width := v.Viewport.Width()
	if !v.ready || height <= 0 || width <= 0 {
		return 0, 0, false
	}
	if clamp {
		if screenRow < 0 {
			screenRow = 0
		}
		if screenRow >= height {
			screenRow = height - 1
		}
		if screenCol < 0 {
			screenCol = 0
		}
		if screenCol > width {
			screenCol = width
		}
	} else if screenRow < 0 || screenRow >= height || screenCol < 0 || screenCol >= width {
		return 0, 0, false
	}

	return v.Viewport.YOffset() + screenRow, v.Viewport.XOffset() + screenCol, true
}
