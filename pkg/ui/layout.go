package ui

import (
	"charm.land/lipgloss/v2"
)

// LayoutManager handles the overall UI layout
type LayoutManager struct {
	width  int
	height int
}

// NewLayoutManager creates a new layout manager
func NewLayoutManager() *LayoutManager {
	return &LayoutManager{
		width:  80,
		height: 24,
	}
}

// SetSize updates the layout dimensions
func (lm *LayoutManager) SetSize(width, height int) {
	lm.width = width
	lm.height = height
}

// ViewportHeight returns the height available for the viewport
// (total height minus status bar)
func (lm *LayoutManager) ViewportHeight() int {
	// Reserve 1 line for status bar
	return lm.height - 1
}

// StatusBarHeight returns the height for status bar
func (lm *LayoutManager) StatusBarHeight() int {
	return 1
}

// RenderLayout combines viewport and status bar
func (lm *LayoutManager) RenderLayout(viewportContent, statusBarContent string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		viewportContent,
		statusBarContent,
	)
}

// GetDimensions returns current width and height
func (lm *LayoutManager) GetDimensions() (width, height int) {
	return lm.width, lm.height
}
