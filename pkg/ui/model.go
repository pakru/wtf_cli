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

// Model represents the Bubble Tea application state
type Model struct {
	// PTY connection
	ptyFile *os.File
	cwdFunc func() (string, error) // Function to get shell's cwd

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

	// Streaming state
	wtfStream  <-chan commands.WtfStreamEvent
	wtfContent string

	// UI state
	width  int
	height int
	ready  bool

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
		ptyFile:             ptyFile,
		cwdFunc:             cwdFunc,
		viewport:            viewport,
		statusBar:           statusBar,
		inputHandler:        input.NewInputHandler(ptyFile),
		palette:             palette.NewCommandPalette(),
		historyPicker:       historypicker.NewHistoryPickerPanel(),
		resultPanel:         result.NewResultPanel(),
		settingsPanel:       settings.NewSettingsPanel(),
		modelPicker:         picker.NewModelPickerPanel(),
		optionPicker:        picker.NewOptionPickerPanel(),
		sidebar:             sidebar.NewSidebar(),
		dispatcher:          commands.NewDispatcher(),
		buffer:              buf,
		session:             sess,
		currentDir:          initialDir,
		fullScreenPanel:     fullscreen.NewFullScreenPanel(80, 24),
		altScreenState:      terminal.NewAltScreenState(),
		ptyNormalizer:       terminal.NewNormalizer(),
		ptyBatchMaxSize:     16384,                 // 16KB
		ptyBatchMaxWait:     16 * time.Millisecond, // ~60fps
		streamThrottleDelay: 50 * time.Millisecond, // Throttle stream updates
	}
}

// Init initializes the model (Bubble Tea lifecycle method)
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenToPTY(m.ptyFile), // Start listening to PTY output
		tickDirectory(),        // Start directory update ticker
	)
}

// tickDirectory creates a command that periodically updates directory
func tickDirectory() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return directoryUpdateMsg{}
	})
}

