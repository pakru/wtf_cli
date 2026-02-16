package ui

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/logging"
	"wtf_cli/pkg/pty"
	"wtf_cli/pkg/ui/components/fullscreen"
	"wtf_cli/pkg/ui/components/historypicker"
	"wtf_cli/pkg/ui/components/palette"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/components/result"
	"wtf_cli/pkg/ui/components/settings"
	"wtf_cli/pkg/ui/components/sidebar"
	"wtf_cli/pkg/ui/components/statusbar"
	"wtf_cli/pkg/ui/components/viewport"
	"wtf_cli/pkg/ui/components/welcome"
	"wtf_cli/pkg/ui/input"
	"wtf_cli/pkg/ui/render"
	"wtf_cli/pkg/ui/terminal"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

const streamThinkingPlaceholder = "Thinking..."

// Model represents the Bubble Tea application state
type Model struct {
	// PTY connection
	ptyFile *os.File
	cwdFunc func() (string, error) // Function to get shell's cwd
	// secretDetector checks whether the PTY is in canonical secret-input mode.
	// Injectable for tests.
	secretDetector func(*os.File) bool

	// UI Components
	viewport      viewport.PTYViewport              // Viewport for PTY output
	statusBar     *statusbar.StatusBarView          // Status bar at bottom
	inputHandler  *input.InputHandler               // Input routing to PTY
	palette       *palette.CommandPalette           // Command palette overlay
	historyPicker *historypicker.HistoryPickerPanel // History search picker
	resultPanel   *result.ResultPanel               // Result panel overlay
	settingsPanel *settings.SettingsPanel           // Settings panel overlay
	modelPicker   *picker.ModelPickerPanel
	optionPicker  *picker.OptionPickerPanel
	sidebar       *sidebar.Sidebar // Sidebar for AI suggestions

	// Command system
	dispatcher *commands.Dispatcher

	// Data
	buffer     *buffer.CircularBuffer
	session    *capture.SessionContext
	currentDir string
	gitBranch  string

	// gitBranchResolver resolves a git branch label from a directory path.
	// Injectable for tests.
	gitBranchResolver func(string) string

	// Streaming state
	wtfStream               <-chan commands.WtfStreamEvent
	streamPlaceholderActive bool
	streamStartPending      bool

	// UI state
	width           int
	height          int
	ready           bool
	terminalFocused bool

	exitPending   bool
	exitConfirmID int

	resizeDebounceID int       // Counter to debounce resize events
	resizeTime       time.Time // When last PTY resize occurred (to suppress prompt reprint)
	initialResize    bool      // Track if we've done the initial resize

	ptyNormalizer *terminal.Normalizer

	// PTY output batching
	ptyBatchBuffer  []byte        // Accumulated PTY data
	ptyBatchTimer   bool          // Whether flush timer is pending
	ptyBatchMaxSize int           // Max bytes before forced flush (default: 16KB)
	ptyBatchMaxWait time.Duration // Max time before flush (default: 16ms)

	// Stream update throttling
	streamThrottlePending bool
	streamThrottleDelay   time.Duration // Default: 50ms

	// Full-screen app support (vim, nano, htop)
	fullScreenMode  bool
	fullScreenPanel *fullscreen.FullScreenPanel
	altScreenState  *terminal.AltScreenState
}

// NewModel creates a new Bubble Tea model
func NewModel(ptyFile *os.File, buf *buffer.CircularBuffer, sess *capture.SessionContext, cwdFunc func() (string, error)) Model {
	initialDir := getCurrentDir()
	// Try to get initial dir from cwd function
	if cwdFunc != nil {
		if cwd, err := cwdFunc(); err == nil {
			initialDir = cwd
		}
	}

	// Create viewport and add welcome message at the start
	viewport := viewport.NewPTYViewport()
	viewport.AppendOutput([]byte(welcome.WelcomeMessage()))

	statusBar := statusbar.NewStatusBarView()
	statusBar.SetModel(loadModelFromConfig())

	return Model{
		ptyFile:        ptyFile,
		cwdFunc:        cwdFunc,
		secretDetector: pty.IsSecretInputMode,
		viewport:       viewport,
		statusBar:      statusBar,
		inputHandler:   input.NewInputHandler(ptyFile),
		palette:        palette.NewCommandPalette(),
		historyPicker:  historypicker.NewHistoryPickerPanel(),
		resultPanel:    result.NewResultPanel(),
		settingsPanel:  settings.NewSettingsPanel(),
		modelPicker:    picker.NewModelPickerPanel(),
		optionPicker:   picker.NewOptionPickerPanel(),
		sidebar:        sidebar.NewSidebar(),
		dispatcher:     commands.NewDispatcher(),
		buffer:         buf,
		session:        sess,
		currentDir:     initialDir,

		gitBranchResolver:   statusbar.ResolveGitBranch,
		fullScreenPanel:     fullscreen.NewFullScreenPanel(80, 24),
		altScreenState:      terminal.NewAltScreenState(),
		ptyNormalizer:       terminal.NewNormalizer(),
		ptyBatchMaxSize:     16384,                 // 16KB
		ptyBatchMaxWait:     16 * time.Millisecond, // ~60fps
		streamThrottleDelay: 50 * time.Millisecond, // Throttle stream updates
		terminalFocused:     true,
	}
}

