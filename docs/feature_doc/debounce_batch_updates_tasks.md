# Priority 5: Debounce and Batch Updates - Implementation Tasks

## Overview

Apply debouncing and batching patterns to reduce UI flicker and improve performance during high-frequency updates.

**Current Status:**
- ✅ **Resize Events**: Already debounced (150ms delay via `resizeDebounceID`)
- ⏳ **PTY Output**: Reads per-message, no batching
- ⏳ **Stream Content**: Updates sidebar per-chunk, no throttling
- ⏳ **Viewport Updates**: Appends immediately on each PTY message

---

## Task 1: Batch PTY Output Reads

**Problem**: Each PTY read (up to 4KB) triggers an immediate `Update()` cycle.

**Solution**: Accumulate PTY data in a buffer and flush periodically or when buffer reaches threshold.

### Files to Modify

#### [MODIFY] `pkg/ui/model.go`

1. Add batching fields to Model:
```go
type Model struct {
    // ... existing fields ...
    
    // PTY output batching
    ptyBatchBuffer  []byte        // Accumulated PTY data
    ptyBatchTimer   bool          // Whether flush timer is pending
    ptyBatchMaxSize int           // Max bytes before forced flush (default: 16KB)
    ptyBatchMaxWait time.Duration // Max time before flush (default: 16ms)
}
```

2. Create batch messages:
```go
type ptyBatchFlushMsg struct{}
```

3. Modify `ptyOutputMsg` handler:
```go
case ptyOutputMsg:
    // Append to batch buffer
    m.ptyBatchBuffer = append(m.ptyBatchBuffer, msg.data...)
    
    // Force flush if buffer exceeds threshold
    if len(m.ptyBatchBuffer) >= m.ptyBatchMaxSize {
        m.flushPTYBatch()
        return m, listenToPTY(m.ptyFile)
    }
    
    // Start flush timer if not already pending
    var cmd tea.Cmd
    if !m.ptyBatchTimer {
        m.ptyBatchTimer = true
        cmd = tea.Tick(m.ptyBatchMaxWait, func(time.Time) tea.Msg {
            return ptyBatchFlushMsg{}
        })
    }
    
    return m, tea.Batch(cmd, listenToPTY(m.ptyFile))

case ptyBatchFlushMsg:
    m.ptyBatchTimer = false
    if len(m.ptyBatchBuffer) > 0 {
        m.flushPTYBatch()
    }
    return m, nil
```

4. Implement flush helper:
```go
func (m *Model) flushPTYBatch() {
    data := m.ptyBatchBuffer
    m.ptyBatchBuffer = m.ptyBatchBuffer[:0]
    
    // Process accumulated data
    chunks := m.altScreenState.SplitTransitions(data)
    // ... existing chunk processing logic ...
}
```

### Estimated Effort: 1-2 hours

---

## Task 2: Throttle Stream Content Updates

**Problem**: Each streaming chunk (even single tokens) triggers sidebar re-render.

**Solution**: Debounce sidebar updates during active streaming.

### Files to Modify

#### [MODIFY] `pkg/ui/model.go`

1. Add throttle fields:
```go
type Model struct {
    // ... existing fields ...
    
    // Stream update throttling
    streamThrottlePending bool
    streamThrottleDelay   time.Duration // Default: 50ms
}
```

2. Create throttle message:
```go
type streamThrottleFlushMsg struct{}
```

3. Modify `WtfStreamEvent` handler:
```go
case commands.WtfStreamEvent:
    if msg.Delta != "" {
        m.wtfContent += msg.Delta
        
        // Throttle sidebar updates
        if !m.streamThrottlePending {
            m.streamThrottlePending = true
            return m, tea.Batch(
                tea.Tick(m.streamThrottleDelay, func(time.Time) tea.Msg {
                    return streamThrottleFlushMsg{}
                }),
                listenToWtfStream(m.wtfStream),
            )
        }
        return m, listenToWtfStream(m.wtfStream)
    }
    // ... rest of handler ...

case streamThrottleFlushMsg:
    m.streamThrottlePending = false
    if m.sidebar != nil && m.sidebar.IsVisible() {
        m.sidebar.SetContent(m.wtfContent)
    }
    return m, nil
```

