package ui

import (
	"log/slog"
	"time"

	"wtf_cli/pkg/ui/components/fullscreen"
	"wtf_cli/pkg/ui/terminal"

	tea "charm.land/bubbletea/v2"
)

type resizeApplyMsg struct {
	id     int
	width  int
	height int
}

func (m Model) handleWindowSize(msg tea.WindowSizeMsg) (Model, tea.Cmd) {
	m.width = msg.Width
	m.height = msg.Height
	m.ready = true
	slog.Debug("window_resize", "width", m.width, "height", m.height)

	// Update UI component sizes immediately for display
	m.resizeComponents(m.width, m.height)
	if m.fullScreenMode && m.fullScreenPanel != nil {
		m.fullScreenPanel.Resize(m.width, m.height)
	}

	// Debounce PTY resize to avoid multiple prompt reprints during drag
	m.resizeDebounceID++
	resizeID := m.resizeDebounceID
	return m, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
		return resizeApplyMsg{id: resizeID, width: msg.Width, height: msg.Height}
	})
}

func (m Model) handleResizeApply(msg resizeApplyMsg) (Model, tea.Cmd) {
	// Only apply PTY resize if this is the most recent resize event
	if msg.id != m.resizeDebounceID {
		return m, nil
	}
	// Resize PTY so bash knows correct terminal dimensions for line wrapping
	if m.ptyFile != nil {
		if m.fullScreenMode {
			contentWidth, contentHeight := fullscreen.ContentSize(m.width, m.height)
			terminal.ResizePTY(m.ptyFile, contentWidth, contentHeight)
		} else {
			viewportHeight := m.height - 1
			viewportWidth := m.width
			if m.sidebar != nil && m.sidebar.IsVisible() {
				left, _ := splitSidebarWidths(m.width)
				viewportWidth = left
			}
			m.resizePTYViewport(viewportWidth, viewportHeight)
			// Track resize time to suppress prompt reprint output
			// Skip suppression on initial resize (first time we get correct size)
			if m.initialResize {
				m.resizeTime = time.Now()
			}
			m.initialResize = true
		}
	}
	return m, nil
}

// resizePTYViewport resizes the PTY to the given viewport dimensions.
// It is a no-op when dimensions are not valid, which avoids uint16 overflow
// before the first WindowSizeMsg has initialized model dimensions.
func (m *Model) resizePTYViewport(width, height int) {
	if m.ptyFile == nil || width <= 0 || height <= 0 {
		return
	}
	if err := terminal.ResizePTY(m.ptyFile, width, height); err != nil {
		slog.Warn("pty_resize_failed", "width", width, "height", height, "error", err)
	}
}

func (m *Model) applyLayout() {
	if m.fullScreenMode {
		if m.fullScreenPanel != nil {
			m.fullScreenPanel.Resize(m.width, m.height)
		}
		if m.ptyFile != nil {
			contentWidth, contentHeight := fullscreen.ContentSize(m.width, m.height)
			terminal.ResizePTY(m.ptyFile, contentWidth, contentHeight)
		}
		return
	}

	viewportWidth, viewportHeight := m.resizeComponents(m.width, m.height)
	// Keep shell wrapping in sync with the visible terminal pane when sidebar
	// visibility changes.
	m.resizePTYViewport(viewportWidth, viewportHeight)
}

func (m *Model) resizeComponents(width, height int) (viewportWidth, viewportHeight int) {
	viewportHeight = height - 1
	viewportWidth = width

	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, right := splitSidebarWidths(width)
		viewportWidth = left
		m.sidebar.SetSize(right, viewportHeight)
	}

	m.viewport.SetSize(viewportWidth, viewportHeight)
	m.palette.SetSize(width, height)
	resultHeight := height
	if resultHeight > 0 {
		resultHeight--
	}
	m.resultPanel.SetSize(width, resultHeight)
	m.settingsPanel.SetSize(width, height)
	if m.toolApproval != nil {
		m.toolApproval.SetSize(width, height)
	}
	if m.modelPicker != nil {
		m.modelPicker.SetSize(width, height)
	}
	if m.optionPicker != nil {
		m.optionPicker.SetSize(width, height)
	}
	if m.historyPicker != nil {
		m.historyPicker.SetSize(width, height)
	}
	return viewportWidth, viewportHeight
}

func splitSidebarWidths(total int) (left int, right int) {
	const minPaneWidth = 20

	if total <= 0 {
		return 0, 0
	}

	if total < minPaneWidth*2 {
		left = int(float64(total) * 0.6)
		if left < 1 {
			left = 1
		}
		right = total - left
		if right < 1 {
			right = 1
			left = total - right
		}
		return left, right
	}

	left = int(float64(total) * 0.6)
	right = total - left

	if left < minPaneWidth {
		left = minPaneWidth
		right = total - left
	}
	if right < minPaneWidth {
		right = minPaneWidth
		left = total - right
	}

	if left < 1 {
		left = 1
	}
	if right < 1 {
		right = 1
	}

	return left, right
}