// Init initializes the model (Bubble Tea lifecycle method)
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenToPTY(m.ptyFile), // Start listening to PTY output
		tickDirectory(),        // Start directory update ticker
		resolveGitBranchCmd(m.currentDir, m.gitBranchResolver),
	)
}

// tickDirectory creates a command that periodically updates directory
func tickDirectory() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return directoryUpdateMsg{}
	})
}

type directoryUpdateMsg struct{}

type gitBranchMsg struct {
	dir    string
	branch string
}

type exitConfirmTimeoutMsg struct {
	id int
}

type resizeApplyMsg struct {
	id     int
	width  int
	height int
}

// Update handles messages and updates model state (Bubble Tea lifecycle method)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		slog.Debug("window_resize", "width", m.width, "height", m.height)

		// Update UI component sizes immediately for display
		viewportHeight := m.height - 1
		viewportWidth := m.width
		if m.sidebar != nil && m.sidebar.IsVisible() {
			left, right := splitSidebarWidths(m.width)
			viewportWidth = left
			m.sidebar.SetSize(right, viewportHeight)
		}
		m.viewport.SetSize(viewportWidth, viewportHeight)
		m.palette.SetSize(m.width, m.height)
		resultHeight := m.height
		if resultHeight > 0 {
			resultHeight--
		}
		m.resultPanel.SetSize(m.width, resultHeight)
		m.settingsPanel.SetSize(m.width, m.height)
		if m.modelPicker != nil {
			m.modelPicker.SetSize(m.width, m.height)
		}
		if m.optionPicker != nil {
			m.optionPicker.SetSize(m.width, m.height)
		}
		if m.historyPicker != nil {
			m.historyPicker.SetSize(m.width, m.height)
		}
		if m.fullScreenMode && m.fullScreenPanel != nil {
			m.fullScreenPanel.Resize(m.width, m.height)
		}

		// Debounce PTY resize to avoid multiple prompt reprints during drag
		m.resizeDebounceID++
		resizeID := m.resizeDebounceID
		return m, tea.Tick(150*time.Millisecond, func(time.Time) tea.Msg {
			return resizeApplyMsg{id: resizeID, width: msg.Width, height: msg.Height}
		})

	case resizeApplyMsg:
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
				terminal.ResizePTY(m.ptyFile, viewportWidth, viewportHeight)
				// Track resize time to suppress prompt reprint output
				// Skip suppression on initial resize (first time we get correct size)
				if m.initialResize {
					m.resizeTime = time.Now()
				}
				m.initialResize = true
			}
		}
		return m, nil

	case tea.MouseMsg:
		// Route to sidebar when visible.
		if m.sidebar != nil && m.sidebar.IsVisible() {
			cmd := m.sidebar.HandleMouse(msg)
			return m, cmd
		}
		return m, nil

	case tea.PasteMsg:
		if msg.Content == "" {
			return m, nil
		}

		if m.fullScreenMode {
			if m.inputHandler != nil {
				m.inputHandler.HandlePaste(msg.Content)
			}
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "pty_fullscreen", "len", len(msg.Content))
			}
			return m, nil
		}

		if m.inputHandler != nil {
			secretMode := m.ptyFile != nil && m.secretDetector != nil && m.secretDetector(m.ptyFile)
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
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "option_picker", "len", len(msg.Content))
			}
			return m, applyPasteToOverlay(msg.Content, m.optionPicker.Update)
		}

		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "model_picker", "len", len(msg.Content))
			}
			return m, applyPasteToOverlay(msg.Content, m.modelPicker.Update)
		}

		if m.settingsPanel.IsVisible() {
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "settings_panel", "len", len(msg.Content))
			}
			return m, applyPasteToOverlay(msg.Content, m.settingsPanel.Update)
		}

		if m.resultPanel.IsVisible() {
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "result_panel_ignored", "len", len(msg.Content))
			}
			return m, nil
		}

		if m.palette.IsVisible() {
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "palette", "len", len(msg.Content))
			}
			return m, applyPasteToOverlay(msg.Content, m.palette.Update)
		}

		if m.historyPicker != nil && m.historyPicker.IsVisible() {
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "history_picker", "len", len(msg.Content))
			}
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
		logger := slog.Default()
		if logger.Enabled(context.Background(), logging.LevelTrace) {
			logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", "pty", "len", len(msg.Content))
		}
		return m, nil

	case tea.KeyPressMsg:
		// Full-screen mode: bypass all shortcuts, route to PTY
		if m.fullScreenMode {
			// Clear input buffer if echo is disabled (password entry)
			if m.ptyFile != nil && pty.IsEchoDisabled(m.ptyFile) {
				m.inputHandler.ClearLineBuffer()
			}
			handled, cmd := m.inputHandler.HandleKey(msg)
			if handled {
				return m, cmd
			}
			return m, nil // Always return here to avoid fallthrough
		}

		if m.inputHandler != nil {
			secretMode := m.ptyFile != nil && m.secretDetector != nil && m.secretDetector(m.ptyFile)
			m.inputHandler.SetSecretMode(secretMode)
			if secretMode {
				handled, cmd := m.inputHandler.HandleKey(msg)
				if handled {
					return m, cmd
				}
				return m, nil
			}
		}

		if m.exitPending && msg.String() != "ctrl+d" {
			m.exitPending = false
			m.statusBar.SetMessage("")
		}

		if m.optionPicker != nil && m.optionPicker.IsVisible() {
			cmd := m.optionPicker.Update(msg)
			return m, cmd
		}

		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			cmd := m.modelPicker.Update(msg)
			return m, cmd
		}

		// Priority 1: Overlays (settings, palette, history picker)
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

		// Priority 2: Result panel
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

		// Priority 3: Sidebar input handling.
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
		handled, cmd := m.inputHandler.HandleKey(msg)
		if handled {
			return m, cmd
		}

		// If not handled by input handler, ignore
		return m, nil

	case input.ShowPaletteMsg:
		// Show the command palette
		slog.Info("palette_open")
		m.palette.Show()
		m.inputHandler.SetPaletteMode(true)
		return m, nil

	case input.ToggleChatMsg:
		// Ctrl+T pressed - toggle chat sidebar visibility
		if m.sidebar != nil {
			if m.sidebar.IsVisible() {
				// Hide sidebar
				m.sidebar.Hide()
				slog.Info("sidebar_close", "reason", "ctrl_t")
				m.setTerminalFocused(true)
				m.applyLayout()
			} else {
				// Show sidebar, preserve existing title.
				title := m.sidebar.GetTitle()
				if title == "" {
					title = "WTF Analysis"
				}
				m.sidebar.Show(title, "")
				m.sidebar.FocusInput()
				m.setTerminalFocused(false)
				slog.Info("sidebar_open", "reason", "ctrl_t", "title", title)
				m.applyLayout()
			}
		}
		return m, nil

	case input.FocusSwitchMsg:
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
			title := m.sidebar.GetTitle()
			if title == "" {
				title = "WTF Analysis"
			}
			m.sidebar.Show(title, "")
			m.sidebar.FocusInput()
			m.setTerminalFocused(false)
			slog.Info("sidebar_open", "reason", "shift_tab", "title", title)
			m.applyLayout()
		}
		return m, nil

	case palette.PaletteSelectMsg:
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
			m.statusBar.SetModel(cfg.OpenRouter.Model)
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
			if m.sidebar != nil {
				if m.sidebar.IsVisible() {
					m.sidebar.Hide()
					slog.Info("sidebar_close", "reason", "chat_command")
					m.setTerminalFocused(true)
				} else {
					// Preserve existing title
					title := m.sidebar.GetTitle()
					if title == "" {
						title = "WTF Analysis"
					}
					m.sidebar.Show(title, "")
					m.sidebar.FocusInput()
					m.setTerminalFocused(false)
					slog.Info("sidebar_open", "reason", "chat_command", "title", title)
				}
				m.applyLayout()
			}
			return m, nil
		}

		if streamHandler, ok := handler.(commands.StreamingHandler); ok {
			isExplain := handler.Name() == "/explain"
			if m.sidebar != nil {
				m.sidebar.Show(result.Title, "")
				// Focus input so user can start typing immediately
				m.sidebar.FocusInput()
				m.setTerminalFocused(false)
				slog.Info("sidebar_open", "title", result.Title, "streaming", true)
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

	case palette.PaletteCancelMsg:
		// Palette cancelled
		slog.Info("palette_cancel")
		m.inputHandler.SetPaletteMode(false)
		return m, nil

	case input.ShowHistoryPickerMsg:
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

	case historypicker.HistoryPickerSelectMsg:
		// Command selected from history picker
		slog.Info("history_picker_select", "command", msg.Command)
		m.inputHandler.SetHistoryPickerMode(false)
		// Replace current prompt content with the selected command (bash-like behavior).
		if m.inputHandler != nil {
			m.inputHandler.SendToPTY([]byte{21}) // Ctrl+U clears the line
			m.inputHandler.SendToPTY([]byte(msg.Command))
			m.inputHandler.SetLineBuffer(msg.Command)
		}
		return m, nil

	case historypicker.HistoryPickerCancelMsg:
		// History picker cancelled
		slog.Info("history_picker_cancel")
		m.inputHandler.SetHistoryPickerMode(false)
		return m, nil

	case sidebar.CommandExecuteMsg:
		cmdText, ok := sidebar.SanitizeCommand(msg.Command)
		if !ok {
			return m, nil
		}
		if m.inputHandler != nil {
			m.inputHandler.SendToPTY([]byte{21}) // Ctrl+U clears the line
			m.inputHandler.SendToPTY([]byte(cmdText))
			m.inputHandler.SetLineBuffer(cmdText)
		}
		m.setTerminalFocused(true)
		return m, nil

	case input.CommandSubmittedMsg:
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

	case settings.SettingsCloseMsg:
		// Settings panel closed
		slog.Info("settings_close")
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.Hide()
		}
		if m.optionPicker != nil && m.optionPicker.IsVisible() {
			m.optionPicker.Hide()
		}
		return m, nil

	case settings.StartCopilotAuthMsg:
		slog.Info("copilot_auth_status_request")
		return m, fetchCopilotAuthStatusCmd(true)

	case copilotAuthStatusMsg:
		summary, detail, message := formatCopilotAuthStatus(msg.Status, msg.Err)
		if m.settingsPanel != nil {
			m.settingsPanel.UpdateCopilotAuthStatus(summary, detail)
			if msg.ShowPrompt {
				m.settingsPanel.SetCopilotAuthMessage(message)
			}
		}
		return m, nil

	case settings.SettingsSaveMsg:
		// Save settings to file
		if err := config.Save(msg.ConfigPath, msg.Config); err != nil {
			slog.Error("settings_save_error", "error", err)
		} else {
			slog.Info("settings_save",
				"provider", msg.Config.LLMProvider,
				"model", getModelForProvider(msg.Config),
				"log_level", msg.Config.LogLevel,
				"log_format", msg.Config.LogFormat,
				"log_file", msg.Config.LogFile,
			)
			logging.SetLevel(msg.Config.LogLevel)
		}
		m.statusBar.SetModel(getModelForProvider(msg.Config))
		return m, nil

	case input.CtrlDPressedMsg:
		if m.exitPending {
			m.exitPending = false
			m.statusBar.SetMessage("")
			if m.inputHandler != nil {
				if err := m.inputHandler.SendToPTY([]byte{4}); err != nil {
					slog.Error("exit_send_eof_error", "error", err)
				}
			}
			return m, tea.Quit
		}
		m.exitPending = true
		m.exitConfirmID++
		confirmID := m.exitConfirmID
		m.statusBar.SetMessage("Press Ctrl+D again to exit")
		return m, tea.Tick(2*time.Second, func(time.Time) tea.Msg {
			return exitConfirmTimeoutMsg{id: confirmID}
		})

	case exitConfirmTimeoutMsg:
		if m.exitPending && msg.id == m.exitConfirmID {
			m.exitPending = false
			m.statusBar.SetMessage("")
		}
		return m, nil

	case picker.OpenModelPickerMsg:
		slog.Info("model_picker_open", "current", msg.Current, "field_key", msg.FieldKey, "cached_models", len(msg.Options))
		slog.Debug("model_picker_open_details",
			"field_key", msg.FieldKey,
			"api_url", msg.APIURL,
			"has_api_key", msg.APIKey != "",
		)
		if m.modelPicker != nil {
			m.modelPicker.SetSize(m.width, m.height)
			m.modelPicker.Show(msg.Options, msg.Current, msg.FieldKey)
		}
		// Fetch dynamic model list based on provider
		var cmd tea.Cmd
		switch msg.FieldKey {
		case "model":
			if msg.APIURL != "" {
				cmd = refreshModelCacheCmd(msg.APIURL)
			} else {
				slog.Debug("model_picker_no_api_url")
			}
		case "openai_model":
			if msg.APIKey != "" {
				cmd = fetchOpenAIModelsCmd(msg.APIKey)
			} else {
				slog.Debug("openai_models_fetch_skipped", "reason", "missing_api_key")
			}
		case "copilot_model":
			cmd = fetchCopilotModelsCmd()
		case "anthropic_model":
			if msg.APIKey != "" {
				cmd = fetchAnthropicModelsCmd(msg.APIKey)
			} else {
				slog.Debug("anthropic_models_fetch_skipped", "reason", "missing_api_key")
			}
		case "google_model":
			if msg.APIKey != "" {
				cmd = fetchGoogleModelsCmd(msg.APIKey)
			} else {
				slog.Debug("google_models_fetch_skipped", "reason", "missing_api_key")
			}
		}
		return m, cmd

	case picker.ModelPickerSelectMsg:
		slog.Info("model_picker_select", "model", msg.ModelID, "field_key", msg.FieldKey)
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.Hide()
		}
		if m.settingsPanel != nil {
			switch msg.FieldKey {
			case "model":
				m.settingsPanel.SetModelValue(msg.ModelID)
			case "openai_model":
				m.settingsPanel.SetOpenAIModelValue(msg.ModelID)
			case "copilot_model":
				m.settingsPanel.SetCopilotModelValue(msg.ModelID)
			case "anthropic_model":
				m.settingsPanel.SetAnthropicModelValue(msg.ModelID)
			case "google_model":
				m.settingsPanel.SetGoogleModelValue(msg.ModelID)
			default:
				// Fallback to OpenRouter model for backwards compatibility
				m.settingsPanel.SetModelValue(msg.ModelID)
			}
		}
		return m, nil

	case picker.OpenOptionPickerMsg:
		slog.Info("option_picker_open", "field", msg.FieldKey, "current", msg.Current)
		if m.optionPicker != nil {
			m.optionPicker.SetSize(m.width, m.height)
			m.optionPicker.Show(msg.Title, msg.FieldKey, msg.Options, msg.Current)
		}
		return m, nil

	case picker.OptionPickerSelectMsg:
		slog.Info("option_picker_select", "field", msg.FieldKey, "value", msg.Value)
		if m.optionPicker != nil && m.optionPicker.IsVisible() {
			m.optionPicker.Hide()
		}
		if m.settingsPanel != nil {
			switch msg.FieldKey {
			case "llm_provider":
				m.settingsPanel.SetProviderValue(msg.Value)
				if msg.Value == "copilot" {
					return m, fetchCopilotAuthStatusCmd(false)
				}
			case "log_level":
				m.settingsPanel.SetLogLevelValue(msg.Value)
			case "log_format":
				m.settingsPanel.SetLogFormatValue(msg.Value)
			}
		}
		return m, nil

	case picker.ModelPickerRefreshMsg:
		if msg.Err != nil {
			slog.Error("model_picker_refresh_error", "error", msg.Err)
			return m, nil
		}
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.UpdateOptions(msg.Cache.Models)
		}
		if m.settingsPanel != nil {
			m.settingsPanel.SetModelCache(msg.Cache)
		}
		slog.Info("model_picker_refresh_done", "models", len(msg.Cache.Models))
		return m, nil

	case providerModelsRefreshMsg:
		if msg.Err != nil {
			slog.Error("provider_models_refresh_error", "field_key", msg.FieldKey, "error", msg.Err)
			return m, nil
		}
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.UpdateOptions(msg.Models)
		}
		slog.Info("provider_models_refresh_done", "field_key", msg.FieldKey, "models", len(msg.Models))
		return m, nil

	case streamStartResultMsg:
		m.streamStartPending = false
		if msg.err != nil {
			slog.Error("wtf_stream_start_error", "error", msg.err)
			if m.sidebar != nil {
				m.sidebar.SetStreaming(false)
				m.clearStreamPlaceholder()
				m.sidebar.AppendErrorMessage(msg.err.Error())
				m.sidebar.RefreshView()
			} else {
				m.resultPanel.Show("Error", fmt.Sprintf("Error: %v", msg.err))
			}
			m.wtfStream = nil
			m.streamPlaceholderActive = false
			return m, nil
		}

		if msg.stream == nil {
			if m.sidebar != nil {
				m.sidebar.SetStreaming(false)
				m.clearStreamPlaceholder()
				if msg.origin == streamOriginExplain && msg.result != nil {
					m.sidebar.StartAssistantMessageWithContent(msg.result.Content)
					m.sidebar.RefreshView()
				}
			} else if msg.origin == streamOriginExplain && msg.result != nil {
				m.resultPanel.Show(msg.result.Title, msg.result.Content)
			}
			m.wtfStream = nil
			m.streamPlaceholderActive = false
			return m, nil
		}

		m.wtfStream = msg.stream
		return m, listenToWtfStream(m.wtfStream)

	case result.ResultPanelCloseMsg:
		// Result panel closed
		return m, nil

	case sidebar.ChatSubmitMsg:
		if m.sidebar == nil || msg.Content == "" {
			return m, nil
		}

		// Guard: refuse new stream while one is active (prevents deadlock)
		if m.wtfStream != nil || m.streamStartPending {
			return m, nil
		}

		// Add user message to sidebar history
		m.sidebar.AppendUserMessage(msg.Content)
		m.sidebar.RefreshView()

		// Build context and start chat stream
		ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
		history := append([]ai.ChatMessage(nil), m.sidebar.GetMessages()...)
		m.streamStartPending = true
		m.streamPlaceholderActive = false
		m.startStreamPlaceholder()
		return m, startChatStreamCmd(ctx, history)

	case commands.WtfStreamEvent:
		if msg.Err != nil {
			slog.Error("wtf_stream_error", "error", msg.Err)
			// Clear all stream state (guard nil)
			if m.sidebar != nil {
				m.sidebar.SetStreaming(false)
				m.clearStreamPlaceholder()
				m.sidebar.AppendErrorMessage(msg.Err.Error())
				m.sidebar.RefreshView() // Ensure error is visible immediately
			}
			m.wtfStream = nil
			m.streamThrottlePending = false
			m.streamPlaceholderActive = false
			return m, nil
		}

		if m.sidebar != nil {
			if msg.Delta != "" {
				// Ensure streaming state is active
				if !m.sidebar.IsStreaming() {
					m.sidebar.SetStreaming(true)
				}

				// Replace placeholder on first real delta
				if !m.replaceStreamPlaceholder(msg.Delta) {
					m.sidebar.UpdateLastMessage(msg.Delta)
				}

				// Throttle rendering
				if !m.streamThrottlePending {
					m.streamThrottlePending = true
					// Immediate refresh on first chunk for responsiveness
					m.sidebar.RefreshView()
					return m, tea.Batch(
						tea.Tick(m.streamThrottleDelay, func(time.Time) tea.Msg {
							return streamThrottleFlushMsg{}
						}),
						listenToWtfStream(m.wtfStream),
					)
				}
				// Subsequent chunks: just listen, don't schedule another tick
				return m, listenToWtfStream(m.wtfStream)
			}
			if msg.Done {
				m.clearStreamPlaceholder()
				m.sidebar.SetStreaming(false)
				m.sidebar.RefreshView() // Final refresh
				m.wtfStream = nil
				m.streamThrottlePending = false
				m.streamPlaceholderActive = false
				return m, nil
			}
		}
		if m.wtfStream != nil {
			return m, listenToWtfStream(m.wtfStream)
		}
		return m, nil

	case streamThrottleFlushMsg:
		m.streamThrottlePending = false

		// Re-render from chat messages.
		if m.sidebar != nil {
			m.sidebar.RefreshView() // Re-renders viewport from messages[]
		}
		return m, nil

	case ptyOutputMsg:
		// Suppress PTY output briefly after resize to prevent prompt reprint from showing
		if !m.resizeTime.IsZero() && time.Since(m.resizeTime) < 100*time.Millisecond {
			// Skip appending to viewport but still schedule next read
			return m, listenToPTY(m.ptyFile)
		}

		// Append to batch buffer
		m.ptyBatchBuffer = append(m.ptyBatchBuffer, msg.data...)

		// Force flush if buffer exceeds threshold
		if len(m.ptyBatchBuffer) >= m.ptyBatchMaxSize {
			m.flushPTYBatch()
			return m, listenToPTY(m.ptyFile)
		}

		// Start flush timer if not already pending
		var flushCmd tea.Cmd
		if !m.ptyBatchTimer {
			m.ptyBatchTimer = true
			flushCmd = tea.Tick(m.ptyBatchMaxWait, func(time.Time) tea.Msg {
				return ptyBatchFlushMsg{}
			})
		}

		return m, tea.Batch(flushCmd, listenToPTY(m.ptyFile))

	case ptyBatchFlushMsg:
		m.ptyBatchTimer = false
		if len(m.ptyBatchBuffer) > 0 {
			m.flushPTYBatch()
		}
		return m, nil

	case ptyErrorMsg:
		// PTY error - probably shell exited
		slog.Error("pty_error", "error", msg.err)
		return m, tea.Quit

	case directoryUpdateMsg:
		// Update current directory from shell process
		if m.cwdFunc != nil {
			if cwd, err := m.cwdFunc(); err == nil {
				m.currentDir = cwd
			}
		}
		// Always resolve git branch on every tick — the resolver is cheap
		// (reads .git/HEAD) and this ensures branch changes from commands
		// like `git checkout` are reflected promptly.
		branchCmd := resolveGitBranchCmd(m.currentDir, m.gitBranchResolver)
		// Schedule next update
		return m, tea.Batch(tickDirectory(), branchCmd)

	case gitBranchMsg:
		if msg.dir == m.currentDir {
			m.gitBranch = msg.branch
		}
		return m, nil
	}

	return m, nil
}

