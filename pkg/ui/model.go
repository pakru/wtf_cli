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

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/cellbuf"
)

// Model represents the Bubble Tea application state
type Model struct {
	// PTY connection
	ptyFile *os.File
	cwdFunc func() (string, error) // Function to get shell's cwd

	// UI Components
	viewport      PTYViewport     // Viewport for PTY output
	statusBar     *StatusBarView  // Status bar at bottom
	inputHandler  *InputHandler   // Input routing to PTY
	palette       *CommandPalette // Command palette overlay
	resultPanel   *ResultPanel    // Result panel overlay
	settingsPanel *SettingsPanel  // Settings panel overlay
	modelPicker   *ModelPickerPanel
	optionPicker  *OptionPickerPanel
	sidebar       *Sidebar // AI response sidebar

	// Command system
	dispatcher *commands.Dispatcher

	// Data
	buffer     *buffer.CircularBuffer
	session    *capture.SessionContext
	currentDir string

	// Streaming state
	wtfStream  <-chan commands.WtfStreamEvent
	wtfContent string
	wtfTitle   string

	// UI state
	width  int
	height int
	ready  bool

	exitPending   bool
	exitConfirmID int

	resizeDebounceID int       // Counter to debounce resize events
	resizeTime       time.Time // When last PTY resize occurred (to suppress prompt reprint)
	initialResize    bool      // Track if we've done the initial resize

	ptyLineBuffer []byte
	ptyPendingCR  bool

	// Full-screen app support (vim, nano, htop)
	fullScreenMode  bool
	fullScreenPanel *FullScreenPanel
	altScreenState  *AltScreenState
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
	viewport := NewPTYViewport()
	viewport.AppendOutput([]byte(WelcomeMessage()))

	statusBar := NewStatusBarView()
	statusBar.SetModel(loadModelFromConfig())

	return Model{
		ptyFile:         ptyFile,
		cwdFunc:         cwdFunc,
		viewport:        viewport,
		statusBar:       statusBar,
		inputHandler:    NewInputHandler(ptyFile),
		palette:         NewCommandPalette(),
		resultPanel:     NewResultPanel(),
		settingsPanel:   NewSettingsPanel(),
		modelPicker:     NewModelPickerPanel(),
		optionPicker:    NewOptionPickerPanel(),
		sidebar:         NewSidebar(),
		dispatcher:      commands.NewDispatcher(),
		buffer:          buf,
		session:         sess,
		currentDir:      initialDir,
		fullScreenPanel: NewFullScreenPanel(80, 24),
		altScreenState:  NewAltScreenState(),
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
		m.resultPanel.SetSize(m.width, m.height)
		m.settingsPanel.SetSize(m.width, m.height)
		if m.modelPicker != nil {
			m.modelPicker.SetSize(m.width, m.height)
		}
		if m.optionPicker != nil {
			m.optionPicker.SetSize(m.width, m.height)
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
				contentWidth, contentHeight := contentSize(m.width, m.height)
				ResizePTY(m.ptyFile, contentWidth, contentHeight)
			} else {
				viewportHeight := m.height - 1
				viewportWidth := m.width
				if m.sidebar != nil && m.sidebar.IsVisible() {
					left, _ := splitSidebarWidths(m.width)
					viewportWidth = left
				}
				ResizePTY(m.ptyFile, viewportWidth, viewportHeight)
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

	case showPaletteMsg:
		// Show the command palette
		slog.Info("palette_open")
		m.palette.Show()
		m.inputHandler.SetPaletteMode(true)
		return m, nil

	case paletteSelectMsg:
		// Command selected from palette
		slog.Info("palette_select", "command", msg.command)
		m.inputHandler.SetPaletteMode(false)

		// Execute the command
		ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
		handler, ok := m.dispatcher.GetHandler(msg.command)
		if !ok {
			m.resultPanel.Show("Error", "Unknown command: "+msg.command)
			return m, nil
		}
		result := handler.Execute(ctx)

		// Special case: /settings opens settings panel
		if result.Title == "__OPEN_SETTINGS__" {
			slog.Info("settings_open")
			cfg, _ := config.Load(config.GetConfigPath())
			m.settingsPanel.SetSize(m.width, m.height)
			m.settingsPanel.Show(cfg, config.GetConfigPath())
			m.statusBar.SetModel(cfg.OpenRouter.Model)
			return m, nil
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
				m.wtfTitle = result.Title
				m.wtfContent = ""
				return m, nil
			}
			if m.sidebar != nil {
				m.sidebar.Show(result.Title, result.Content)
				slog.Info("sidebar_open", "title", result.Title, "streaming", true)
				m.applyLayout()
			}
			m.wtfTitle = result.Title
			m.wtfContent = ""
			m.wtfStream = stream
			return m, listenToWtfStream(stream)
		}

		// Show result in panel
		m.resultPanel.Show(result.Title, result.Content)
		m.wtfTitle = result.Title
		m.wtfContent = ""

		return m, nil

	case paletteCancelMsg:
		// Palette cancelled
		slog.Info("palette_cancel")
		m.inputHandler.SetPaletteMode(false)
		return m, nil

	case settingsCloseMsg:
		// Settings panel closed
		slog.Info("settings_close")
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.Hide()
		}
		if m.optionPicker != nil && m.optionPicker.IsVisible() {
			m.optionPicker.Hide()
		}
		return m, nil

	case settingsSaveMsg:
		// Save settings to file
		if err := config.Save(msg.configPath, msg.config); err != nil {
			slog.Error("settings_save_error", "error", err)
		} else {
			slog.Info("settings_save",
				"model", msg.config.OpenRouter.Model,
				"log_level", msg.config.LogLevel,
				"log_format", msg.config.LogFormat,
				"log_file", msg.config.LogFile,
			)
		}
		m.statusBar.SetModel(msg.config.OpenRouter.Model)
		return m, nil

	case ctrlDPressedMsg:
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

	case openModelPickerMsg:
		slog.Info("model_picker_open", "current", msg.current, "cached_models", len(msg.options))
		if m.modelPicker != nil {
			m.modelPicker.SetSize(m.width, m.height)
			m.modelPicker.Show(msg.options, msg.current)
		}
		cmd := refreshModelCacheCmd(msg.apiURL)
		return m, cmd

	case modelPickerSelectMsg:
		slog.Info("model_picker_select", "model", msg.modelID)
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.Hide()
		}
		if m.settingsPanel != nil {
			m.settingsPanel.SetModelValue(msg.modelID)
		}
		return m, nil

	case openOptionPickerMsg:
		slog.Info("option_picker_open", "field", msg.fieldKey, "current", msg.current)
		if m.optionPicker != nil {
			m.optionPicker.SetSize(m.width, m.height)
			m.optionPicker.Show(msg.title, msg.fieldKey, msg.options, msg.current)
		}
		return m, nil

	case optionPickerSelectMsg:
		slog.Info("option_picker_select", "field", msg.fieldKey, "value", msg.value)
		if m.optionPicker != nil && m.optionPicker.IsVisible() {
			m.optionPicker.Hide()
		}
		if m.settingsPanel != nil {
			switch msg.fieldKey {
			case "log_level":
				m.settingsPanel.SetLogLevelValue(msg.value)
			case "log_format":
				m.settingsPanel.SetLogFormatValue(msg.value)
			}
		}
		return m, nil

	case modelPickerRefreshMsg:
		if msg.err != nil {
			slog.Error("model_picker_refresh_error", "error", msg.err)
			return m, nil
		}
		if m.modelPicker != nil && m.modelPicker.IsVisible() {
			m.modelPicker.UpdateOptions(msg.cache.Models)
		}
		if m.settingsPanel != nil {
			m.settingsPanel.SetModelCache(msg.cache)
		}
		slog.Info("model_picker_refresh_done", "models", len(msg.cache.Models))
		return m, nil

	case resultPanelCloseMsg:
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
			return m, nil
		}
		if msg.Delta != "" {
			if m.wtfContent == "" {
				m.wtfContent = msg.Delta
			} else {
				m.wtfContent += msg.Delta
			}
			if m.sidebar != nil && m.sidebar.IsVisible() {
				m.sidebar.SetContent(m.wtfContent)
			}
		}
		if msg.Done {
			slog.Info("wtf_stream_done")
			m.wtfStream = nil
			return m, nil
		}
		if m.wtfStream != nil {
			return m, listenToWtfStream(m.wtfStream)
		}
		return m, nil

	case ptyOutputMsg:
		// Suppress PTY output briefly after resize to prevent prompt reprint from showing
		if !m.resizeTime.IsZero() && time.Since(m.resizeTime) < 100*time.Millisecond {
			// Skip appending to viewport but still schedule next read
			return m, listenToPTY(m.ptyFile)
		}

		chunks := m.altScreenState.SplitTransitions(msg.data)
		for i, chunk := range chunks {
			if m.inputHandler != nil {
				m.inputHandler.UpdateTerminalModes(chunk.data)
			}

			if chunk.entering {
				if !m.fullScreenMode {
					m.enterFullScreen(len(chunk.data))
				}
				if m.fullScreenPanel != nil && len(chunk.data) > 0 {
					m.fullScreenPanel.Write(chunk.data)
				}
				continue
			}

			if chunk.exiting {
				if m.fullScreenMode && hasFutureEnter(chunks[i+1:]) {
					if m.fullScreenPanel != nil && len(chunk.data) > 0 {
						m.fullScreenPanel.Write(chunk.data)
					}
					continue
				}

				if m.fullScreenMode && m.fullScreenPanel != nil && len(chunk.data) > 0 {
					m.fullScreenPanel.Write(chunk.data)
				}
				if m.fullScreenMode {
					m.exitFullScreen()
				} else if len(chunk.data) > 0 {
					m.appendPTYOutput(chunk.data)
					m.viewport.AppendOutput(chunk.data)
				}
				continue
			}

			if len(chunk.data) == 0 {
				continue
			}

			if m.fullScreenMode {
				// Full-screen mode: send to panel, NOT to buffer (buffer isolation)
				if m.fullScreenPanel != nil {
					m.fullScreenPanel.Write(chunk.data)
				}
			} else {
				// Normal mode: append to viewport AND buffer
				m.appendPTYOutput(chunk.data)
				m.viewport.AppendOutput(chunk.data)
			}
		}

		// Schedule next read
		return m, listenToPTY(m.ptyFile)

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

	// Update status bar width and directory
	m.statusBar.SetWidth(m.width)
	m.statusBar.SetDirectory(m.currentDir)

	// Base view: viewport + status bar
	topView := m.viewport.View()
	if m.sidebar != nil && m.sidebar.IsVisible() {
		topView = lipgloss.JoinHorizontal(lipgloss.Top, topView, m.sidebar.View())
	}

	baseView := lipgloss.JoinVertical(lipgloss.Left, topView, m.statusBar.Render())

	overlayView := baseView
	if m.settingsPanel.IsVisible() {
		overlayView = m.overlayCenter(overlayView, m.settingsPanel.View())
	}

	if m.optionPicker != nil && m.optionPicker.IsVisible() {
		v.SetContent(m.overlayCenter(overlayView, m.optionPicker.View()))
		return v
	}

	if m.modelPicker != nil && m.modelPicker.IsVisible() {
		v.SetContent(m.overlayCenter(overlayView, m.modelPicker.View()))
		return v
	}

	// Overlay result panel if visible
	if m.resultPanel.IsVisible() {
		v.SetContent(m.overlayCenter(overlayView, m.resultPanel.View()))
		return v
	}

	// Overlay command palette if visible
	if m.palette.IsVisible() {
		v.SetContent(m.overlayCenter(overlayView, m.palette.View()))
		return v
	}

	v.SetContent(overlayView)
	return v
}

