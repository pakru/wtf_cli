package ui

import (
	"log/slog"
	"os"
	"time"

	"wtf_cli/pkg/capture"

	tea "charm.land/bubbletea/v2"
)

// PTY message types
type ptyOutputMsg struct {
	data []byte
}

type ptyErrorMsg struct {
	err error
}

func (m Model) handlePTYOutput(msg ptyOutputMsg) (Model, tea.Cmd) {
	// Suppress PTY output briefly after resize to prevent prompt reprint from showing
	if !m.resizeTime.IsZero() && time.Since(m.resizeTime) < 100*time.Millisecond {
		// Skip appending to viewport but still schedule next read
		return m, listenToPTY(m.ptyFile)
	}

	if len(msg.data) > 0 {
		m.startupPTYOutputSeen = true
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
}

func (m Model) handlePTYBatchFlush() (Model, tea.Cmd) {
	m.ptyBatchTimer = false
	if len(m.ptyBatchBuffer) > 0 {
		m.flushPTYBatch()
	}
	return m, nil
}

func (m Model) handlePTYError(msg ptyErrorMsg) (Model, tea.Cmd) {
	// PTY error - probably shell exited
	slog.Error("pty_error", "error", msg.err)
	return m, tea.Quit
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