// View renders the UI (Bubble Tea lifecycle method)
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

	// Enable mouse wheel when sidebar is visible.
	if m.sidebar != nil && m.sidebar.IsVisible() {
		// Mouse mode is disabled until HandleMouse implements wheel scrolling
		// TODO: Re-enable once mouse scrolling is properly implemented in sidebar
		// v.MouseMode = tea.MouseModeCellMotion
	}

	v.SetContent(m.renderCanvas())
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

// Helper functions

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

	viewportHeight := m.height - 1
	viewportWidth := m.width

	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, right := splitSidebarWidths(m.width)
		viewportWidth = left
		m.sidebar.SetSize(right, viewportHeight)
	}

	m.viewport.SetSize(viewportWidth, viewportHeight)
	m.palette.SetSize(m.width, m.height)
	resultHeight := m.height
	if resultHeight > 0 {
		resultHeight--
	}
	m.resultPanel.SetSize(m.width, resultHeight)
	m.settingsPanel.SetSize(m.width, m.height)
	if m.modelPicker != nil {
		m.modelPicker.SetSize(m.width, m.height)
	}
	if m.optionPicker != nil {
		m.optionPicker.SetSize(m.width, m.height)
	}
	// NOTE: PTY resize is handled by the debounced resizeApplyMsg handler
	// to avoid duplicate prompts during terminal resize
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

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "~"
	}
	return dir
}

