package ui

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/ui/components/continueprompt"
	"wtf_cli/pkg/ui/components/sidebar"
	"wtf_cli/pkg/ui/components/toolapproval"

	tea "charm.land/bubbletea/v2"
)

// Stream invariants:
// - First delta refreshes immediately; follow-up deltas are batched until streamThrottleFlushMsg.
// - Stream listener is re-armed after every stream event, including tool-approval events.
// - Tool approval modal remains topmost overlay.

type streamThrottleFlushMsg struct {
	streamID int
}

type streamStartOrigin int

const (
	streamOriginExplain streamStartOrigin = iota
	streamOriginChat
)

type streamStartResultMsg struct {
	streamID int
	origin   streamStartOrigin
	stream   <-chan commands.WtfStreamEvent
	err      error
	result   *commands.Result
}

type wtfStreamEventMsg struct {
	streamID int
	event    commands.WtfStreamEvent
}

type contextStreamingHandler interface {
	StartStreamWithContext(context.Context, *commands.Context) (<-chan commands.WtfStreamEvent, error)
}

func (m Model) handleStreamStartResult(msg streamStartResultMsg) (Model, tea.Cmd) {
	if msg.streamID != m.streamID {
		return m, nil
	}
	m.streamStartPending = false
	if msg.err != nil {
		slog.Error("wtf_stream_start_error", "error", msg.err)
		if m.sidebar != nil {
			m.sidebar.SetStreaming(false)
			m.clearStreamPlaceholder()
			m.sidebar.AppendErrorMessage(msg.err.Error())
			m.sidebar.RefreshView()
		} else {
			m.resultPanel.Show("Error", fmt.Sprintf("Error: %v", msg.err))
		}
		m.endStreamRun()
		return m, nil
	}

	if msg.stream == nil {
		if m.sidebar != nil {
			m.sidebar.SetStreaming(false)
			m.clearStreamPlaceholder()
			if msg.origin == streamOriginExplain && msg.result != nil {
				m.sidebar.StartAssistantMessageWithContent(msg.result.Content)
				m.sidebar.RefreshView()
			}
		} else if msg.origin == streamOriginExplain && msg.result != nil {
			m.resultPanel.Show(msg.result.Title, msg.result.Content)
		}
		m.endStreamRun()
		return m, nil
	}

	m.wtfStream = msg.stream
	return m, m.continueStreamListen()
}

func (m Model) handleToolApprovalDecision(msg toolapproval.DecisionMsg) (Model, tea.Cmd) {
	// User picked an option in the approval popup. Map to ApprovalDecision
	// and dispatch on the agent loop's Reply channel (capacity 1, so the
	// send never blocks). Hiding the panel before sending keeps the View
	// consistent if subsequent events arrive immediately.
	if msg.Request == nil || msg.Request.Reply == nil {
		if m.toolApproval != nil {
			m.toolApproval.Hide()
		}
		return m, nil
	}
	decision := commands.ApprovalDecision{}
	switch msg.Kind {
	case toolapproval.DecisionAllowOnce:
		decision = commands.ApprovalDecision{Allow: true}
	case toolapproval.DecisionAllowSession:
		decision = commands.ApprovalDecision{Allow: true, Persistent: true}
	case toolapproval.DecisionDeny:
		decision = commands.ApprovalDecision{Allow: false}
	}
	logArgs := []any{
		"tool", msg.Request.Name,
		"allow", decision.Allow,
		"persistent", decision.Persistent,
	}
	if msg.Request.Escape != nil {
		// AllowOutsideWorkdir is not set here: UIApprover.approveEscape
		// unconditionally sets it on any allowed reply to an escape request
		// once it receives this decision, independent of what the UI sends —
		// this branch only adds log context, it carries no authorization.
		logArgs = append(logArgs,
			"outside_workdir", true,
			"resolved_path", msg.Request.Escape.ResolvedPath,
			"grant_dir", msg.Request.Escape.GrantDir,
		)
	}
	slog.Info("tool_approval_user_decision", logArgs...)
	if m.toolApproval != nil {
		m.toolApproval.Hide()
	}
	// Non-blocking send: Reply is buffered with capacity 1 by UIApprover.
	select {
	case msg.Request.Reply <- decision:
	default:
		slog.Warn("tool_approval_reply_dropped", "tool", msg.Request.Name)
	}
	return m, nil
}

