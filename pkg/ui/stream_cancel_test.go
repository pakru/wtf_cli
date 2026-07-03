package ui

import (
	"testing"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/testutils"
)

func TestModel_EscCancelsActiveStream(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	m.sidebar.Show()
	m.sidebar.SetStreaming(true)
	m.sidebar.StartAssistantMessageWithContent("partial response")

	updated, cmd := m.Update(testutils.TestKeyEsc)
	if cmd != nil {
		t.Fatal("Esc cancellation should not emit a command")
	}
	m = updated.(Model)

	if !*canceled {
		t.Fatal("expected stream cancel func to be called")
	}
	if m.hasActiveStream() {
		t.Fatal("expected active stream state to be cleared")
	}
	if m.wtfStream != nil {
		t.Fatal("expected stream channel to be cleared")
	}
	if m.streamStartPending {
		t.Fatal("expected streamStartPending to be false")
	}
	if m.sidebar.IsStreaming() {
		t.Fatal("expected sidebar streaming state to be false")
	}
	if !m.sidebar.IsVisible() {
		t.Fatal("expected sidebar to remain visible")
	}

	messages := m.sidebar.GetMessages()
	if len(messages) != 2 {
		t.Fatalf("expected partial response plus cancel message, got %d messages", len(messages))
	}
	if messages[0].Content != "partial response" {
		t.Fatalf("expected partial response to be preserved, got %q", messages[0].Content)
	}
	if messages[1].Content != streamCanceledMessage {
		t.Fatalf("expected cancel message %q, got %q", streamCanceledMessage, messages[1].Content)
	}
}

func TestModel_EscCancelReplacesThinkingPlaceholder(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	m.startStreamPlaceholder()

	updated, _ := m.Update(testutils.TestKeyEsc)
	m = updated.(Model)

	if !*canceled {
		t.Fatal("expected stream cancel func to be called")
	}
	if m.streamPlaceholderActive {
		t.Fatal("expected placeholder state to be cleared")
	}
	if got := latestAssistantMessageContent(t, m); got != streamCanceledMessage {
		t.Fatalf("expected placeholder replaced with %q, got %q", streamCanceledMessage, got)
	}
}

func TestModel_EscCancelsToolApprovalWait(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	req := &commands.ApprovalRequest{
		Name:  "read_file",
		Reply: make(chan commands.ApprovalDecision, 1),
	}
	m.toolApproval.Show(req)

	updated, _ := m.Update(testutils.TestKeyEsc)
	m = updated.(Model)

	if !*canceled {
		t.Fatal("expected stream cancel func to be called")
	}
	if m.toolApproval.IsVisible() {
		t.Fatal("expected tool approval popup to be hidden")
	}
	select {
	case decision := <-req.Reply:
		t.Fatalf("Esc should cancel the run via context, not send approval decision: %+v", decision)
	default:
	}
}

func TestModel_EscCancelsContinuePromptWait(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	req := &commands.ContinuationRequest{
		Reply: make(chan commands.ContinuationDecision, 1),
	}
	m.continuePrompt.Show(req)

	updated, _ := m.Update(testutils.TestKeyEsc)
	m = updated.(Model)

	if !*canceled {
		t.Fatal("expected stream cancel func to be called")
	}
	if m.continuePrompt.IsVisible() {
		t.Fatal("expected continue prompt to be hidden")
	}
	select {
	case decision := <-req.Reply:
		t.Fatalf("Esc should cancel the run via context, not send continuation decision: %+v", decision)
	default:
	}
}

func TestModel_EscLetsPaletteHandleBeforeCancelingActiveStream(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	m.palette.Show()

	updated, cmd := m.Update(testutils.TestKeyEsc)
	if cmd == nil {
		t.Fatal("expected palette Esc handling to emit a cancel command")
	}
	m = updated.(Model)

	if *canceled {
		t.Fatal("expected palette Esc handling not to cancel active stream")
	}
	if !m.hasActiveStream() {
		t.Fatal("expected active stream to remain active")
	}
	if m.palette.IsVisible() {
		t.Fatal("expected palette to be hidden")
	}
}

func TestModel_EscLetsHistoryPickerHandleBeforeCancelingActiveStream(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	m.historyPicker.Show("", []string{"echo one"})

	updated, cmd := m.Update(testutils.TestKeyEsc)
	if cmd == nil {
		t.Fatal("expected history picker Esc handling to emit a cancel command")
	}
	m = updated.(Model)

	if *canceled {
		t.Fatal("expected history picker Esc handling not to cancel active stream")
	}
	if !m.hasActiveStream() {
		t.Fatal("expected active stream to remain active")
	}
	if m.historyPicker.IsVisible() {
		t.Fatal("expected history picker to be hidden")
	}
}

func TestModel_EscLetsSettingsHandleBeforeCancelingActiveStream(t *testing.T) {
	m, canceled := modelWithCancelableStream()
	m.settingsPanel.Show(config.Default(), "/tmp/test_config.json")

	updated, cmd := m.Update(testutils.TestKeyEsc)
	if cmd == nil {
		t.Fatal("expected settings Esc handling to emit a close command")
	}
	m = updated.(Model)

	if *canceled {
		t.Fatal("expected settings Esc handling not to cancel active stream")
	}
	if !m.hasActiveStream() {
		t.Fatal("expected active stream to remain active")
	}
	if m.settingsPanel.IsVisible() {
		t.Fatal("expected settings panel to be hidden")
	}
}

func TestModel_EscCancelIgnoresStaleStreamEvents(t *testing.T) {
	m, _ := modelWithCancelableStream()
	oldStreamID := m.streamID

	updated, _ := m.Update(testutils.TestKeyEsc)
	m = updated.(Model)

	updated, _ = m.Update(wtfStreamEventMsg{
		streamID: oldStreamID,
		event:    commands.WtfStreamEvent{Delta: " stale"},
	})
	m = updated.(Model)

	if got := latestAssistantMessageContent(t, m); got != streamCanceledMessage {
		t.Fatalf("expected stale event to be ignored, got latest message %q", got)
	}
}

func modelWithCancelableStream() (Model, *bool) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	canceled := false
	m.streamID = 1
	m.wtfStream = make(chan commands.WtfStreamEvent)
	m.streamCancel = func() {
		canceled = true
	}
	return m, &canceled
}