func loadModelFromConfig() string {
	path := config.GetConfigPath()
	if path == "" {
		return config.Default().OpenRouter.Model
	}
	if _, err := os.Stat(path); err != nil {
		return config.Default().OpenRouter.Model
	}
	cfg, err := config.Load(path)
	if err != nil {
		return config.Default().OpenRouter.Model
	}
	return getModelForProvider(cfg)
}

// getModelForProvider returns the model name for the currently selected provider
func getModelForProvider(cfg config.Config) string {
	switch cfg.LLMProvider {
	case "openai":
		if cfg.Providers.OpenAI.Model != "" {
			return cfg.Providers.OpenAI.Model
		}
		return "gpt-4o"
	case "copilot":
		if cfg.Providers.Copilot.Model != "" {
			return cfg.Providers.Copilot.Model
		}
		return "gpt-4o"
	case "anthropic":
		if cfg.Providers.Anthropic.Model != "" {
			return cfg.Providers.Anthropic.Model
		}
		return "claude-3-5-sonnet-20241022"
	case "google":
		if cfg.Providers.Google.Model != "" {
			return cfg.Providers.Google.Model
		}
		return "gemini-3-flash-preview"
	default: // openrouter or unknown
		if cfg.OpenRouter.Model != "" {
			return cfg.OpenRouter.Model
		}
		return config.Default().OpenRouter.Model
	}
}