func (m Model) handleContinuePromptDecision(msg continueprompt.DecisionMsg) (Model, tea.Cmd) {
	// User answered the "continue?" popup. Dispatch the decision on the agent
	// loop's Reply channel (capacity 1, so the send never blocks). Hiding the
	// panel before sending keeps the View consistent if events arrive
	// immediately. A "stop" decision makes the loop emit a graceful Done.
	if msg.Request == nil || msg.Request.Reply == nil {
		if m.continuePrompt != nil {
			m.continuePrompt.Hide()
		}
		return m, nil
	}
	slog.Info("continue_prompt_user_decision",
		"continue", msg.Continue,
		"tool_calls", msg.Request.ToolCalls,
	)
	if m.continuePrompt != nil {
		m.continuePrompt.Hide()
	}
	select {
	case msg.Request.Reply <- commands.ContinuationDecision{Continue: msg.Continue}:
	default:
		slog.Warn("continue_prompt_reply_dropped")
	}
	return m, nil
}

func (m Model) handleChatSubmit(msg sidebar.ChatSubmitMsg) (Model, tea.Cmd) {
	if m.sidebar == nil || msg.Content == "" {
		return m, nil
	}

	// Guard: refuse new stream while one is active (prevents deadlock)
	if m.hasActiveStream() {
		return m, nil
	}

	// Add user message to sidebar history
	m.sidebar.AppendUserMessage(msg.Content)
	m.sidebar.RefreshView()

	// Build context and start chat stream
	ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
	history := append([]ai.ChatMessage(nil), m.sidebar.GetMessages()...)
	runCtx, streamID := m.beginStreamRun()
	m.startStreamPlaceholder()
	return m, startChatStreamCmd(streamID, runCtx, ctx, m.chatHandler(), history)
}

func (m Model) handleWtfStreamEvent(msg commands.WtfStreamEvent) (Model, tea.Cmd) {
	if msg.Err != nil {
		slog.Error("wtf_stream_error", "error", msg.Err)
		// Clear all stream state (guard nil)
		if m.sidebar != nil {
			m.sidebar.SetStreaming(false)
			m.clearStreamPlaceholder()
			m.sidebar.AppendErrorMessage(msg.Err.Error())
			m.sidebar.RefreshView() // Ensure error is visible immediately
		}
		if m.toolApproval != nil {
			m.toolApproval.Hide()
		}
		if m.continuePrompt != nil {
			m.continuePrompt.Hide()
		}
		m.endStreamRun()
		return m, nil
	}

	// Tool approval popup: show modal, keep listening so subsequent events
	// (deltas, finished events) continue to flow through. The agent
	// goroutine is blocked on the request's Reply channel; the user's
	// answer is dispatched via toolapproval.DecisionMsg below.
	if msg.ToolApproval != nil {
		if m.toolApproval != nil {
			m.toolApproval.SetSize(m.width, m.height)
			m.toolApproval.Show(msg.ToolApproval)
		}
		showArgs := []any{"tool", msg.ToolApproval.Name}
		if esc := msg.ToolApproval.Escape; esc != nil {
			showArgs = append(showArgs,
				"outside_workdir", true,
				"resolved_path", esc.ResolvedPath,
				"grant_dir", esc.GrantDir,
			)
		}
		slog.Info("tool_approval_show", showArgs...)
		return m, m.continueStreamListen()
	}

	// Continue prompt: the loop hit its per-batch iteration limit and is asking
	// whether to keep going. Same modal pattern as approval — the agent
	// goroutine blocks on Reply until continueprompt.DecisionMsg is dispatched.
	if msg.ContinuePrompt != nil {
		if m.continuePrompt != nil {
			m.continuePrompt.SetSize(m.width, m.height)
			m.continuePrompt.Show(msg.ContinuePrompt)
		}
		slog.Info("continue_prompt_show", "tool_calls", msg.ContinuePrompt.ToolCalls)
		return m, m.continueStreamListen()
	}

	if msg.ToolCallStart != nil {
		if m.sidebar != nil {
			line := formatToolCallStart(msg.ToolCallStart)
			if m.streamPlaceholderActive {
				m.sidebar.SetLastMessageContent(line)
				m.streamPlaceholderActive = false
			} else {
				m.sidebar.UpdateLastMessage(line)
			}
			m.sidebar.RefreshView()
		}
		return m, m.continueStreamListen()
	}

	if msg.ToolCallFinished != nil {
		if m.sidebar != nil {
			m.sidebar.UpdateLastMessage(formatToolCallSuffix(msg.ToolCallFinished))
			m.sidebar.RefreshView()
		}
		m.toolCallNewTurnNeeded = true
		return m, m.continueStreamListen()
	}

	if m.sidebar != nil {
		if msg.Delta != "" {
			// Ensure streaming state is active
			if !m.sidebar.IsStreaming() {
				m.sidebar.SetStreaming(true)
			}

			// After a tool call, start a fresh assistant message so the
			// tool call line and the continuation text are visually separate.
			if m.toolCallNewTurnNeeded {
				m.toolCallNewTurnNeeded = false
				m.sidebar.StartAssistantMessageWithContent(msg.Delta)
				m.sidebar.RefreshView()
				return m, m.continueStreamListen()
			}

			// Replace placeholder on first real delta
			if !m.replaceStreamPlaceholder(msg.Delta) {
				m.sidebar.UpdateLastMessage(msg.Delta)
			}

			// Throttle rendering
			if !m.streamThrottlePending {
				m.streamThrottlePending = true
				// Immediate refresh on first chunk for responsiveness
				m.sidebar.RefreshView()
				return m, tea.Batch(
					tea.Tick(m.streamThrottleDelay, func(time.Time) tea.Msg {
						return streamThrottleFlushMsg{streamID: m.streamID}
					}),
					listenToWtfStream(m.streamID, m.wtfStream),
				)
			}
			// Subsequent chunks: just listen, don't schedule another tick
			return m, m.continueStreamListen()
		}
		if msg.Done {
			m.clearStreamPlaceholder()
			m.sidebar.SetStreaming(false)
			m.sidebar.RefreshView() // Final refresh
			m.endStreamRun()
			// A graceful stop can land right after a ToolCallFinished (e.g. the
			// user chose Stop at the continuation prompt) with no delta to clear
			// the flag. Reset it so the next stream's first delta replaces its
			// placeholder instead of being treated as a post-tool continuation.
			return m, nil
		}
	}
	return m, m.continueStreamListen()
}

