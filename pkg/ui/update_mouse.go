package ui

import (
	"time"

	"wtf_cli/pkg/ui/render"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleMouseWheel(msg tea.MouseWheelMsg) (Model, tea.Cmd) {
	// Mouse wheel scrolls the terminal viewport (when terminal focused) or
	// the chat sidebar (when sidebar focused). Full-screen mode passes mouse
	// events to the PTY application directly via the normal input path.
	if m.hasBlockingOverlay() {
		return m, nil
	}
	m2 := msg.Mouse()
	if !m.terminalFocused && m.sidebar != nil && m.sidebar.IsVisible() {
		// Sidebar has focus; let sidebar handle wheel.
		cmd := m.sidebar.HandleWheel(msg)
		return m, cmd
	}
	switch m2.Button {
	case tea.MouseWheelUp:
		m.viewport.ScrollUp()
		if !m.viewport.IsAtBottom() {
			m.setScrollMode(true)
		}
	case tea.MouseWheelDown:
		m.viewport.ScrollDown()
		if m.viewport.IsAtBottom() {
			m.setScrollMode(false)
		}
	}
	return m, nil
}

func (m Model) handleMouseClick(msg tea.MouseClickMsg) (Model, tea.Cmd) {
	if m.hasBlockingOverlay() {
		return m, nil
	}
	mouse := msg.Mouse()
	if mouse.Button != tea.MouseLeft {
		return m, nil
	}
	viewportHeight := render.ViewportHeight(m.height)
	if viewportHeight <= 0 || mouse.Y < 0 || mouse.Y >= viewportHeight || mouse.X < 0 || mouse.X >= m.width {
		return m, nil
	}
	viewportWidth := m.width
	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, _ := splitSidebarWidths(m.width)
		viewportWidth = left
		if mouse.X >= viewportWidth {
			m.focusSidebarInputFromMouse()
			if row, col, ok := m.sidebar.SelectionPoint(mouse.X, mouse.Y, viewportWidth); ok {
				m.viewport.ClearSelection()
				m.sidebar.StartSelection(row, col)
			}
			return m, nil
		}
		m.focusTerminalFromMouse()
	}
	if mouse.X >= 0 && mouse.X < viewportWidth {
		if m.sidebar != nil {
			m.sidebar.ClearSelection()
		}
		m.viewport.StartSelection(mouse.Y, mouse.X)
	}
	return m, nil
}

func (m Model) handleMouseMotion(msg tea.MouseMotionMsg) (Model, tea.Cmd) {
	if m.hasBlockingOverlay() {
		return m, nil
	}
	mouse := msg.Mouse()
	viewportWidth := m.width
	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, _ := splitSidebarWidths(m.width)
		viewportWidth = left
		if m.sidebar.HasActiveSelection() {
			if row, col, ok := m.sidebar.SelectionPoint(mouse.X, mouse.Y, viewportWidth); ok {
				m.sidebar.UpdateSelection(row, col)
			}
			return m, nil
		}
	}
	if m.viewport.HasActiveSelection() {
		if mouse.X < viewportWidth {
			m.viewport.UpdateSelection(mouse.Y, mouse.X)
		} else {
			m.viewport.UpdateSelection(mouse.Y, viewportWidth)
		}
	}
	return m, nil
}

func (m Model) handleMouseRelease(msg tea.MouseReleaseMsg) (Model, tea.Cmd) {
	if m.hasBlockingOverlay() {
		return m, nil
	}
	mouse := msg.Mouse()
	viewportWidth := m.width
	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, _ := splitSidebarWidths(m.width)
		viewportWidth = left
		if m.sidebar.HasActiveSelection() {
			if row, col, ok := m.sidebar.SelectionPoint(mouse.X, mouse.Y, viewportWidth); ok {
				m.sidebar.UpdateSelection(row, col)
			}
			return m, m.copySelectedText(m.sidebar.FinishSelection())
		}
	}
	if m.viewport.HasActiveSelection() {
		if mouse.X < viewportWidth {
			m.viewport.UpdateSelection(mouse.Y, mouse.X)
		} else {
			m.viewport.UpdateSelection(mouse.Y, viewportWidth)
		}
		return m, m.copySelectedText(m.viewport.FinishSelection())
	}
	return m, nil
}

func (m *Model) copySelectedText(text string) tea.Cmd {
	if text == "" {
		return nil
	}
	if m.statusBar != nil {
		m.statusBar.SetMessage(selectedTextCopiedMessage)
	}
	return tea.Batch(
		tea.SetClipboard(text),
		tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return clearStatusMsgMsg{}
		}),
	)
}

func (m *Model) clearTextSelections() {
	m.viewport.ClearSelection()
	if m.sidebar != nil {
		m.sidebar.ClearSelection()
	}
}

func (m *Model) focusSidebarInputFromMouse() {
	if m.sidebar == nil || !m.sidebar.IsVisible() {
		return
	}
	m.setTerminalFocused(false)
	// A sidebar click always returns keyboard input to chat, even if the
	// sidebar was already focused on its message viewport.
	m.sidebar.FocusInput()
}

func (m *Model) focusTerminalFromMouse() {
	m.setTerminalFocused(true)
	if m.sidebar != nil && m.sidebar.IsVisible() {
		m.sidebar.BlurInput()
	}
}