func refreshModelCacheCmd(apiURL string) tea.Cmd {
	trimmed := strings.TrimSpace(apiURL)
	if trimmed == "" {
		return nil
	}

	return func() tea.Msg {
		slog.Info("model_picker_refresh_start", "api_url", trimmed)
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		cache, err := ai.RefreshOpenRouterModelCache(ctx, trimmed, ai.DefaultModelCachePath())
		return picker.ModelPickerRefreshMsg{Cache: cache, Err: err}
	}
}

// providerModelsRefreshMsg is sent when dynamic model fetching completes
type providerModelsRefreshMsg struct {
	Models   []ai.ModelInfo
	FieldKey string
	Err      error
}

func fetchOpenAIModelsCmd(apiKey string) tea.Cmd {
	if apiKey == "" {
		return nil
	}

	return func() tea.Msg {
		slog.Info("openai_models_fetch_start")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		models, err := ai.FetchOpenAIModels(ctx, apiKey)
		return providerModelsRefreshMsg{Models: models, FieldKey: "openai_model", Err: err}
	}
}

func fetchAnthropicModelsCmd(apiKey string) tea.Cmd {
	if apiKey == "" {
		return nil
	}

	return func() tea.Msg {
		slog.Info("anthropic_models_fetch_start")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		models, err := ai.FetchAnthropicModels(ctx, apiKey)
		return providerModelsRefreshMsg{Models: models, FieldKey: "anthropic_model", Err: err}
	}
}