type directoryUpdateMsg struct{}

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

	case tea.KeyPressMsg:
		if m.fullScreenMode {
			handled, cmd := m.inputHandler.HandleKey(msg)
			if handled {
				return m, cmd
			}
			return m, nil
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

		// If settings panel is visible, handle its keys first
		if m.settingsPanel.IsVisible() {
			cmd := m.settingsPanel.Update(msg)
			return m, cmd
		}

		// If result panel is visible, handle its keys first
		if m.resultPanel.IsVisible() {
			cmd := m.resultPanel.Update(msg)
			return m, cmd
		}

		// If palette is visible, handle its keys first
		if m.palette.IsVisible() {
			cmd := m.palette.Update(msg)
			return m, cmd
		}

		// If history picker is visible, handle its keys first
		if m.historyPicker != nil && m.historyPicker.IsVisible() {
			cmd := m.historyPicker.Update(msg)
			return m, cmd
		}

		if m.sidebar != nil && m.sidebar.IsVisible() && m.sidebar.ShouldHandleKey(msg) {
			wasVisible := m.sidebar.IsVisible()
			cmd := m.sidebar.Update(msg)
			if wasVisible && !m.sidebar.IsVisible() {
				slog.Info("sidebar_close", "reason", "key")
				m.applyLayout()
			}
			return m, cmd
		}

		// Use input handler to route keys to PTY
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
			return m, nil
		case commands.ResultActionOpenHistoryPicker:
			slog.Info("history_picker_from_command")
			// Emit ShowHistoryPickerMsg with empty initial filter
			return m, func() tea.Msg {
				return input.ShowHistoryPickerMsg{InitialFilter: ""}
			}
		}

		if streamHandler, ok := handler.(commands.StreamingHandler); ok {
			stream, err := streamHandler.StartStream(ctx)
			if err != nil {
				if m.sidebar != nil {
					slog.Error("sidebar_open_error", "error", err)
					m.sidebar.Show(result.Title, fmt.Sprintf("Error: %v", err))
					m.applyLayout()
				}
				return m, nil
			}
			if stream == nil {
				m.resultPanel.Show(result.Title, result.Content)
				m.wtfContent = ""
				return m, nil
			}
			if m.sidebar != nil {
				m.sidebar.Show(result.Title, result.Content)
				slog.Info("sidebar_open", "title", result.Title, "streaming", true)
				m.applyLayout()
			}
			m.wtfContent = ""
			m.wtfStream = stream
			return m, listenToWtfStream(stream)
		}

		// Show result in panel
		m.resultPanel.Show(result.Title, result.Content)
		m.wtfContent = ""

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

	case input.CommandSubmittedMsg:
		if m.session == nil {
			return m, nil
		}
		if strings.TrimSpace(msg.Command) == "" {
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

	case settings.SettingsSaveMsg:
		// Save settings to file
		if err := config.Save(msg.ConfigPath, msg.Config); err != nil {
			slog.Error("settings_save_error", "error", err)
		} else {
			slog.Info("settings_save",
				"model", msg.Config.OpenRouter.Model,
				"log_level", msg.Config.LogLevel,
				"log_format", msg.Config.LogFormat,
				"log_file", msg.Config.LogFile,
			)
			logging.SetLevel(msg.Config.LogLevel)
		}
		m.statusBar.SetModel(msg.Config.OpenRouter.Model)
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
		slog.Info("model_picker_open", "current", msg.Current, "cached_models", len(msg.Options))
		if m.modelPicker != nil {
			m.modelPicker.SetSize(m.width, m.height)
			m.modelPicker.Show(msg.Options, msg.Current)
		}
		cmd := refreshModelCacheCmd(msg.APIURL)
		return m, cmd

	case picker.ModelPickerSelectMsg:
		slog.Info("model_picker_select", "model", msg.ModelID)
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.Hide()
		}
		if m.settingsPanel != nil {
			m.settingsPanel.SetModelValue(msg.ModelID)
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

	case result.ResultPanelCloseMsg:
		// Result panel closed
		return m, nil

	case commands.WtfStreamEvent:
		if msg.Err != nil {
			slog.Error("wtf_stream_error", "error", msg.Err)
			m.wtfContent = fmt.Sprintf("Error: %v", msg.Err)
			if m.sidebar != nil && m.sidebar.IsVisible() {
				m.sidebar.SetContent(m.wtfContent)
			}
			m.wtfStream = nil
			m.streamThrottlePending = false
			return m, nil
		}
		if msg.Delta != "" {
			// Accumulate content
			if m.wtfContent == "" {
				m.wtfContent = msg.Delta
			} else {
				m.wtfContent += msg.Delta
			}

			// Throttle sidebar updates
			if !m.streamThrottlePending {
				m.streamThrottlePending = true
				// Immediately update on first chunk
				if m.sidebar != nil && m.sidebar.IsVisible() {
					m.sidebar.SetContent(m.wtfContent)
				}
				return m, tea.Batch(
					tea.Tick(m.streamThrottleDelay, func(time.Time) tea.Msg {
						return streamThrottleFlushMsg{}
					}),
					listenToWtfStream(m.wtfStream),
				)
			}
			// Subsequent chunks: just listen, don't update sidebar yet
			return m, listenToWtfStream(m.wtfStream)
		}
		if msg.Done {
			logger := slog.Default()
			if logger.Enabled(context.Background(), logging.LevelTrace) {
				logger.Log(
					context.Background(),
					logging.LevelTrace,
					"wtf_stream_full_response",
					"response", m.wtfContent,
				)
			}
			slog.Info("wtf_stream_done")
			m.wtfStream = nil
			// Final update to sidebar with all content
			if m.sidebar != nil && m.sidebar.IsVisible() {
				m.sidebar.SetContent(m.wtfContent)
			}
			m.streamThrottlePending = false
			return m, nil
		}
		if m.wtfStream != nil {
			return m, listenToWtfStream(m.wtfStream)
		}
		return m, nil

	case streamThrottleFlushMsg:
		m.streamThrottlePending = false
		// Update sidebar with accumulated content
		if m.sidebar != nil && m.sidebar.IsVisible() {
			m.sidebar.SetContent(m.wtfContent)
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
		// Update current directory from /proc/<pid>/cwd
		if m.cwdFunc != nil {
			if cwd, err := m.cwdFunc(); err == nil {
				m.currentDir = cwd
			}
		}
		// Schedule next update
		return m, tickDirectory()
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
	modelName := config.Default().OpenRouter.Model
	path := config.GetConfigPath()
	if path == "" {
		return modelName
	}
	if _, err := os.Stat(path); err != nil {
		return modelName
	}
	cfg, err := config.Load(path)
	if err != nil {
		return modelName
	}
	if strings.TrimSpace(cfg.OpenRouter.Model) == "" {
		return modelName
	}
	return cfg.OpenRouter.Model
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
