package ui

import (
	"testing"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
)

func TestPTYOutputBatching(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ptyBatchMaxSize = 100
	m.ptyBatchMaxWait = 10 * time.Millisecond

	// Send multiple small chunks
	for i := 0; i < 5; i++ {
		updated, _ := m.Update(ptyOutputMsg{data: []byte("chunk")})
		m = updated.(Model)
	}

	// Buffer should have accumulated data
	if len(m.ptyBatchBuffer) != 25 {
		t.Errorf("Expected 25 bytes buffered, got %d", len(m.ptyBatchBuffer))
	}

	// Timer should be pending
	if !m.ptyBatchTimer {
		t.Error("Expected batch timer to be pending")
	}

	// Trigger flush
	updated, _ := m.Update(ptyBatchFlushMsg{})
	m = updated.(Model)

	// Buffer should be empty
	if len(m.ptyBatchBuffer) != 0 {
		t.Error("Expected buffer to be flushed")
	}

	// Timer should be reset
	if m.ptyBatchTimer {
		t.Error("Expected batch timer to be reset")
	}
}

func TestPTYBatchForcedFlush(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ptyBatchMaxSize = 50
	m.ptyBatchMaxWait = 10 * time.Millisecond

	// Send data that exceeds threshold
	largeData := make([]byte, 60)
	for i := range largeData {
		largeData[i] = 'x'
	}

	updated, _ := m.Update(ptyOutputMsg{data: largeData})
	m = updated.(Model)

	// Buffer should be flushed immediately
	if len(m.ptyBatchBuffer) != 0 {
		t.Errorf("Expected buffer to be flushed after exceeding threshold, got %d bytes", len(m.ptyBatchBuffer))
	}
}

func TestPTYBatchTimerStartsOnce(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ptyBatchMaxSize = 1000
	m.ptyBatchMaxWait = 10 * time.Millisecond

	// First chunk should start timer
	updated, _ := m.Update(ptyOutputMsg{data: []byte("first")})
	m = updated.(Model)
	if !m.ptyBatchTimer {
		t.Error("Expected timer to start on first chunk")
	}

	// Second chunk should NOT change timer state
	prevTimer := m.ptyBatchTimer
	updated, _ = m.Update(ptyOutputMsg{data: []byte("second")})
	m = updated.(Model)
	if m.ptyBatchTimer != prevTimer {
		t.Error("Expected timer state to remain unchanged on second chunk")
	}

	// Buffer should accumulate both chunks
	if len(m.ptyBatchBuffer) != 11 {
		t.Errorf("Expected 11 bytes buffered (first+second), got %d", len(m.ptyBatchBuffer))
	}
}
