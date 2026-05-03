package ui

import (
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
)

// --- formatToolCallStart ---

func TestFormatToolCallStart_Basic(t *testing.T) {
	info := &commands.ToolCallInfo{
		Name:     "read_file",
		ArgsJSON: `{"path":"main.go","start_line":1,"end_line":50}`,
	}
	got := formatToolCallStart(info)
	if !strings.Contains(got, "read_file") {
		t.Errorf("expected tool name in output, got %q", got)
	}
	if !strings.Contains(got, "main.go") {
		t.Errorf("expected args in output, got %q", got)
	}
	if !strings.HasPrefix(got, "\n\n🔧") {
		t.Errorf("expected newline prefix and tool emoji, got %q", got)
	}
}

func TestFormatToolCallStart_LongArgsTruncated(t *testing.T) {
	longArgs := `{"path":"` + strings.Repeat("a", 200) + `"}`
	info := &commands.ToolCallInfo{Name: "read_file", ArgsJSON: longArgs}
	got := formatToolCallStart(info)
	if !strings.Contains(got, "…") {
		t.Errorf("expected truncation ellipsis for long args, got %q", got)
	}
	// The formatted line (excluding the leading \n\n→ Tool: read_file() wrapper)
	// should not balloon unboundedly.
	if len(got) > 200 {
		t.Errorf("expected truncated output, got length %d", len(got))
	}
}

// --- formatToolCallSuffix ---

func TestFormatToolCallSuffix_Success(t *testing.T) {
	info := &commands.ToolCallInfo{
		Result: "line1\nline2\nline3",
	}
	got := formatToolCallSuffix(info)
	if !strings.Contains(got, "3 lines") {
		t.Errorf("expected line count, got %q", got)
	}
}

func TestFormatToolCallSuffix_Denied(t *testing.T) {
	info := &commands.ToolCallInfo{Denied: true}
	got := formatToolCallSuffix(info)
	if !strings.Contains(got, "denied") {
		t.Errorf("expected 'denied' in output, got %q", got)
	}
}

func TestFormatToolCallSuffix_Error(t *testing.T) {
	info := &commands.ToolCallInfo{ErrorMessage: "path outside cwd"}
	got := formatToolCallSuffix(info)
	if !strings.Contains(got, "error") {
		t.Errorf("expected 'error' in output, got %q", got)
	}
	if !strings.Contains(got, "path outside cwd") {
		t.Errorf("expected error message, got %q", got)
	}
}

func TestFormatToolCallSuffix_LongErrorTruncated(t *testing.T) {
	info := &commands.ToolCallInfo{ErrorMessage: strings.Repeat("x", 200)}
	got := formatToolCallSuffix(info)
	if !strings.Contains(got, "…") {
		t.Errorf("expected truncation ellipsis for long error, got %q", got)
	}
}

func TestFormatToolCallSuffix_EmptyResult(t *testing.T) {
	info := &commands.ToolCallInfo{Result: ""}
	got := formatToolCallSuffix(info)
	if !strings.Contains(got, "no output") {
		t.Errorf("expected 'no output' for empty result, got %q", got)
	}
}

// --- ToolCallStart event handling ---

func TestToolCallStartEvent_AppendsToCurrentMessage(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.streamThrottleDelay = 10 * time.Millisecond
	m.startStreamPlaceholder()

	// Simulate assistant text arriving first.
	updated, _ := m.Update(commands.WtfStreamEvent{Delta: "I'll read the file."})
	m = updated.(Model)

	info := &commands.ToolCallInfo{Name: "read_file", ArgsJSON: `{"path":"main.go"}`}
	updated, _ = m.Update(commands.WtfStreamEvent{ToolCallStart: info})
	m = updated.(Model)

	content := latestAssistantMessageContent(t, m)
	if !strings.Contains(content, "read_file") {
		t.Errorf("expected tool name in message content, got %q", content)
	}
	if !strings.Contains(content, "I'll read the file.") {
		t.Errorf("expected preceding text preserved, got %q", content)
	}
	// Still one message (tool call appended inline).
	msgs := m.sidebar.GetMessages()
	if len(msgs) != 1 {
		t.Errorf("expected 1 message, got %d", len(msgs))
	}
}