func fetchGoogleModelsCmd(apiKey string) tea.Cmd {
	if apiKey == "" {
		return nil
	}

	return func() tea.Msg {
		slog.Info("google_models_fetch_start")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		models, err := ai.FetchGoogleModels(ctx, apiKey)
		return providerModelsRefreshMsg{Models: models, FieldKey: "google_model", Err: err}
	}
}

func fetchCopilotModelsCmd() tea.Cmd {
	return func() tea.Msg {
		slog.Info("copilot_models_fetch_start")
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		models, err := ai.FetchCopilotModels(ctx)
		return providerModelsRefreshMsg{Models: models, FieldKey: "copilot_model", Err: err}
	}
}

func (m Model) renderCanvas() *lipgloss.Canvas {
	const (
		baseLayerZ     = 0
		settingsLayerZ = 1
		overlayLayerZ  = 2
	)

	if m.width <= 0 || m.height <= 0 {
		return lipgloss.NewCanvas()
	}

	// Update status bar width and directory for this frame.
	m.statusBar.SetWidth(m.width)
	m.statusBar.SetDirectory(m.currentDir)
	m.statusBar.SetGitBranch(m.gitBranch)

	viewportHeight := render.ViewportHeight(m.height)
	viewportWidth := m.width
	sidebarWidth := 0
	if m.sidebar != nil && m.sidebar.IsVisible() {
		left, right := splitSidebarWidths(m.width)
		viewportWidth = left
		sidebarWidth = right
	}

	layers := make([]*lipgloss.Layer, 0, 5)

	if viewportWidth > 0 && viewportHeight > 0 {
		viewportLayer := lipgloss.NewLayer(m.viewport.View()).
			X(0).Y(0).
			Width(viewportWidth).Height(viewportHeight).
			Z(baseLayerZ)
		layers = append(layers, viewportLayer)
	}

	if sidebarWidth > 0 && viewportHeight > 0 && m.sidebar != nil && m.sidebar.IsVisible() {
		sidebarLayer := lipgloss.NewLayer(m.sidebar.View()).
			X(viewportWidth).Y(0).
			Width(sidebarWidth).Height(viewportHeight).
			Z(baseLayerZ)
		layers = append(layers, sidebarLayer)
	}

	statusLayer := lipgloss.NewLayer(m.statusBar.Render()).
		X(0).Y(viewportHeight).
		Width(m.width).Height(1).
		Z(baseLayerZ)
	layers = append(layers, statusLayer)

	if m.settingsPanel.IsVisible() {
		layers = addOverlayLayer(layers, m.settingsPanel.View(), m.width, m.height, settingsLayerZ)
	}

	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		layers = addOverlayLayer(layers, m.optionPicker.View(), m.width, m.height, overlayLayerZ)
	} else if m.modelPicker != nil && m.modelPicker.IsVisible() {
		layers = addOverlayLayer(layers, m.modelPicker.View(), m.width, m.height, overlayLayerZ)
	} else if m.resultPanel.IsVisible() {
		layers = addOverlayLayer(layers, m.resultPanel.View(), m.width, viewportHeight, overlayLayerZ)
	} else if m.palette.IsVisible() {
		layers = addOverlayLayer(layers, m.palette.View(), m.width, m.height, overlayLayerZ)
	} else if m.historyPicker != nil && m.historyPicker.IsVisible() {
		layers = addOverlayLayer(layers, m.historyPicker.View(), m.width, m.height, overlayLayerZ)
	}

	return lipgloss.NewCanvas(layers...)
}