// overlayCenter places a panel centered vertically over the base view
func (m Model) overlayCenter(base, panel string) string {
	baseLines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	// Calculate vertical position (center)
	panelHeight := len(panelLines)
	if panelHeight > m.height {
		panelLines = panelLines[:m.height]
		panelHeight = len(panelLines)
	}
	startRow := (m.height - panelHeight) / 2
	if startRow < 0 {
		startRow = 0
	}

	panelWidth := 0
	for _, line := range panelLines {
		width := ansi.StringWidth(line)
		if width > panelWidth {
			panelWidth = width
		}
	}
	if panelWidth > m.width {
		panelWidth = m.width
	}
	startCol := (m.width - panelWidth) / 2
	if startCol < 0 {
		startCol = 0
	}

	// Build result with overlay to preserve background text outside the panel.
	result := make([]string, m.height)
	for i := 0; i < m.height; i++ {
		if i < len(baseLines) {
			result[i] = baseLines[i]
		} else {
			result[i] = ""
		}
	}

	// Overlay panel lines while keeping the base view visible outside the panel area.
	for i, panelLine := range panelLines {
		row := startRow + i
		if row >= 0 && row < m.height {
			result[row] = overlayLine(result[row], panelLine, startCol, panelWidth, m.width)
		}
	}

	return strings.Join(result, "\n")
}

