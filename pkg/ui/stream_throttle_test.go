package ui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
)

func TestStreamThrottling(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.streamThrottleDelay = 10 * time.Millisecond
	m.startStreamPlaceholder()

	// First chunk should update immediately and start timer
	updated, _ := m.Update(commands.WtfStreamEvent{Delta: "chunk1"})
	m = updated.(Model)

	if got := latestAssistantMessageContent(t, m); got != "chunk1" {
		t.Errorf("Expected content 'chunk1', got %q", got)
	}

	if !m.streamThrottlePending {
		t.Error("Expected throttle to be pending after first chunk")
	}

	// Rapid subsequent updates should not trigger immediate sidebar updates
	for i := 2; i <= 10; i++ {
		updated, _ = m.Update(commands.WtfStreamEvent{Delta: "x"})
		m = updated.(Model)
	}

	// Content should accumulate on the streaming assistant message.
	if got := latestAssistantMessageContent(t, m); got != "chunk1xxxxxxxxx" {
		t.Errorf("Expected accumulated content, got %q", got)
	}

	// Timer should still be pending (only one timer set)
	if !m.streamThrottlePending {
		t.Error("Expected throttle to remain pending")
	}

	// Trigger flush
	updated, _ = m.Update(streamThrottleFlushMsg{})
	m = updated.(Model)

	// Timer should be reset
	if m.streamThrottlePending {
		t.Error("Expected throttle to be reset after flush")
	}
}

func TestStreamDoneUpdates(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.streamThrottleDelay = 10 * time.Millisecond
	m.startStreamPlaceholder()

	// Add some content
	updated, _ := m.Update(commands.WtfStreamEvent{Delta: "content"})
	m = updated.(Model)

	// Mark as done
	updated, _ = m.Update(commands.WtfStreamEvent{Done: true})
	m = updated.(Model)

	// Stream should be cleared
	if m.wtfStream != nil {
		t.Error("Expected wtfStream to be nil after done")
	}

	// Throttle should be reset
	if m.streamThrottlePending {
		t.Error("Expected throttle to be reset after done")
	}
	if m.sidebar.IsStreaming() {
		t.Error("Expected sidebar streaming state to be false after done")
	}
}

func TestStreamErrorResetsThrottle(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.streamThrottleDelay = 10 * time.Millisecond
	m.startStreamPlaceholder()

	// Start stream
	updated, _ := m.Update(commands.WtfStreamEvent{Delta: "start"})
	m = updated.(Model)

	if !m.streamThrottlePending {
		t.Error("Expected throttle pending")
	}

	// Send error
	updated, _ = m.Update(commands.WtfStreamEvent{Err: fmt.Errorf("test error")})
	m = updated.(Model)

	// Throttle should be reset
	if m.streamThrottlePending {
		t.Error("Expected throttle to be reset on error")
	}

	// Stream should be cleared
	if m.wtfStream != nil {
		t.Error("Expected wtfStream to be nil after error")
	}
	if got := latestAssistantMessageContent(t, m); !strings.Contains(got, "test error") {
		t.Errorf("Expected latest assistant message to include error, got %q", got)
	}
}

func latestAssistantMessageContent(t *testing.T, m Model) string {
	t.Helper()
	if m.sidebar == nil {
		t.Fatal("Expected sidebar to be initialized")
	}
	messages := m.sidebar.GetMessages()
	if len(messages) == 0 {
		t.Fatal("Expected at least one sidebar message")
	}
	latest := messages[len(messages)-1]
	if latest.Role != "assistant" {
		t.Fatalf("Expected latest role assistant, got %q", latest.Role)
	}
	return latest.Content
}
