package ui

import (
	"wtf_cli/pkg/ui/render"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Rendering invariants:
// - Fullscreen mode uses the Bubble Tea alt screen.
// - Normal mode enables tea.MouseModeCellMotion.
// - Golden-test output must not change.

// View renders the UI (Bubble Tea lifecycle method)
func (m Model) View() tea.View {
	var v tea.View
	if !m.ready {
		v.SetContent("Initializing...")
		return v
	}

	// Full-screen mode: render only the fullscreen panel (no status bar)
	if m.fullScreenMode && m.fullScreenPanel != nil && m.fullScreenPanel.IsVisible() {
		v.AltScreen = true
		v.SetContent(m.fullScreenPanel.View())
		return v
	}

	// Enable mouse reporting for scrollback, sidebar scrolling, and in-app
	// drag selection. Cell motion reports drags without sending idle movement.
	if !m.fullScreenMode {
		v.MouseMode = tea.MouseModeCellMotion
	}

	v.SetContent(m.renderCanvas().Render())
	return v
}

// Render returns the string representation of the UI and whether altscreen is needed.
// Only exposed for testing purposes.
func (m Model) Render() (string, bool) {
	if !m.ready {
		return "Initializing...", false
	}

	// Full-screen mode: render only the fullscreen panel (no status bar)
	if m.fullScreenMode && m.fullScreenPanel != nil && m.fullScreenPanel.IsVisible() {
		return m.fullScreenPanel.View(), true
	}

	canvas := m.renderCanvas()
	return canvas.Render(), false
}

func (m Model) renderCanvas() *lipgloss.Canvas {
	const (
		baseLayerZ        = 0
		settingsLayerZ    = 1
		overlayLayerZ     = 2
		toolApprovalLayer = 3 // approval/continue popups are topmost overlays (modal)
	)

	width := m.width
	height := m.height
	if width <= 0 {
		width = m.viewport.Viewport.Width()
	}
	if height <= 0 {
		height = m.viewport.Viewport.Height() + 1
	}
	if width <= 0 || height <= 0 {
		return lipgloss.NewCanvas(0, 0)
	}

	// Update status bar width and directory for this frame.
	m.statusBar.SetWidth(width)
	m.statusBar.SetDirectory(m.currentDir)
	m.statusBar.SetGitBranch(m.gitBranch)

	viewportHeight := render.ViewportHeight(height)
	viewportWidth := width
	sidebarWidth := 0
	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, right := splitSidebarWidths(width)
		viewportWidth = left
		sidebarWidth = right
	}

	layers := make([]*lipgloss.Layer, 0, 5)

	if viewportWidth > 0 && viewportHeight > 0 {
		viewportLayer := lipgloss.NewLayer(m.viewport.View()).
			X(0).Y(0).
			Z(baseLayerZ)
		layers = append(layers, viewportLayer)
	}

	if sidebarWidth > 0 && viewportHeight > 0 && m.sidebar != nil && m.sidebar.IsVisible() {
		sidebarLayer := lipgloss.NewLayer(m.sidebar.View()).
			X(viewportWidth).Y(0).
			Z(baseLayerZ)
		layers = append(layers, sidebarLayer)
	}

	statusLayer := lipgloss.NewLayer(m.statusBar.Render()).
		X(0).Y(viewportHeight).
		Z(baseLayerZ)
	layers = append(layers, statusLayer)

	if m.settingsPanel.IsVisible() {
		layers = addOverlayLayer(layers, m.settingsPanel.View(), width, height, settingsLayerZ)
	}

	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		layers = addOverlayLayer(layers, m.optionPicker.View(), width, height, overlayLayerZ)
	} else if m.modelPicker != nil && m.modelPicker.IsVisible() {
		layers = addOverlayLayer(layers, m.modelPicker.View(), width, height, overlayLayerZ)
	} else if m.resultPanel.IsVisible() {
		layers = addOverlayLayer(layers, m.resultPanel.View(), width, viewportHeight, overlayLayerZ)
	} else if m.palette.IsVisible() {
		layers = addOverlayLayer(layers, m.palette.View(), width, height, overlayLayerZ)
	} else if m.historyPicker != nil && m.historyPicker.IsVisible() {
		layers = addOverlayLayer(layers, m.historyPicker.View(), width, height, overlayLayerZ)
	}

	if m.toolApproval != nil && m.toolApproval.IsVisible() {
		layers = addOverlayLayer(layers, m.toolApproval.View(), width, height, toolApprovalLayer)
	} else if m.continuePrompt != nil && m.continuePrompt.IsVisible() {
		layers = addOverlayLayer(layers, m.continuePrompt.View(), width, height, toolApprovalLayer)
	}

	return lipgloss.NewCanvas(width, height).Compose(lipgloss.NewCompositor(layers...))
}

func addOverlayLayer(layers []*lipgloss.Layer, view string, screenW, screenH, z int) []*lipgloss.Layer {
	if view == "" || screenW <= 0 || screenH <= 0 {
		return layers
	}
	panelW := lipgloss.Width(view)
	panelH := lipgloss.Height(view)
	x, y, w, h := render.CenterRect(panelW, panelH, screenW, screenH)
	if w <= 0 || h <= 0 {
		return layers
	}
	layer := lipgloss.NewLayer(view).
		X(x).Y(y).
		Z(z)
	return append(layers, layer)
}
