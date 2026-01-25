package ui

import (
	"bytes"
	"context"
	"log/slog"

	"wtf_cli/pkg/logging"
)

func (m *Model) flushPTYBatch() {
	data := m.ptyBatchBuffer
	m.ptyBatchBuffer = m.ptyBatchBuffer[:0]

	// Process accumulated data
	chunks := m.altScreenState.SplitTransitions(data)
	for i, chunk := range chunks {
		if m.inputHandler != nil {
			m.inputHandler.UpdateTerminalModes(chunk.Data)
		}
		if len(chunk.Data) > 0 {
			hasLeft := bytes.Contains(chunk.Data, []byte("\x1b[D")) || bytes.Contains(chunk.Data, []byte("\x1bOD"))
			hasRight := bytes.Contains(chunk.Data, []byte("\x1b[C")) || bytes.Contains(chunk.Data, []byte("\x1bOC"))
			hasBackspace := bytes.Contains(chunk.Data, []byte{0x08}) || bytes.Contains(chunk.Data, []byte{0x7f})
			if hasLeft || hasRight || hasBackspace {
				logger := slog.Default()
				ctx := context.Background()
				if logger.Enabled(ctx, logging.LevelTrace) {
					logger.Log(ctx, logging.LevelTrace, "pty_output_nav", "left", hasLeft, "right", hasRight, "backspace", hasBackspace, "len", len(chunk.Data))
				}
			}
		}

		if chunk.Entering {
			if !m.fullScreenMode {
				m.enterFullScreen(len(chunk.Data))
			}
			if m.fullScreenPanel != nil && len(chunk.Data) > 0 {
				m.fullScreenPanel.Write(chunk.Data)
			}
			continue
		}

		if chunk.Exiting {
			if m.fullScreenMode && hasFutureEnter(chunks[i+1:]) {
				if m.fullScreenPanel != nil && len(chunk.Data) > 0 {
					m.fullScreenPanel.Write(chunk.Data)
				}
				continue
			}

			if m.fullScreenMode && m.fullScreenPanel != nil && len(chunk.Data) > 0 {
				m.fullScreenPanel.Write(chunk.Data)
			}
			if m.fullScreenMode {
				m.exitFullScreen()
			} else if len(chunk.Data) > 0 {
				m.appendNormalizedLines(chunk.Data)
				m.viewport.AppendOutput(chunk.Data)
			}
			continue
		}

		if len(chunk.Data) == 0 {
			continue
		}

		if m.fullScreenMode {
			// Full-screen mode: send to panel, NOT to buffer (buffer isolation)
			if m.fullScreenPanel != nil {
				m.fullScreenPanel.Write(chunk.Data)
			}
		} else {
			// Normal mode: append to viewport AND buffer
			m.appendNormalizedLines(chunk.Data)
			m.viewport.AppendOutput(chunk.Data)
		}
	}
}
