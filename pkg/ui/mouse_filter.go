package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
)

const mouseEventMinInterval = 15 * time.Millisecond

var lastMouseEvent time.Time

// MouseEventFilter drops very high frequency mouse motion/wheel updates before
// they reach the model. Click and release messages are never filtered.
func MouseEventFilter(_ tea.Model, msg tea.Msg) tea.Msg {
	switch msg.(type) {
	case tea.MouseMotionMsg, tea.MouseWheelMsg:
		now := time.Now()
		if !lastMouseEvent.IsZero() && now.Sub(lastMouseEvent) < mouseEventMinInterval {
			return nil
		}
		lastMouseEvent = now
	}
	return msg
}
