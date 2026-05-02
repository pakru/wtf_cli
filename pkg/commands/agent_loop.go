package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/tools"
)

// ErrMaxIterations is reported when the agent loop hits its iteration cap
// before the model produces a final, tool-call-free turn.
var ErrMaxIterations = errors.New("agent: max iterations exceeded")

// ApprovalDecision is the user's answer to an ApprovalRequest.
type ApprovalDecision struct {
	// Allow is true when the call should proceed.
	Allow bool

	// Persistent indicates the user picked "always allow this session" — the
	// approver is expected to remember this and auto-allow subsequent calls
	// to the same tool. Auto-allowed calls do NOT round-trip through Approve.
	Persistent bool
}

// ApprovalRequest is sent by the agent loop when it wants to invoke a tool
// that needs user permission. The loop blocks on Reply until the approver
// dispatches a decision (or the context is canceled).
type ApprovalRequest struct {
	ID    string
	Name  string
	Args  json.RawMessage
	Reply chan ApprovalDecision
}

// Approver decides whether a tool call should run.
//
// Implementations may run in any goroutine. The agent loop waits on Approve
// synchronously, so blocking implementations (e.g. a UI popup) are fine.
// Approve must respect ctx.Done(): when ctx is canceled it should return
// promptly with the context's error.
type Approver interface {
	Approve(ctx context.Context, req *ApprovalRequest) (ApprovalDecision, error)
}

// AutoAllowApprover approves every tool call without prompting. Used as the
// default when no UI approver is wired up (e.g. headless tests, PR 2 of the
// agent rollout where the popup component does not exist yet).
type AutoAllowApprover struct{}

// Approve always returns Allow=true. ctx cancellation is honored.
func (AutoAllowApprover) Approve(ctx context.Context, _ *ApprovalRequest) (ApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return ApprovalDecision{}, err
	}
	return ApprovalDecision{Allow: true}, nil
}

// AgentLoopConfig groups everything the loop needs beyond the provider and
// initial request.
type AgentLoopConfig struct {
	// Registry is the set of tools the model may invoke. Tools not in the
	// registry but mentioned by the model produce a soft error tool message —
	// they do not abort the loop.
	Registry *tools.Registry

	// Approver gates tool invocation. Required.
	Approver Approver

	// MaxIterations bounds the number of provider round-trips. Denials count
	// as iterations to bound retry cost when the user keeps denying.
	MaxIterations int

	// PerCallTimeout caps an individual provider streaming call. The overall
	// loop is governed by the caller's ctx with no internal total timeout.
	PerCallTimeout time.Duration

	// Tag identifies the calling flow (e.g. "explain", "chat") in slog
	// records. Optional; logs use "agent" if empty.
	Tag string
}

// RunAgentLoop drives one /explain or /chat invocation: alternating provider
// streams and tool executions until the model produces a turn with no further
// tool calls (or we hit the iteration cap).
//
// Text deltas are forwarded to out as they arrive. Tool-call lifecycle events
// (start, approval, finished) are also emitted on out. The function closes
// out before returning, regardless of outcome.
//
// Cancellation: respects ctx. Each provider call gets its own
// context.WithTimeout(ctx, cfg.PerCallTimeout) so a slow upstream cannot stall
// the whole loop indefinitely.
func RunAgentLoop(
	ctx context.Context,
	provider ai.Provider,
	req ai.ChatRequest,
	cfg AgentLoopConfig,
	out chan<- WtfStreamEvent,
) {
	defer close(out)

	if cfg.Approver == nil {
		out <- WtfStreamEvent{Err: errors.New("agent: approver is required"), Done: true}
		return
	}
	if cfg.MaxIterations <= 0 {
		cfg.MaxIterations = 1
	}
	if cfg.PerCallTimeout <= 0 {
		cfg.PerCallTimeout = 60 * time.Second
	}
	tag := cfg.Tag
	if tag == "" {
		tag = "agent"
	}

	// If the provider can't do tools, behave exactly as today: one streaming
	// call, forward deltas, done. Don't bother with the loop bookkeeping.
	caps := provider.Capabilities()
	if !caps.Tools {
		req.Tools = nil
	}

	totalToolCalls := 0
	for iter := 0; iter < cfg.MaxIterations; iter++ {
		if err := ctx.Err(); err != nil {
			out <- WtfStreamEvent{Err: err, Done: true}
			return
		}

		slog.Info("agent_iteration_start",
			"tag", tag,
			"iter", iter,
			"message_count", len(req.Messages),
			"has_tools", len(req.Tools) > 0,
		)

		callCtx, cancel := context.WithTimeout(ctx, cfg.PerCallTimeout)
		stream, err := provider.CreateChatCompletionStream(callCtx, req)
		if err != nil {
			cancel()
			slog.Error("agent_stream_open_error", "tag", tag, "iter", iter, "error", err)
			out <- WtfStreamEvent{Err: err, Done: true}
			return
		}

		assistantText, drainErr := drainStreamText(stream, out)
		toolCalls := stream.ToolCalls()
		stopReason := stream.StopReason()
		stream.Close()
		cancel()

		if drainErr != nil {
			slog.Error("agent_stream_error", "tag", tag, "iter", iter, "error", drainErr)
			out <- WtfStreamEvent{Err: drainErr, Done: true}
			return
		}

		slog.Debug("agent_iteration_response",
			"tag", tag,
			"iter", iter,
			"text_chars", utf8.RuneCountInString(assistantText),
			"tool_calls", len(toolCalls),
			"stop_reason", stopReason,
		)

		if len(toolCalls) == 0 {
			out <- WtfStreamEvent{Done: true}
			slog.Info("agent_loop_done",
				"tag", tag,
				"iterations", iter+1,
				"total_tool_calls", totalToolCalls,
			)
			return
		}

		// Append the assistant turn (text + tool_calls) so the next iteration
		// has the full conversation context.
		req.Messages = append(req.Messages, ai.Message{
			Role:      "assistant",
			Content:   assistantText,
			ToolCalls: toolCalls,
		})

		// Execute each tool call sequentially. Approval, denial, or unknown
		// tool name all produce a "tool" message so the model sees what
		// happened and can recover on the next iteration.
		for _, tc := range toolCalls {
			totalToolCalls++
			info := &ToolCallInfo{
				ID:       tc.ID,
				Name:     tc.Name,
				ArgsJSON: string(tc.Arguments),
			}
			out <- WtfStreamEvent{ToolCallStart: info}

			toolMsg, finished := executeOneTool(ctx, cfg, tc, info, tag, out)
			req.Messages = append(req.Messages, toolMsg)
			out <- WtfStreamEvent{ToolCallFinished: finished}

			if err := ctx.Err(); err != nil {
				out <- WtfStreamEvent{Err: err, Done: true}
				return
			}
		}
	}

	slog.Warn("agent_loop_max_iterations", "tag", tag, "max", cfg.MaxIterations, "tool_calls", totalToolCalls)
	out <- WtfStreamEvent{Err: ErrMaxIterations, Done: true}
}