// Helper functions

func hasFutureEnter(chunks []altScreenChunk) bool {
	for _, chunk := range chunks {
		if chunk.entering {
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
			contentWidth, contentHeight := contentSize(m.width, m.height)
			ResizePTY(m.ptyFile, contentWidth, contentHeight)
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
	m.resultPanel.SetSize(m.width, m.height)
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
		return modelPickerRefreshMsg{cache: cache, err: err}
	}
}

func overlayLine(baseLine, panelLine string, startCol, panelWidth, totalWidth int) string {
	if totalWidth <= 0 {
		return ""
	}
	if startCol < 0 {
		startCol = 0
	}
	if startCol > totalWidth {
		startCol = totalWidth
	}
	if panelWidth < 0 {
		panelWidth = 0
	}
	if startCol+panelWidth > totalWidth {
		panelWidth = totalWidth - startCol
	}

	baseBuf := cellbuf.NewBuffer(totalWidth, 1)
	cellbuf.SetContent(baseBuf, baseLine)

	if panelWidth > 0 && panelLine != "" {
		panelBuf := cellbuf.NewBuffer(panelWidth, 1)
		cellbuf.SetContent(panelBuf, panelLine)

		for x := 0; x < panelWidth; x++ {
			panelCell := panelBuf.Cell(x, 0)
			if panelCell == nil || panelCell.Width == 0 {
				continue
			}
			baseBuf.SetCell(startCol+x, 0, panelCell)
		}
	}

	_, line := cellbuf.RenderLine(baseBuf, 0)
	lineWidth := ansi.StringWidth(line)
	if lineWidth < totalWidth {
		line += ansi.ResetStyle + strings.Repeat(" ", totalWidth-lineWidth)
	}
	return line
}

// PTY message types

type ptyOutputMsg struct {
	data []byte
}

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

func (m *Model) appendPTYOutput(data []byte) {
	if m.buffer == nil || len(data) == 0 {
		return
	}

	for _, b := range data {
		if m.ptyPendingCR {
			if b == '\n' {
				if len(m.ptyLineBuffer) > 0 {
					m.buffer.Write(m.ptyLineBuffer)
					m.ptyLineBuffer = m.ptyLineBuffer[:0]
				}
				m.ptyPendingCR = false
				continue
			}
			if b == '\r' {
				continue
			}
			m.ptyLineBuffer = m.ptyLineBuffer[:0]
			m.ptyPendingCR = false
		}

		switch b {
		case '\r':
			m.ptyPendingCR = true
		case '\n':
			if len(m.ptyLineBuffer) > 0 {
				m.buffer.Write(m.ptyLineBuffer)
				m.ptyLineBuffer = m.ptyLineBuffer[:0]
			}
		case '\t':
			m.ptyLineBuffer = append(m.ptyLineBuffer, tabSpaces...)
		default:
			m.ptyLineBuffer = append(m.ptyLineBuffer, b)
		}
	}
}
