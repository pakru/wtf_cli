package ui

import (
	"context"
	"log/slog"

	"wtf_cli/pkg/logging"
	"wtf_cli/pkg/pty"
	"wtf_cli/pkg/ui/input"

	tea "charm.land/bubbletea/v2"
)

// Key dispatch priority:
// 1. Fullscreen passthrough
// 2. Secret input passthrough
// 3. Exit confirmation cancellation
// 4. Tool approval modal
// 5. Pickers / settings / palette / history overlays
// 6. Result panel
// 7. Focus switch
// 8. Sidebar input
// 9. Terminal scroll keys
// 10. PTY input handler

func (m Model) handlePaste(msg tea.PasteMsg) (Model, tea.Cmd) {
	if msg.Content == "" {
		return m, nil
	}

	if m.fullScreenMode {
		if m.inputHandler != nil {
			m.inputHandler.HandlePaste(msg.Content)
		}
		tracePasteRoute("pty_fullscreen", len(msg.Content))
		return m, nil
	}

	if m.inputHandler != nil {
		secretMode := m.inSecretMode()
		m.inputHandler.SetSecretMode(secretMode)
		if secretMode {
			m.inputHandler.HandlePaste(msg.Content)
			return m, nil
		}
	}

	if m.exitPending {
		m.exitPending = false
		m.statusBar.SetMessage("")
	}

	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		tracePasteRoute("option_picker", len(msg.Content))
		return m, applyPasteToOverlay(msg.Content, m.optionPicker.Update)
	}

	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		tracePasteRoute("model_picker", len(msg.Content))
		return m, applyPasteToOverlay(msg.Content, m.modelPicker.Update)
	}

	if m.settingsPanel.IsVisible() {
		tracePasteRoute("settings_panel", len(msg.Content))
		return m, applyPasteToOverlay(msg.Content, m.settingsPanel.Update)
	}

	if m.toolApproval != nil && m.toolApproval.IsVisible() {
		// Approval popup is modal; ignore pastes until the user picks.
		return m, nil
	}

	if m.resultPanel.IsVisible() {
		tracePasteRoute("result_panel_ignored", len(msg.Content))
		return m, nil
	}

	if m.palette.IsVisible() {
		tracePasteRoute("palette", len(msg.Content))
		return m, applyPasteToOverlay(msg.Content, m.palette.Update)
	}

	if m.historyPicker != nil && m.historyPicker.IsVisible() {
		tracePasteRoute("history_picker", len(msg.Content))
		return m, applyPasteToOverlay(msg.Content, m.historyPicker.Update)
	}

	// Route paste to sidebar input when focused.
	if m.sidebar != nil && m.sidebar.IsVisible() {
		if m.sidebar.IsFocusedOnInput() {
			m.sidebar.HandlePaste(msg.Content)
			return m, nil
		}
	}

	if m.inputHandler != nil {
		m.inputHandler.HandlePaste(msg.Content)
	}
	tracePasteRoute("pty", len(msg.Content))
	return m, nil
}

func tracePasteRoute(target string, n int) {
	logger := slog.Default()
	if logger.Enabled(context.Background(), logging.LevelTrace) {
		logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", target, "len", n)
	}
}

