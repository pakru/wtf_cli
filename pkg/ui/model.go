package ui

import (
	"os"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/pty"
	"wtf_cli/pkg/ui/components/continueprompt"
	"wtf_cli/pkg/ui/components/fullscreen"
	"wtf_cli/pkg/ui/components/historypicker"
	"wtf_cli/pkg/ui/components/palette"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/components/result"
	"wtf_cli/pkg/ui/components/settings"
	"wtf_cli/pkg/ui/components/sidebar"
	"wtf_cli/pkg/ui/components/statusbar"
	"wtf_cli/pkg/ui/components/toolapproval"
	"wtf_cli/pkg/ui/components/viewport"
	"wtf_cli/pkg/ui/components/welcome"
	"wtf_cli/pkg/ui/input"
	"wtf_cli/pkg/ui/terminal"

	tea "charm.land/bubbletea/v2"
)

const (
	streamThinkingPlaceholder = "Thinking..."
	selectedTextCopiedMessage = "Selected text copied to clipboard"
)

// Model represents the Bubble Tea application state
type Model struct {
	// PTY connection
	ptyFile *os.File
	cwdFunc func() (string, error) // Function to get shell's cwd
	// secretDetector checks whether the PTY is in canonical secret-input mode.
	// Injectable for tests.
	secretDetector func(*os.File) bool

	// UI Components
	viewport       viewport.PTYViewport              // Viewport for PTY output
	statusBar      *statusbar.StatusBarView          // Status bar at bottom
	inputHandler   *input.InputHandler               // Input routing to PTY
	palette        *palette.CommandPalette           // Command palette overlay
	historyPicker  *historypicker.HistoryPickerPanel // History search picker
	resultPanel    *result.ResultPanel               // Result panel overlay
	settingsPanel  *settings.SettingsPanel           // Settings panel overlay
	modelPicker    *picker.ModelPickerPanel
	optionPicker   *picker.OptionPickerPanel
	sidebar        *sidebar.Sidebar // Sidebar for AI suggestions
	toolApproval   *toolapproval.Panel
	continuePrompt *continueprompt.Panel

	// Command system
	dispatcher *commands.Dispatcher

	// sessionApprovals holds per-tool "allow always this session" decisions.
	// Lives for the wtf_cli process lifetime so that approving a tool once
	// "always" persists across multiple /explain or /chat invocations.
	sessionApprovals *commands.SessionApprovals

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
	toolCallNewTurnNeeded   bool // true after a tool call finishes; next delta starts a new assistant message

	// UI state
	width           int
	height          int
	ready           bool
	terminalFocused bool
	scrollMode      bool // True when user is browsing scrollback (auto-scroll paused)

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

	startupPTYOutputSeen bool
	startupUpdateShown   bool
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
	provider, model := loadProviderAndModelFromConfig()

	m := Model{
		ptyFile:          ptyFile,
		cwdFunc:          cwdFunc,
		secretDetector:   pty.IsSecretInputMode,
		viewport:         viewport,
		statusBar:        statusBar,
		inputHandler:     input.NewInputHandler(ptyFile),
		palette:          palette.NewCommandPalette(),
		historyPicker:    historypicker.NewHistoryPickerPanel(),
		resultPanel:      result.NewResultPanel(),
		settingsPanel:    settings.NewSettingsPanel(),
		modelPicker:      picker.NewModelPickerPanel(),
		optionPicker:     picker.NewOptionPickerPanel(),
		sidebar:          sidebar.NewSidebar(),
		toolApproval:     toolapproval.NewPanel(),
		continuePrompt:   continueprompt.NewPanel(),
		dispatcher:       commands.NewDispatcher(),
		sessionApprovals: commands.NewSessionApprovals(),
		buffer:           buf,
		session:          sess,
		currentDir:       initialDir,

		gitBranchResolver:   statusbar.ResolveGitBranch,
		fullScreenPanel:     fullscreen.NewFullScreenPanel(80, 24),
		altScreenState:      terminal.NewAltScreenState(),
		ptyNormalizer:       terminal.NewNormalizer(),
		ptyBatchMaxSize:     16384,                 // 16KB
		ptyBatchMaxWait:     16 * time.Millisecond, // ~60fps
		streamThrottleDelay: 50 * time.Millisecond, // Throttle stream updates
		terminalFocused:     true,
	}
	m.sidebar.SetActiveLLM(provider, model)
	m.installAgentFactories()
	return m
}

// chatHandler returns the dispatcher's /chat handler so the call inherits the
// installed ApproverFactory. Falls back to a fresh handler (auto-allow
// approver) if the dispatcher disagrees about the type.
func (m *Model) chatHandler() *commands.ChatHandler {
	if h, ok := m.dispatcher.GetHandler("/chat"); ok {
		if ch, ok := h.(*commands.ChatHandler); ok {
			return ch
		}
	}
	return &commands.ChatHandler{}
}