func TestToolCallStartEvent_ReplacesPlaceholderWhenNoTextYet(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.startStreamPlaceholder()

	// ToolCallStart fires before any text delta (placeholder still active).
	info := &commands.ToolCallInfo{Name: "read_file", ArgsJSON: `{"path":"x.go"}`}
	updated, _ := m.Update(commands.WtfStreamEvent{ToolCallStart: info})
	m = updated.(Model)

	if m.streamPlaceholderActive {
		t.Error("expected streamPlaceholderActive=false after ToolCallStart replaces it")
	}
	content := latestAssistantMessageContent(t, m)
	if strings.Contains(content, "Thinking") {
		t.Errorf("expected placeholder to be replaced, got %q", content)
	}
	if !strings.Contains(content, "read_file") {
		t.Errorf("expected tool name in replaced placeholder, got %q", content)
	}
}

// --- ToolCallFinished event handling ---

func TestToolCallFinishedEvent_AppendsSuffix(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.startStreamPlaceholder()

	info := &commands.ToolCallInfo{Name: "read_file", ArgsJSON: `{"path":"x.go"}`}
	updated, _ := m.Update(commands.WtfStreamEvent{ToolCallStart: info})
	m = updated.(Model)

	finished := &commands.ToolCallInfo{
		Name:     "read_file",
		ArgsJSON: `{"path":"x.go"}`,
		Result:   "line1\nline2",
	}
	updated, _ = m.Update(commands.WtfStreamEvent{ToolCallFinished: finished})
	m = updated.(Model)

	content := latestAssistantMessageContent(t, m)
	if !strings.Contains(content, "2 lines") {
		t.Errorf("expected line count in finished suffix, got %q", content)
	}
}

func TestToolCallFinishedEvent_SetsTurnNeededFlag(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.startStreamPlaceholder()

	finished := &commands.ToolCallInfo{Name: "read_file", Result: "ok"}
	updated, _ := m.Update(commands.WtfStreamEvent{ToolCallFinished: finished})
	m = updated.(Model)

	if !m.toolCallNewTurnNeeded {
		t.Error("expected toolCallNewTurnNeeded=true after ToolCallFinished")
	}
}

// --- New message on next delta after tool call ---

func TestNextDeltaAfterToolCallStartsNewMessage(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.startStreamPlaceholder()

	// Simulate one agent iteration: text → tool call → tool done.
	updated, _ := m.Update(commands.WtfStreamEvent{Delta: "Checking the file."})
	m = updated.(Model)

	start := &commands.ToolCallInfo{Name: "read_file", ArgsJSON: `{"path":"x.go"}`}
	updated, _ = m.Update(commands.WtfStreamEvent{ToolCallStart: start})
	m = updated.(Model)

	finished := &commands.ToolCallInfo{Name: "read_file", Result: "content here"}
	updated, _ = m.Update(commands.WtfStreamEvent{ToolCallFinished: finished})
	m = updated.(Model)

	// Next LLM iteration starts producing text.
	updated, _ = m.Update(commands.WtfStreamEvent{Delta: "Based on the file,"})
	m = updated.(Model)

	msgs := m.sidebar.GetMessages()
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages (tool-call turn + continuation), got %d", len(msgs))
	}
	if !strings.Contains(msgs[0].Content, "Checking the file.") {
		t.Errorf("first message should contain initial text, got %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[0].Content, "read_file") {
		t.Errorf("first message should contain tool call, got %q", msgs[0].Content)
	}
	if !strings.Contains(msgs[1].Content, "Based on the file,") {
		t.Errorf("second message should contain continuation text, got %q", msgs[1].Content)
	}
	if m.toolCallNewTurnNeeded {
		t.Error("toolCallNewTurnNeeded should be cleared after first delta of new turn")
	}
}