func (m Model) handleStreamThrottleFlush(msg streamThrottleFlushMsg) (Model, tea.Cmd) {
	if msg.streamID != 0 && msg.streamID != m.streamID {
		return m, nil
	}
	m.streamThrottlePending = false

	// Re-render from chat messages.
	if m.sidebar != nil {
		m.sidebar.RefreshView() // Re-renders viewport from messages[]
	}
	return m, nil
}

func (m Model) continueStreamListen() tea.Cmd {
	if m.wtfStream == nil {
		return nil
	}
	return listenToWtfStream(m.streamID, m.wtfStream)
}

func (m *Model) beginStreamRun() (context.Context, int) {
	if m.streamCancel != nil {
		m.streamCancel()
	}
	m.streamID++
	runCtx, cancel := context.WithCancel(context.Background())
	m.streamCancel = cancel
	m.wtfStream = nil
	m.streamStartPending = true
	m.streamThrottlePending = false
	m.streamPlaceholderActive = false
	m.toolCallNewTurnNeeded = false
	return runCtx, m.streamID
}

func (m *Model) endStreamRun() {
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.wtfStream = nil
	m.streamStartPending = false
	m.streamThrottlePending = false
	m.toolCallNewTurnNeeded = false
}

func (m Model) hasActiveStream() bool {
	return m.streamCancel != nil || m.streamStartPending || m.wtfStream != nil
}

func (m Model) cancelActiveStream() (Model, tea.Cmd) {
	if !m.hasActiveStream() {
		return m, nil
	}
	if m.streamCancel != nil {
		m.streamCancel()
		m.streamCancel = nil
	}
	m.streamID++
	m.wtfStream = nil
	m.streamStartPending = false
	m.streamThrottlePending = false
	m.toolCallNewTurnNeeded = false
	if m.toolApproval != nil {
		m.toolApproval.Hide()
	}
	if m.continuePrompt != nil {
		m.continuePrompt.Hide()
	}
	m.showStreamCanceledMessage()
	return m, nil
}