func resolveGitBranchCmd(dir string, resolver func(string) string) tea.Cmd {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" || resolver == nil {
		return nil
	}
	return func() tea.Msg {
		return gitBranchMsg{
			dir:    trimmed,
			branch: resolver(trimmed),
		}
	}
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
		Width(w).Height(h).
		Z(z)
	return append(layers, layer)
}

// PTY message types

type ptyOutputMsg struct {
	data []byte
}

type ptyBatchFlushMsg struct{}

type streamThrottleFlushMsg struct{}

type ptyErrorMsg struct {
	err error
}

type streamStartOrigin int

const (
	streamOriginExplain streamStartOrigin = iota
	streamOriginChat
)

type streamStartResultMsg struct {
	origin streamStartOrigin
	stream <-chan commands.WtfStreamEvent
	err    error
	result *commands.Result
}

func (m *Model) buildExplainUserMessage(ctx *commands.Context) string {
	if ctx == nil {
		return "[Asked to explain output from terminal. Last command: N/A]"
	}
	lineCount := 0
	lines := ctx.GetLastNLines(ai.DefaultContextLines)
	if len(lines) > 0 {
		lineCount = len(lines)
	}

	command := "N/A"
	if ctx.Session != nil {
		last := ctx.Session.GetLastN(1)
		if len(last) > 0 && strings.TrimSpace(last[0].Command) != "" {
			command = strings.TrimSpace(last[0].Command)
		}
	}

	return fmt.Sprintf("[Asked to explain last %d lines from terminal. Last command: `%s`]", lineCount, command)
}

