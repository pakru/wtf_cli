package ui

import (
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/historypicker"
	"wtf_cli/pkg/ui/components/palette"
	"wtf_cli/pkg/ui/components/sidebar"
	"wtf_cli/pkg/ui/input"

	tea "charm.land/bubbletea/v2"
)

func (m Model) handleShowPalette() (Model, tea.Cmd) {
	// Show the command palette
	slog.Info("palette_open")
	m.palette.Show()
	m.inputHandler.SetPaletteMode(true)
	return m, nil
}

func (m Model) handleToggleChat() (Model, tea.Cmd) {
	// Ctrl+T pressed - toggle chat sidebar visibility
	m.toggleSidebar("ctrl_t")
	return m, nil
}

func (m Model) handleFocusSwitch() (Model, tea.Cmd) {
	if m.hasBlockingOverlay() {
		return m, nil
	}
	if m.sidebar == nil {
		return m, nil
	}
	if m.sidebar.IsVisible() {
		m.setTerminalFocused(!m.terminalFocused)
		return m, nil
	}
	if !m.sidebar.IsVisible() {
		m.showSidebar("shift_tab")
	}
	return m, nil
}

func (m Model) handlePaletteSelect(msg palette.PaletteSelectMsg) (Model, tea.Cmd) {
	// Command selected from palette
	slog.Info("palette_select", "command", msg.Command)
	m.inputHandler.SetPaletteMode(false)

	// Execute the command
	ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
	handler, ok := m.dispatcher.GetHandler(msg.Command)
	if !ok {
		m.resultPanel.Show("Error", "Unknown command: "+msg.Command)
		return m, nil
	}
	result := handler.Execute(ctx)

	switch result.Action {
	case commands.ResultActionOpenSettings:
		slog.Info("settings_open")
		cfg, _ := config.Load(config.GetConfigPath())
		m.settingsPanel.SetSize(m.width, m.height)
		m.settingsPanel.Show(cfg, config.GetConfigPath())
		if cfg.LLMProvider == "copilot" {
			return m, fetchCopilotAuthStatusCmd(false)
		}
		return m, nil
	case commands.ResultActionOpenHistoryPicker:
		slog.Info("history_picker_from_command")
		// Emit ShowHistoryPickerMsg with empty initial filter
		return m, func() tea.Msg {
			return input.ShowHistoryPickerMsg{InitialFilter: ""}
		}
	case commands.ResultActionToggleChat:
		// Toggle chat sidebar visibility (same as Ctrl+T)
		m.toggleSidebar("chat_command")
		return m, nil
	}

	if streamHandler, ok := handler.(commands.StreamingHandler); ok {
		isExplain := handler.Name() == "/explain"
		if m.sidebar != nil {
			m.sidebar.Show()
			// Focus input so user can start typing immediately
			m.sidebar.FocusInput()
			m.setTerminalFocused(false)
			slog.Info("sidebar_open", "streaming", true)
			m.applyLayout()
		}
		m.streamPlaceholderActive = false
		if isExplain && m.sidebar != nil {
			m.sidebar.AppendUserMessage(m.buildExplainUserMessage(ctx))
			m.sidebar.RefreshView()
		}
		m.startStreamPlaceholder()
		m.streamStartPending = true
		return m, startExplainStreamCmd(ctx, streamHandler, result)
	}

	// Show result in panel
	m.resultPanel.Show(result.Title, result.Content)

	return m, nil
}

func (m Model) handlePaletteCancel() (Model, tea.Cmd) {
	// Palette cancelled
	slog.Info("palette_cancel")
	m.inputHandler.SetPaletteMode(false)
	return m, nil
}

func (m Model) handleShowHistoryPicker(msg input.ShowHistoryPickerMsg) (Model, tea.Cmd) {
	// Show the history picker
	slog.Info("history_picker_open", "initial_filter", msg.InitialFilter)
	// Load bash history + session history
	bashHistory, err := capture.ReadBashHistory(500)
	if err != nil {
		slog.Error("history_picker_load_error", "error", err)
		bashHistory = []string{}
	}
	sessionHistory := []capture.CommandRecord{}
	if m.session != nil {
		sessionHistory = m.session.GetHistory()
	}
	commands := capture.MergeHistory(bashHistory, sessionHistory)

	if m.historyPicker != nil {
		m.historyPicker.SetSize(m.width, m.height)
		m.historyPicker.Show(msg.InitialFilter, commands)
	}
	m.inputHandler.SetHistoryPickerMode(true)
	return m, nil
}

func (m Model) handleHistoryPickerSelect(msg historypicker.HistoryPickerSelectMsg) (Model, tea.Cmd) {
	// Command selected from history picker
	slog.Info("history_picker_select", "command", msg.Command)
	m.inputHandler.SetHistoryPickerMode(false)
	// Replace current prompt content with the selected command (bash-like behavior).
	m.replacePromptCommand(msg.Command)
	return m, nil
}

func (m Model) handleHistoryPickerCancel() (Model, tea.Cmd) {
	// History picker cancelled
	slog.Info("history_picker_cancel")
	m.inputHandler.SetHistoryPickerMode(false)
	return m, nil
}

func (m Model) handleSidebarCommandExecute(msg sidebar.CommandExecuteMsg) (Model, tea.Cmd) {
	cmdText, ok := sidebar.SanitizeCommand(msg.Command)
	if !ok {
		return m, nil
	}
	m.replacePromptCommand(cmdText)
	m.setTerminalFocused(true)
	return m, nil
}

func (m *Model) replacePromptCommand(cmd string) {
	if m.inputHandler == nil {
		return
	}
	m.inputHandler.SendToPTY([]byte{21}) // Ctrl+U clears the line
	m.inputHandler.SendToPTY([]byte(cmd))
	m.inputHandler.SetLineBuffer(cmd)
}

func (m Model) handleCommandSubmitted(msg input.CommandSubmittedMsg) (Model, tea.Cmd) {
	if strings.TrimSpace(msg.Command) == "" {
		return m, nil
	}

	if m.session == nil {
		return m, nil
	}
	m.session.AddCommand(capture.CommandRecord{
		Command:    msg.Command,
		StartTime:  time.Now(),
		EndTime:    time.Now(),
		WorkingDir: m.currentDir,
	})
	return m, nil
}
