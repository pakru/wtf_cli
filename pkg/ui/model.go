package ui

import (
	"os"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Model represents the Bubble Tea application state
type Model struct {
	// PTY connection
	ptyFile *os.File

	// UI Components
	viewport     PTYViewport    // Viewport for PTY output
	statusBar    *StatusBarView // Status bar at bottom
	inputHandler *InputHandler  // Input routing to PTY

	// Data
	buffer     *buffer.CircularBuffer
	session    *capture.SessionContext
	currentDir string

	// UI state
	width  int
	height int
	ready  bool

	// Features (will be added in later tasks)
	showCommandPalette bool
	commandInput       string
}

// NewModel creates a new Bubble Tea model
func NewModel(ptyFile *os.File, buf *buffer.CircularBuffer, sess *capture.SessionContext) Model {
	return Model{
		ptyFile:      ptyFile,
		viewport:     NewPTYViewport(),
		statusBar:    NewStatusBarView(),
		inputHandler: NewInputHandler(ptyFile),
		buffer:       buf,
		session:      sess,
		currentDir:   getCurrentDir(),
	}
}

// Init initializes the model (Bubble Tea lifecycle method)
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		listenToPTY(m.ptyFile), // Start listening to PTY output
	)
}

// Update handles messages and updates model state (Bubble Tea lifecycle method)
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true

		// Update viewport size (leave room for status bar = 1 line)
		m.viewport.SetSize(msg.Width, msg.Height-1)
		return m, nil

	case tea.KeyMsg:
		// Use input handler to route keys to PTY
		handled, cmd := m.inputHandler.HandleKey(msg)
		if handled {
			return m, cmd
		}

		// If not handled by input handler, ignore
		// (most keys should go to PTY)
		return m, nil

	case ptyOutputMsg:
		// PTY sent output - append to viewport
		m.viewport.AppendOutput(msg.data)

		// Schedule next read
		return m, listenToPTY(m.ptyFile)

	case ptyErrorMsg:
		// PTY error - probably shell exited
		return m, tea.Quit
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

	// Combine viewport (top) and status bar (bottom)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.viewport.View(),
		m.statusBar.Render(),
	)
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
