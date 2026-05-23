package ui

import (
	"log/slog"

	"wtf_cli/pkg/ui/terminal"
)

func hasFutureEnter(chunks []terminal.AltScreenChunk) bool {
	for _, chunk := range chunks {
		if chunk.Entering {
			return true
		}
	}
	return false
}

func (m *Model) enterFullScreen(dataLen int) {
	slog.Info("fullscreen_enter", "data_len", dataLen)
	m.setScrollMode(false)
	m.fullScreenMode = true
	if m.fullScreenPanel != nil {
		m.fullScreenPanel.Show()
	}
	if m.inputHandler != nil {
		m.inputHandler.SetFullScreenMode(true)
	}
	m.applyLayout()
}

func (m *Model) exitFullScreen() {
	slog.Info("fullscreen_exit")
	m.fullScreenMode = false
	if m.fullScreenPanel != nil {
		m.fullScreenPanel.Hide()
		m.fullScreenPanel.Reset()
	}
	if m.inputHandler != nil {
		m.inputHandler.SetFullScreenMode(false)
	}
	m.applyLayout()
}