// installAgentFactories wires the dispatcher's /explain and /chat handlers so
// each invocation produces a UIApprover (tool-call approval popup) and a
// UIContinuer ("continue the tool-call loop?" popup) bound to that call's event
// channel, plus the shared session-approvals store.
//
// Handlers fall back to AutoAllowApprover / AutoStopContinuer when no factory
// is set, which is what tests and headless runs rely on. Doing this once at
// construction time (rather than at each call) avoids reaching into the
// dispatcher mid-flight.
func (m *Model) installAgentFactories() {
	approverFactory := func(out chan<- commands.WtfStreamEvent) commands.Approver {
		return commands.NewUIApprover(out, m.sessionApprovals)
	}
	continuerFactory := func(out chan<- commands.WtfStreamEvent) commands.Continuer {
		return commands.NewUIContinuer(out)
	}
	if h, ok := m.dispatcher.GetHandler("/explain"); ok {
		if eh, ok := h.(*commands.ExplainHandler); ok {
			eh.ApproverFactory = approverFactory
			eh.ContinuerFactory = continuerFactory
		}
	}
	if h, ok := m.dispatcher.GetHandler("/chat"); ok {
		if ch, ok := h.(*commands.ChatHandler); ok {
			ch.ApproverFactory = approverFactory
			ch.ContinuerFactory = continuerFactory
		}
	}
}

// Init initializes the model (Bubble Tea lifecycle method)
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenToPTY(m.ptyFile), // Start listening to PTY output
		tickDirectory(),        // Start directory update ticker
		resolveGitBranchCmd(m.currentDir, m.gitBranchResolver),
		fetchUpdateCheckCmd(),
	)
}

// Update handles messages and updates model state (Bubble Tea lifecycle method)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		return m.handleWindowSize(msg)

	case resizeApplyMsg:
		return m.handleResizeApply(msg)

	case tea.MouseWheelMsg:
		return m.handleMouseWheel(msg)

	case tea.MouseClickMsg:
		return m.handleMouseClick(msg)

	case tea.MouseMotionMsg:
		return m.handleMouseMotion(msg)

	case tea.MouseReleaseMsg:
		return m.handleMouseRelease(msg)

	case tea.PasteMsg:
		return m.handlePaste(msg)

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case input.ShowPaletteMsg:
		return m.handleShowPalette()

	case input.ToggleChatMsg:
		return m.handleToggleChat()

	case input.FocusSwitchMsg:
		return m.handleFocusSwitch()

	case palette.PaletteSelectMsg:
		return m.handlePaletteSelect(msg)

	case palette.PaletteCancelMsg:
		return m.handlePaletteCancel()

	case input.ShowHistoryPickerMsg:
		return m.handleShowHistoryPicker(msg)

	case historypicker.HistoryPickerSelectMsg:
		return m.handleHistoryPickerSelect(msg)

	case historypicker.HistoryPickerCancelMsg:
		return m.handleHistoryPickerCancel()

	case sidebar.CommandExecuteMsg:
		return m.handleSidebarCommandExecute(msg)

	case input.CommandSubmittedMsg:
		return m.handleCommandSubmitted(msg)

	case settings.SettingsCloseMsg:
		return m.handleSettingsClose()

	case settings.StartCopilotAuthMsg:
		return m.handleStartCopilotAuth()

	case copilotAuthStatusMsg:
		return m.handleCopilotAuthStatus(msg)

	case settings.SettingsSaveMsg:
		return m.handleSettingsSave(msg)

	case input.CtrlDPressedMsg:
		return m.handleCtrlDPressed()

	case exitConfirmTimeoutMsg:
		return m.handleExitConfirmTimeout(msg)

	case clearStatusMsgMsg:
		return m.handleClearStatusMsg()

	case picker.OpenModelPickerMsg:
		return m.handleOpenModelPicker(msg)

	case picker.ModelPickerSelectMsg:
		return m.handleModelPickerSelect(msg)

	case picker.OpenOptionPickerMsg:
		return m.handleOpenOptionPicker(msg)

	case picker.OptionPickerSelectMsg:
		return m.handleOptionPickerSelect(msg)

	case picker.ModelPickerRefreshMsg:
		return m.handleModelPickerRefresh(msg)

	case providerModelsRefreshMsg:
		return m.handleProviderModelsRefresh(msg)

	case streamStartResultMsg:
		return m.handleStreamStartResult(msg)

	case result.ResultPanelCloseMsg:
		// Result panel closed
		return m, nil

	case toolapproval.DecisionMsg:
		return m.handleToolApprovalDecision(msg)

	case continueprompt.DecisionMsg:
		return m.handleContinuePromptDecision(msg)

	case sidebar.ChatSubmitMsg:
		return m.handleChatSubmit(msg)

	case commands.WtfStreamEvent:
		return m.handleWtfStreamEvent(msg)

	case streamThrottleFlushMsg:
		return m.handleStreamThrottleFlush()

	case updateCheckMsg:
		return m.handleUpdateCheck(msg)

	case ptyOutputMsg:
		return m.handlePTYOutput(msg)

	case ptyBatchFlushMsg:
		return m.handlePTYBatchFlush()

	case ptyErrorMsg:
		return m.handlePTYError(msg)

	case directoryUpdateMsg:
		return m.handleDirectoryUpdate()

	case gitBranchMsg:
		return m.handleGitBranch(msg)
	}

	return m, nil
}
