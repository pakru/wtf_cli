package ui

import (
	"os"
	"strings"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the Bubble Tea application state
type Model struct {
	// PTY connection
	ptyFile *os.File
	cwdFunc func() (string, error) // Function to get shell's cwd

	// UI Components
	viewport     PTYViewport     // Viewport for PTY output
	statusBar    *StatusBarView  // Status bar at bottom
	inputHandler *InputHandler   // Input routing to PTY
	palette      *CommandPalette // Command palette overlay
	resultPanel  *ResultPanel    // Result panel overlay

	// Command system
	dispatcher *commands.Dispatcher

	// Data
	buffer     *buffer.CircularBuffer
	session    *capture.SessionContext
	currentDir string

	// UI state
	width  int
	height int
	ready  bool
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

	return Model{
		ptyFile:      ptyFile,
		cwdFunc:      cwdFunc,
		viewport:     viewport,
		statusBar:    NewStatusBarView(),
		inputHandler: NewInputHandler(ptyFile),
		palette:      NewCommandPalette(),
		resultPanel:  NewResultPanel(),
		dispatcher:   commands.NewDispatcher(),
		buffer:       buf,
		session:      sess,
		currentDir:   initialDir,
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

// Update handles messages and updates model state (Bubble Tea lifecycle method)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update viewport size (leave room for status bar = 1 line)
		m.viewport.SetSize(msg.Width, msg.Height-1)

		// Update palette and result panel sizes
		m.palette.SetSize(msg.Width, msg.Height)
		m.resultPanel.SetSize(msg.Width, msg.Height)

		// Synchronize PTY size with terminal size
		if m.ptyFile != nil {
			ResizePTY(m.ptyFile, msg.Width, msg.Height-1)
		}

		return m, nil

	case tea.KeyMsg:
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

		// Use input handler to route keys to PTY
		handled, cmd := m.inputHandler.HandleKey(msg)
		if handled {
			return m, cmd
		}

		// If not handled by input handler, ignore
		return m, nil

	case showPaletteMsg:
		// Show the command palette
		m.palette.Show()
		m.inputHandler.SetPaletteMode(true)
		return m, nil

	case paletteSelectMsg:
		// Command selected from palette
		m.inputHandler.SetPaletteMode(false)

		// Execute the command
		ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
		result := m.dispatcher.Dispatch(msg.command, ctx)

		// Show result in panel
		m.resultPanel.Show(result.Title, result.Content)
		return m, nil

	case paletteCancelMsg:
		// Palette cancelled
		m.inputHandler.SetPaletteMode(false)
		return m, nil

	case resultPanelCloseMsg:
		// Result panel closed
		return m, nil

	case ptyOutputMsg:
		// PTY sent output - append to viewport
		m.viewport.AppendOutput(msg.data)

		// Schedule next read
		return m, listenToPTY(m.ptyFile)

	case ptyErrorMsg:
		// PTY error - probably shell exited
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
func (m Model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Update status bar width and directory
	m.statusBar.SetWidth(m.width)
	m.statusBar.SetDirectory(m.currentDir)

	// Base view: viewport + status bar
	baseView := lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		m.statusBar.Render(),
	)

	// Overlay result panel if visible
	if m.resultPanel.IsVisible() {
		return m.overlayCenter(baseView, m.resultPanel.View())
	}

	// Overlay command palette if visible
	if m.palette.IsVisible() {
		return m.overlayCenter(baseView, m.palette.View())
	}

	return baseView
}

// overlayCenter places a panel centered over the base view
func (m Model) overlayCenter(base, panel string) string {
	baseLines := strings.Split(base, "\n")
	panelLines := strings.Split(panel, "\n")

	// Calculate vertical position (center)
	panelHeight := len(panelLines)
	startRow := (m.height - panelHeight) / 2
	if startRow < 0 {
		startRow = 0
	}

	// Calculate horizontal centering
	panelWidth := 0
	for _, line := range panelLines {
		w := lipgloss.Width(line)
		if w > panelWidth {
			panelWidth = w
		}
	}
	leftPad := (m.width - panelWidth) / 2
	if leftPad < 0 {
		leftPad = 0
	}

	// Build result by overlaying panel on base
	result := make([]string, m.height)
	for i := 0; i < m.height; i++ {
		if i < len(baseLines) {
			result[i] = baseLines[i]
		} else {
			result[i] = ""
		}
	}

	// Overlay panel lines
	for i, panelLine := range panelLines {
		row := startRow + i
		if row >= 0 && row < m.height {
			// Pad the panel line and place it
			paddedPanel := strings.Repeat(" ", leftPad) + panelLine
			// Ensure line is long enough
			if len(result[row]) < m.width {
				result[row] = result[row] + strings.Repeat(" ", m.width-len(result[row]))
			}
			result[row] = paddedPanel
		}
	}

	return strings.Join(result, "\n")
}

// Helper functions

func getCurrentDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "~"
	}
	return dir
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
