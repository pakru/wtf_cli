package ui

import "log/slog"

// hasBlockingOverlay reports whether an overlay is active that should absorb
// input and mouse events before terminal/sidebar routing.
func (m *Model) hasBlockingOverlay() bool {
	if m.fullScreenMode {
		return true
	}
	if m.settingsPanel != nil && m.settingsPanel.IsVisible() {
		return true
	}
	if m.palette != nil && m.palette.IsVisible() {
		return true
	}
	if m.historyPicker != nil && m.historyPicker.IsVisible() {
		return true
	}
	if m.resultPanel != nil && m.resultPanel.IsVisible() {
		return true
	}
	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		return true
	}
	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		return true
	}
	return false
}

func (m *Model) setTerminalFocused(focused bool) {
	if m.terminalFocused == focused {
		return
	}
	m.terminalFocused = focused
	m.viewport.SetCursorVisible(focused)

	if m.sidebar == nil || !m.sidebar.IsVisible() {
		return
	}
	if focused {
		m.sidebar.BlurInput()
		return
	}
	m.sidebar.FocusInput()
}

// setScrollMode activates or deactivates scroll mode, keeping viewport auto-scroll
// and the status bar badge in sync.
func (m *Model) setScrollMode(active bool) {
	m.scrollMode = active
	m.viewport.SetAutoScroll(!active)
	m.statusBar.SetScrollMode(active)
}

func (m *Model) showSidebar(reason string) {
	if m.sidebar == nil {
		return
	}
	m.sidebar.Show()
	m.sidebar.FocusInput()
	m.setTerminalFocused(false)
	slog.Info("sidebar_open", "reason", reason)
	m.applyLayout()
}

func (m *Model) hideSidebar(reason string) {
	if m.sidebar == nil {
		return
	}
	m.sidebar.Hide()
	slog.Info("sidebar_close", "reason", reason)
	m.setTerminalFocused(true)
	m.applyLayout()
}

func (m *Model) toggleSidebar(reason string) {
	if m.sidebar == nil {
		return
	}
	if m.sidebar.IsVisible() {
		m.hideSidebar(reason)
		return
	}
	m.showSidebar(reason)
}