### Estimated Effort: 30-45 minutes

---

## Task 3: Coalesce Viewport Updates

**Problem**: `viewport.AppendOutput()` is called for each PTY chunk, triggering re-render.

**Solution**: Viewport updates are handled by Task 1 (batching at PTY level).

**Additional Optimization**: Ensure viewport only re-renders content when actually changed.

### Files to Modify

#### [MODIFY] `pkg/ui/components/viewport/viewport.go`

1. Add dirty flag:
```go
type PTYViewport struct {
    // ... existing fields ...
    dirty bool // True if content changed since last View()
}
```

2. Track changes in `AppendOutput`:
```go
func (v *PTYViewport) AppendOutput(data []byte) {
    if len(data) == 0 {
        return
    }
    v.content = terminal.AppendPTYContent(v.content, data, &v.pendingCR)
    v.dirty = true
    // ... rest of method ...
}
```

3. Clear flag in `View`:
```go
func (v *PTYViewport) View() string {
    v.dirty = false
    // ... existing render logic ...
}
```

### Estimated Effort: 15-30 minutes

---

## Verification Plan

### Unit Tests

```go
// pkg/ui/model_test.go

func TestPTYOutputBatching(t *testing.T) {
    m := NewTestModel()
    m.ptyBatchMaxSize = 100
    m.ptyBatchMaxWait = 10 * time.Millisecond
    
    // Send multiple small chunks
    for i := 0; i < 5; i++ {
        m, _ = m.Update(ptyOutputMsg{data: []byte("chunk")})
    }
    
    // Buffer should have accumulated data
    if len(m.ptyBatchBuffer) != 25 {
        t.Errorf("Expected 25 bytes buffered, got %d", len(m.ptyBatchBuffer))
    }
    
    // Trigger flush
    m, _ = m.Update(ptyBatchFlushMsg{})
    
    // Buffer should be empty
    if len(m.ptyBatchBuffer) != 0 {
        t.Error("Expected buffer to be flushed")
    }
}

func TestStreamThrottling(t *testing.T) {
    m := NewTestModel()
    m.streamThrottleDelay = 10 * time.Millisecond
    
    // Rapid updates should not immediately update sidebar
    for i := 0; i < 10; i++ {
        m, _ = m.Update(commands.WtfStreamEvent{Delta: "x"})
    }
    
    // Content accumulated but sidebar not yet updated
    if m.wtfContent != "xxxxxxxxxx" {
        t.Errorf("Expected accumulated content")
    }
    if !m.streamThrottlePending {
        t.Error("Expected throttle pending")
    }
}
```

### Manual Testing

1. **High-throughput commands**: Run `find / -name "*.go"` and verify no flicker
2. **Streaming responses**: Use `/wtf` command and verify smooth sidebar updates
3. **Resize during output**: Resize while command is running, verify clean behavior

---

## Execution Order

| Step | Task | Effort | Priority |
|------|------|--------|----------|
| 1 | Task 2: Stream Throttling | 30-45 min | High (visible improvement) |
| 2 | Task 1: PTY Batching | 1-2 hours | Medium (performance) |
| 3 | Task 3: Viewport Dirty Flag | 15-30 min | Low (micro-optimization) |

**Total Estimated Effort: 2-3 hours**

---

## Configuration Options

Consider making these configurable via `pkg/config`:

```go
type Config struct {
    // ... existing fields ...
    
    UI struct {
        PTYBatchMaxSize   int           `json:"pty_batch_max_size"`   // Default: 16384
        PTYBatchMaxWait   time.Duration `json:"pty_batch_max_wait"`   // Default: 16ms
        StreamThrottleMs  int           `json:"stream_throttle_ms"`   // Default: 50
    } `json:"ui"`
}
```

---

## Notes

- **Trade-off**: Batching adds latency but reduces CPU usage and flicker
- **Tuning**: Values can be adjusted based on user feedback
- **16ms**: Aligns with 60fps refresh rate for smooth animation
- **50ms for streams**: Acceptable latency for LLM streaming responses