// listenToPTY creates a command that reads from PTY
func listenToPTY(ptyFile *os.File) tea.Cmd {
	return func() tea.Msg {
		buf := make([]byte, 4096)
		n, err := ptyFile.Read(buf)
		if err != nil {
			return ptyErrorMsg{err: err}
		}
		return ptyOutputMsg{data: buf[:n]}
	}
}

func listenToWtfStream(stream <-chan commands.WtfStreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream
		if !ok {
			return commands.WtfStreamEvent{Done: true}
		}
		return event
	}
}

func startExplainStreamCmd(ctx *commands.Context, handler commands.StreamingHandler, result *commands.Result) tea.Cmd {
	return func() tea.Msg {
		stream, err := handler.StartStream(ctx)
		return streamStartResultMsg{
			origin: streamOriginExplain,
			stream: stream,
			err:    err,
			result: result,
		}
	}
}

func startChatStreamCmd(ctx *commands.Context, messages []ai.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		chatHandler := &commands.ChatHandler{}
		stream, err := chatHandler.StartChatStream(ctx, messages)
		return streamStartResultMsg{
			origin: streamOriginChat,
			stream: stream,
			err:    err,
		}
	}
}

func (m *Model) startStreamPlaceholder() {
	if m.sidebar == nil {
		return
	}
	if m.streamPlaceholderActive {
		return
	}
	m.sidebar.SetStreaming(true)
	m.sidebar.StartAssistantMessageWithContent(streamThinkingPlaceholder)
	m.streamPlaceholderActive = true
	m.sidebar.RefreshView()
}

func (m *Model) replaceStreamPlaceholder(delta string) bool {
	if m.sidebar == nil {
		return false
	}
	if !m.streamPlaceholderActive {
		return false
	}
	m.sidebar.SetLastMessageContent(delta)
	m.streamPlaceholderActive = false
	return true
}

func (m *Model) clearStreamPlaceholder() {
	if m.sidebar == nil {
		return
	}
	if m.streamPlaceholderActive {
		m.sidebar.RemoveLastMessage()
		m.streamPlaceholderActive = false
	}
}

func (m *Model) appendNormalizedLines(data []byte) {
	if m.buffer == nil || len(data) == 0 || m.ptyNormalizer == nil {
		return
	}

	lines := m.ptyNormalizer.Append(data)
	for _, line := range lines {
		m.captureCommandFromLine(line)
		m.buffer.Write(line)
	}
}

func (m *Model) captureCommandFromLine(line []byte) {
	if m.session == nil || len(line) == 0 {
		return
	}

	cmd := capture.ExtractCommandFromPrompt(string(line))
	if cmd == "" {
		return
	}

	now := time.Now()
	last := m.session.GetLastN(1)
	if len(last) > 0 && last[0].Command == cmd {
		if now.Sub(last[0].StartTime) < 2*time.Second {
			return
		}
	}

	m.session.AddCommand(capture.CommandRecord{
		Command:    cmd,
		StartTime:  now,
		EndTime:    now,
		WorkingDir: m.currentDir,
	})
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

// Copilot auth status message type.
type copilotAuthStatusMsg struct {
	Status     ai.CopilotAuthStatus
	Err        error
	ShowPrompt bool
}

// fetchCopilotAuthStatusCmd queries the Copilot CLI auth status using the SDK.
func fetchCopilotAuthStatusCmd(showPrompt bool) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()

		slog.Info("copilot_auth_status_start")
		status, err := ai.FetchCopilotAuthStatus(ctx)
		if err != nil {
			slog.Error("copilot_auth_status_error", "error", err)
			return copilotAuthStatusMsg{Err: err, ShowPrompt: showPrompt}
		}

		slog.Info("copilot_auth_status_done", "authenticated", status.Authenticated)
		return copilotAuthStatusMsg{Status: status, ShowPrompt: showPrompt}
	}
}

func formatCopilotAuthStatus(status ai.CopilotAuthStatus, err error) (string, string, string) {
	summary := "❌ Not connected"
	detail := "❌ Not connected (Enter for details)"
	statusLabel := "Not connected"
	if err != nil {
		message := fmt.Sprintf("Status: %s\nError: %v", statusLabel, err)
		return summary, detail, message
	}

	if status.Authenticated {
		summary = "✅ Connected"
		detail = "✅ Connected (Enter for details)"
		statusLabel = "Connected"
	}

	lines := []string{fmt.Sprintf("Status: %s", statusLabel)}
	if status.Login != "" {
		lines = append(lines, fmt.Sprintf("User: %s", status.Login))
	}
	if status.AuthType != "" {
		lines = append(lines, fmt.Sprintf("Auth: %s", status.AuthType))
	}
	if status.Host != "" {
		lines = append(lines, fmt.Sprintf("Host: %s", status.Host))
	}
	if status.StatusMessage != "" {
		lines = append(lines, status.StatusMessage)
	}

	return summary, detail, strings.Join(lines, "\n")
}
