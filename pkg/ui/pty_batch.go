package ui

func (m *Model) flushPTYBatch() {
	data := m.ptyBatchBuffer
	m.ptyBatchBuffer = m.ptyBatchBuffer[:0]

	// Process accumulated data
	chunks := m.altScreenState.SplitTransitions(data)
	for i, chunk := range chunks {
		if m.inputHandler != nil {
			m.inputHandler.UpdateTerminalModes(chunk.Data)
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
				m.appendPTYOutput(chunk.Data)
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
			m.appendPTYOutput(chunk.Data)
			m.viewport.AppendOutput(chunk.Data)
		}
	}
}