func (m *Model) showStreamCanceledMessage() {
	if m.sidebar == nil {
		m.streamPlaceholderActive = false
		return
	}
	if m.streamPlaceholderActive {
		m.sidebar.SetLastMessageContent(streamCanceledMessage)
		m.streamPlaceholderActive = false
	} else {
		m.sidebar.StartAssistantMessageWithContent(streamCanceledMessage)
	}
	m.sidebar.SetStreaming(false)
	m.sidebar.RefreshView()
}

func (m *Model) buildExplainUserMessage(ctx *commands.Context) string {
	if ctx == nil {
		return "[Asked to explain output from terminal. Last command: N/A]"
	}
	lineCount := 0
	lines := ctx.GetLastNLines(ai.DefaultContextLines)
	if len(lines) > 0 {
		lineCount = len(lines)
	}

	command := "N/A"
	if ctx.Session != nil {
		last := ctx.Session.GetLastN(1)
		if len(last) > 0 && strings.TrimSpace(last[0].Command) != "" {
			command = strings.TrimSpace(last[0].Command)
		}
	}

	return fmt.Sprintf("[Asked to explain last %d lines from terminal. Last command: `%s`]", lineCount, command)
}

func listenToWtfStream(streamID int, stream <-chan commands.WtfStreamEvent) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-stream
		if !ok {
			event = commands.WtfStreamEvent{Done: true}
		}
		return wtfStreamEventMsg{streamID: streamID, event: event}
	}
}

func startExplainStreamCmd(streamID int, runCtx context.Context, ctx *commands.Context, handler commands.StreamingHandler, result *commands.Result) tea.Cmd {
	return func() tea.Msg {
		var (
			stream <-chan commands.WtfStreamEvent
			err    error
		)
		if h, ok := handler.(contextStreamingHandler); ok {
			stream, err = h.StartStreamWithContext(runCtx, ctx)
		} else {
			stream, err = handler.StartStream(ctx)
		}
		return streamStartResultMsg{
			streamID: streamID,
			origin:   streamOriginExplain,
			stream:   stream,
			err:      err,
			result:   result,
		}
	}
}

func startChatStreamCmd(streamID int, runCtx context.Context, ctx *commands.Context, handler *commands.ChatHandler, messages []ai.ChatMessage) tea.Cmd {
	return func() tea.Msg {
		stream, err := handler.StartChatStreamWithContext(runCtx, ctx, messages)
		return streamStartResultMsg{
			streamID: streamID,
			origin:   streamOriginChat,
			stream:   stream,
			err:      err,
		}
	}
}

func (m *Model) startStreamPlaceholder() {
	if m.sidebar == nil {
		return
	}
	if m.streamPlaceholderActive {
		return
	}
	m.sidebar.SetStreaming(true)
	m.sidebar.StartAssistantMessageWithContent(streamThinkingPlaceholder)
	m.streamPlaceholderActive = true
	m.sidebar.RefreshView()
}

func (m *Model) replaceStreamPlaceholder(delta string) bool {
	if m.sidebar == nil {
		return false
	}
	if !m.streamPlaceholderActive {
		return false
	}
	m.sidebar.SetLastMessageContent(delta)
	m.streamPlaceholderActive = false
	return true
}

func (m *Model) clearStreamPlaceholder() {
	if m.sidebar == nil {
		return
	}
	if m.streamPlaceholderActive {
		m.sidebar.RemoveLastMessage()
		m.streamPlaceholderActive = false
	}
}

func formatToolCallStart(info *commands.ToolCallInfo) string {
	args := info.ArgsJSON
	if len(args) > 120 {
		args = args[:120] + "…"
	}
	return fmt.Sprintf("\n\n%s%s(%s)", sidebar.MessagePrefix("tool"), info.Name, args)
}

func formatToolCallSuffix(info *commands.ToolCallInfo) string {
	if info.Denied {
		return " — denied"
	}
	if info.ErrorMessage != "" {
		msg := info.ErrorMessage
		if len(msg) > 80 {
			msg = msg[:80] + "…"
		}
		return fmt.Sprintf(" — error: %s", msg)
	}
	if info.Result == "" {
		return " — no output"
	}
	lineCount := strings.Count(info.Result, "\n") + 1
	return fmt.Sprintf(" — %d lines", lineCount)
}