// drainStreamText forwards every non-empty text delta to out and returns the
// concatenated assistant text and any stream error. Tool-call deltas are
// already absorbed by the provider's stream wrapper; we only deal with text.
func drainStreamText(stream ai.ChatStream, out chan<- WtfStreamEvent) (string, error) {
	var sb strings.Builder
	for stream.Next() {
		delta := stream.Content()
		if delta == "" {
			continue
		}
		sb.WriteString(delta)
		out <- WtfStreamEvent{Delta: delta}
	}
	return sb.String(), stream.Err()
}

// executeOneTool runs the approve+invoke cycle for a single tool call and
// returns the tool message to append plus the populated ToolCallInfo for the
// finished event.
func executeOneTool(
	ctx context.Context,
	cfg AgentLoopConfig,
	tc ai.ToolCall,
	info *ToolCallInfo,
	tag string,
	out chan<- WtfStreamEvent,
) (ai.Message, *ToolCallInfo) {
	finished := *info // copy header fields

	approval := &ApprovalRequest{
		ID:   tc.ID,
		Name: tc.Name,
		Args: tc.Arguments,
	}
	slog.Debug("tool_approval_request", "tag", tag, "tool", tc.Name, "id", tc.ID)
	out <- WtfStreamEvent{ToolApproval: approval}

	decision, err := cfg.Approver.Approve(ctx, approval)
	if err != nil {
		// Treat approver error as a denial so the model sees a tool message
		// and can continue. The Go error path is reserved for ctx cancel,
		// which the caller will pick up on the next iteration check.
		slog.Warn("tool_approval_error", "tag", tag, "tool", tc.Name, "error", err)
		finished.Denied = true
		finished.ErrorMessage = err.Error()
		return ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    fmt.Sprintf("Tool call could not be approved: %s", err.Error()),
		}, &finished
	}
	if !decision.Allow {
		slog.Info("tool_approval_decision", "tag", tag, "tool", tc.Name, "allow", false)
		finished.Denied = true
		return ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    "User denied this tool call.",
		}, &finished
	}
	slog.Info("tool_approval_decision", "tag", tag, "tool", tc.Name, "allow", true, "persistent", decision.Persistent)

	tool, ok := cfg.Registry.Get(tc.Name)
	if !ok {
		// Hallucinated tool — soft-fail so the model can correct.
		slog.Warn("tool_unknown", "tag", tag, "tool", tc.Name)
		finished.ErrorMessage = "unknown tool"
		return ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    fmt.Sprintf("Unknown tool: %q. Available tools: %s", tc.Name, strings.Join(toolNames(cfg.Registry), ", ")),
		}, &finished
	}

	start := time.Now()
	result, err := tool.Execute(ctx, tc.Arguments)
	finished.Duration = time.Since(start)
	if err != nil {
		// Hard error from the tool aborts the loop via the caller's ctx
		// check, but we still append something so the assistant turn is well
		// formed.
		slog.Error("tool_call_executed", "tag", tag, "tool", tc.Name, "duration_ms", finished.Duration.Milliseconds(), "error", err)
		finished.ErrorMessage = err.Error()
		return ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    fmt.Sprintf("Tool error: %s", err.Error()),
		}, &finished
	}

	finished.Result = result.Content
	logResult := result.Content
	if len(logResult) > 200 {
		logResult = logResult[:200] + "…"
	}
	slog.Info("tool_call_executed",
		"tag", tag,
		"tool", tc.Name,
		"duration_ms", finished.Duration.Milliseconds(),
		"result_bytes", len(result.Content),
		"is_error", result.IsError,
	)

	return ai.Message{
		Role:       "tool",
		ToolCallID: tc.ID,
		Name:       tc.Name,
		Content:    result.Content,
	}, &finished
}

func toolNames(r *tools.Registry) []string {
	defs := r.Definitions()
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return names
}