func (m Model) inSecretMode() bool {
	return m.ptyFile != nil && m.secretDetector != nil && m.secretDetector(m.ptyFile)
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// Full-screen mode: bypass all shortcuts, route to PTY
	if m.fullScreenMode {
		// Clear input buffer if echo is disabled (password entry)
		if m.ptyFile != nil && pty.IsEchoDisabled(m.ptyFile) {
			m.inputHandler.ClearLineBuffer()
		}
		handled, cmd := m.inputHandler.HandleKey(msg)
		if handled {
			m.clearTextSelections()
			return m, cmd
		}
		return m, nil // Always return here to avoid fallthrough
	}

	if m.inputHandler != nil {
		secretMode := m.inSecretMode()
		m.inputHandler.SetSecretMode(secretMode)
		if secretMode {
			handled, cmd := m.inputHandler.HandleKey(msg)
			if handled {
				m.clearTextSelections()
				return m, cmd
			}
			return m, nil
		}
	}

	if m.exitPending && msg.String() != "ctrl+d" {
		m.exitPending = false
		m.statusBar.SetMessage("")
	}

	// Priority 4: Tool-approval popup. It's a blocking modal — the agent
	// loop is paused waiting for the user's reply, so it must absorb all
	// keys before any other overlay or PTY routing.
	if m.toolApproval != nil && m.toolApproval.IsVisible() {
		cmd := m.toolApproval.Update(msg)
		return m, cmd
	}

	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		cmd := m.optionPicker.Update(msg)
		return m, cmd
	}

	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		cmd := m.modelPicker.Update(msg)
		return m, cmd
	}

	// Priority 5: Overlays (settings, palette, history picker)
	// These should take precedence even if sidebar is visible
	if m.settingsPanel != nil && m.settingsPanel.IsVisible() {
		cmd := m.settingsPanel.Update(msg)
		return m, cmd
	}

	if m.palette != nil && m.palette.IsVisible() {
		cmd := m.palette.Update(msg)
		return m, cmd
	}

	if m.historyPicker != nil && m.historyPicker.IsVisible() {
		cmd := m.historyPicker.Update(msg)
		return m, cmd
	}

	// Priority 6: Result panel
	if m.resultPanel.IsVisible() {
		cmd := m.resultPanel.Update(msg)
		return m, cmd
	}

	// Intercept Shift+Tab before sidebar/PTY routing so focus switching works
	// regardless of current focus target.
	if msg.String() == "shift+tab" {
		return m, func() tea.Msg {
			return input.FocusSwitchMsg{}
		}
	}

	// Priority 8: Sidebar input handling.
	// This runs AFTER overlays and result panel, so they take precedence
	if m.sidebar != nil && m.sidebar.IsVisible() {
		if !m.terminalFocused {
			wasVisible := m.sidebar.IsVisible()
			if cmd := m.sidebar.Update(msg); cmd != nil {
				return m, cmd
			}
			if wasVisible && !m.sidebar.IsVisible() {
				slog.Info("sidebar_close", "reason", "key")
				m.setTerminalFocused(true)
				m.applyLayout()
				return m, nil
			}
			// If sidebar handled key, don't fall through
			if m.sidebar.ShouldHandleKey(msg) {
				return m, nil
			}
		}
	}

	// Use input handler to route keys to PTY
	// Note: ClearLineBuffer for password entry is only done in fullscreen mode
	// where programs like sudo actually disable echo. In normal mode, BubbleTea's
	// raw terminal mode disables echo, but that's not password entry.

	// Scroll-key interception — only when terminal has focus and not in full-screen mode.
	// Handled here (not in InputHandler) so sidebar focus is respected automatically.
	// Alt+Up/Down are used instead of Shift+Up/Down because Konsole and most terminal
	// emulators intercept the Shift variants for their own scrollback.
	if m.terminalFocused && !m.fullScreenMode {
		switch msg.String() {
		case "alt+up":
			m.viewport.ScrollUp()
			if !m.viewport.IsAtBottom() {
				m.setScrollMode(true)
			}
			return m, nil
		case "alt+down":
			m.viewport.ScrollDown()
			if m.viewport.IsAtBottom() {
				m.setScrollMode(false)
			}
			return m, nil
		case "pgup":
			m.viewport.PageUp()
			if !m.viewport.IsAtBottom() {
				m.setScrollMode(true)
			}
			return m, nil
		case "pgdown":
			m.viewport.PageDown()
			if m.viewport.IsAtBottom() {
				m.setScrollMode(false)
			}
			return m, nil
		case "esc":
			if m.scrollMode {
				m.setScrollMode(false)
				return m, nil
			}
		}
	}

	handled, cmd := m.inputHandler.HandleKey(msg)
	if handled {
		m.clearTextSelections()
		// If the user types anything that reaches the PTY or triggers a UI
		// action, exit scroll mode so they see the output of what they typed.
		if m.scrollMode {
			m.setScrollMode(false)
		}
		return m, cmd
	}

	// If not handled by input handler, ignore
	return m, nil
}

func applyPasteToOverlay(content string, update func(tea.KeyPressMsg) tea.Cmd) tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(content))
	for _, r := range content {
		msg := tea.KeyPressMsg(tea.Key{Code: r, Text: string(r)})
		if cmd := update(msg); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}
