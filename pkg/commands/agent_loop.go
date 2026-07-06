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

// ApprovalDecision is the user's answer to an ApprovalRequest.
type ApprovalDecision struct {
	// Allow is true when the call should proceed.
	Allow bool

	// Persistent indicates the user chose to remember this decision for the
	// rest of the session. Its scope is contextual: for an ordinary request
	// (Escape == nil) it means "always allow this tool by name" — the
	// approver remembers this and auto-allows subsequent calls to the same
	// tool, which do NOT round-trip through Approve. For an escape request
	// (Escape != nil) it means "always allow this tool to access this
	// directory" — recorded per (tool, directory), never bleeding into the
	// tool-name grant or to other tools.
	Persistent bool

	// AllowOutsideWorkdir is set only when this decision approves a call
	// that ClassifyCall identified as targeting a path outside the working
	// directory. The agent loop passes an ExecGrant to Execute only when
	// this is true; it is never set as a side effect of an ordinary
	// Allow=true decision, so a headless or session-only approval can never
	// silently unlock filesystem access beyond the working directory.
	AllowOutsideWorkdir bool
}

// ApprovalRequest is sent by the agent loop when it wants to invoke a tool
// that needs user permission. The loop blocks on Reply until the approver
// dispatches a decision (or the context is canceled).
type ApprovalRequest struct {
	ID    string
	Name  string
	Args  json.RawMessage
	Reply chan ApprovalDecision

	// Escape is non-nil when the tool implements tools.EscapeClassifier and
	// classified this call as targeting a path outside the working
	// directory. Approvers use it to decide policy (e.g. checking a
	// per-tool directory grant store) and to show the user what is being
	// approved; nil means this is an ordinary in-workdir (or classification-
	// declined) call.
	Escape *tools.EscapeRequest
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
//
// It never sets AllowOutsideWorkdir, so a headless run can never silently
// grant out-of-workdir filesystem access: an escape-classified call is still
// "Allow"ed here (the tool name / no-UI policy is unconditional), but the
// loop's grant-building step requires AllowOutsideWorkdir=true before it will
// hand Execute anything beyond workdir containment, and Execute enforces
// containment on its own regardless of what the approver said.
type AutoAllowApprover struct{}

// Approve always returns Allow=true. ctx cancellation is honored.
func (AutoAllowApprover) Approve(ctx context.Context, _ *ApprovalRequest) (ApprovalDecision, error) {
	if err := ctx.Err(); err != nil {
		return ApprovalDecision{}, err
	}
	return ApprovalDecision{Allow: true}, nil
}

// ContinuationDecision is the user's answer to a ContinuationRequest.
type ContinuationDecision struct {
	// Continue is true when the loop should run another batch of tool-calling
	// round-trips.
	Continue bool
}

// ContinuationRequest is sent by the agent loop when it reaches the per-batch
// iteration limit (AgentLoopConfig.MaxIterations) and the model still wants to
// call tools. The loop blocks on Reply until the continuer dispatches a
// decision (or the context is canceled).
type ContinuationRequest struct {
	// ToolCalls is the total number of tool calls executed so far in this
	// invocation — surfaced to the user so they can judge the streak length.
	ToolCalls int
	// Iterations is the number of provider round-trips so far.
	Iterations int
	Reply      chan ContinuationDecision
}

// Continuer decides whether the agent loop should keep going past the
// per-batch iteration limit.
//
// Like Approver, implementations may run in any goroutine; the loop waits on
// Continue synchronously, so blocking implementations (e.g. a UI popup) are
// fine. Continue must respect ctx.Done().
type Continuer interface {
	Continue(ctx context.Context, req *ContinuationRequest) (ContinuationDecision, error)
}

// AutoStopContinuer stops the loop at the per-batch limit without prompting.
// Used as the default when no UI continuer is wired up (headless flows,
// tests). It produces a graceful stop, not an error.
type AutoStopContinuer struct{}

// Continue always returns Continue=false. ctx cancellation is honored.
func (AutoStopContinuer) Continue(ctx context.Context, _ *ContinuationRequest) (ContinuationDecision, error) {
	if err := ctx.Err(); err != nil {
		return ContinuationDecision{}, err
	}
	return ContinuationDecision{Continue: false}, nil
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

	// Continuer decides whether to run another batch when the loop reaches
	// MaxIterations consecutive tool-calling round-trips. Optional; defaults to
	// AutoStopContinuer (graceful stop at the limit) when nil.
	Continuer Continuer

	// MaxIterations is the per-batch limit on the number of tool calls the
	// model may run before the loop pauses. When the running total of tool
	// calls since the last "continue" reaches it and the model still wants
	// more tools, Continuer decides whether to grant another batch. Tool calls
	// batched into a single turn all count, so a turn that returns N tool
	// calls advances the counter by N.
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
	if cfg.Continuer == nil {
		cfg.Continuer = AutoStopContinuer{}
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
	toolCallsThisBatch := 0
	for iter := 0; ; iter++ {
		if err := ctx.Err(); err != nil {
			out <- WtfStreamEvent{Err: err, Done: true}
			return
		}

		// Batch checkpoint: once the model has run a full batch of tool calls
		// (MaxIterations of them, whether spread across turns or batched into
		// one) and still wants more, ask the user whether to continue before
		// opening another provider call. A "stop" decision (or an error, e.g.
		// ctx cancel) ends the loop gracefully — never a hard error event.
		if toolCallsThisBatch >= cfg.MaxIterations {
			slog.Info("agent_continuation_prompt",
				"tag", tag,
				"iter", iter,
				"tool_calls_this_batch", toolCallsThisBatch,
				"total_tool_calls", totalToolCalls,
			)
			decision, err := cfg.Continuer.Continue(ctx, &ContinuationRequest{
				ToolCalls:  totalToolCalls,
				Iterations: iter,
			})
			if err != nil || !decision.Continue {
				slog.Info("agent_continuation_stop",
					"tag", tag,
					"iter", iter,
					"total_tool_calls", totalToolCalls,
					"error", err,
				)
				out <- WtfStreamEvent{Done: true}
				return
			}
			toolCallsThisBatch = 0
		}

		slog.Info("agent_iteration_start",
			"tag", tag,
			"iter", iter,
			"message_count", len(req.Messages),
			"tool_count", len(req.Tools),
		)

		callCtx, cancel := context.WithTimeout(ctx, cfg.PerCallTimeout)
		stream, err := provider.CreateChatCompletionStream(callCtx, req)
		if err != nil {
			cancel()
			slog.Error("agent_stream_open_error", "tag", tag, "iter", iter, "error", err)
			out <- WtfStreamEvent{Err: err, Done: true}
			return
		}

		assistantText, drainErr := drainStreamText(ctx, stream, out)
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

			toolMsg, finished, execErr := executeOneTool(ctx, cfg, tc, info, tag)
			out <- WtfStreamEvent{ToolCallFinished: finished}
			if execErr != nil {
				out <- WtfStreamEvent{Err: execErr, Done: true}
				return
			}
			req.Messages = append(req.Messages, toolMsg)

			if err := ctx.Err(); err != nil {
				out <- WtfStreamEvent{Err: err, Done: true}
				return
			}
		}

		toolCallsThisBatch += len(toolCalls)
	}
}

// drainStreamText forwards every non-empty text delta to out and returns the
// concatenated assistant text and any stream error. Tool-call deltas are
// already absorbed by the provider's stream wrapper; we only deal with text.
func drainStreamText(ctx context.Context, stream ai.ChatStream, out chan<- WtfStreamEvent) (string, error) {
	var sb strings.Builder
	for stream.Next() {
		if err := ctx.Err(); err != nil {
			return sb.String(), err
		}
		delta := stream.Content()
		if delta == "" {
			continue
		}
		sb.WriteString(delta)
		select {
		case out <- WtfStreamEvent{Delta: delta}:
		case <-ctx.Done():
			return sb.String(), ctx.Err()
		}
	}
	return sb.String(), stream.Err()
}

// executeOneTool runs the classify+approve+invoke cycle for a single tool
// call. Returns the tool message to append, the populated ToolCallInfo for
// the finished event, and a non-nil error only when the loop must abort
// (hard tool error or context cancellation). Soft failures (denial, unknown
// tool, Result.IsError) return a nil error — the model message carries the
// failure.
func executeOneTool(
	ctx context.Context,
	cfg AgentLoopConfig,
	tc ai.ToolCall,
	info *ToolCallInfo,
	tag string,
) (ai.Message, *ToolCallInfo, error) {
	finished := *info // copy header fields

	// Registry lookup happens before approval: a hallucinated tool name
	// soft-fails immediately rather than prompting the user for a tool that
	// doesn't exist.
	tool, ok := cfg.Registry.Get(tc.Name)
	if !ok {
		slog.Warn("tool_unknown", "tag", tag, "tool", tc.Name)
		finished.ErrorMessage = "unknown tool"
		return ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    fmt.Sprintf("Unknown tool: %q. Available tools: %s", tc.Name, strings.Join(toolNames(cfg.Registry), ", ")),
			IsError:    true,
		}, &finished, nil
	}

	// If the tool supports user-approved out-of-workdir access, classify the
	// call before requesting approval so the popup can show the resolved
	// path and the approver can check a directory grant. A nil result means
	// "no escape" — the ordinary in-workdir approval/containment applies;
	// classification is best-effort UX, never an enforcement decision.
	var escape *tools.EscapeRequest
	if classifier, ok := tool.(tools.EscapeClassifier); ok {
		escape = classifier.ClassifyCall(tc.Arguments)
	}

	approval := &ApprovalRequest{
		ID:     tc.ID,
		Name:   tc.Name,
		Args:   tc.Arguments,
		Escape: escape,
	}
	slog.Debug("tool_approval_request", "tag", tag, "tool", tc.Name, "id", tc.ID, "outside_workdir", escape != nil)

	// The loop does NOT emit a WtfStreamEvent{ToolApproval:...} itself — that
	// would fire even when the approver auto-allows (no popup needed). Each
	// approver decides whether to surface a UI event before answering.
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
			IsError:    true,
		}, &finished, nil
	}
	// Audit log: the authoritative record of every decision — allow or deny,
	// and including ones a session/path-grant store auto-allowed without
	// ever showing a popup. Built once so the deny and allow paths can never
	// drift out of sync on which escape fields get logged.
	logArgs := []any{
		"tag", tag, "tool", tc.Name, "allow", decision.Allow, "persistent", decision.Persistent,
		"outside_workdir", escape != nil,
	}
	if escape != nil {
		logArgs = append(logArgs,
			"resolved_path", escape.ResolvedPath,
			"grant_dir", escape.GrantDir,
			"allow_outside_workdir", decision.AllowOutsideWorkdir,
		)
	}
	slog.Info("tool_approval_decision", logArgs...)

	if !decision.Allow {
		finished.Denied = true
		return ai.Message{
			Role:       "tool",
			ToolCallID: tc.ID,
			Name:       tc.Name,
			Content:    "User denied this tool call.",
			IsError:    true,
		}, &finished, nil
	}

	// The grant only ever carries an escape when classification offered one
	// AND the decision explicitly approved crossing the working-directory
	// boundary. Every other combination (no escape offered, escape offered
	// but AllowOutsideWorkdir unset — e.g. AutoAllowApprover) yields the
	// zero grant, which every tool's Execute treats as workdir-only.
	var grant tools.ExecGrant
	if escape != nil && decision.AllowOutsideWorkdir {
		grant = tools.ExecGrant{ApprovedPath: escape.ResolvedPath, Target: escape.Target}
	}

	start := time.Now()
	result, err := tool.Execute(ctx, tc.Arguments, grant)
	finished.Duration = time.Since(start)
	if err != nil {
		slog.Error("tool_call_executed", "tag", tag, "tool", tc.Name, "duration_ms", finished.Duration.Milliseconds(), "error", err)
		finished.ErrorMessage = err.Error()
		// Context cancellation and all other hard errors abort the loop. The
		// caller surfaces the error on out and returns; no tool message is
		// appended because Tool.Execute guarantees non-nil error only for
		// loop-aborting conditions (see tools.Tool contract).
		return ai.Message{}, &finished, err
	}

	finished.Result = result.Content
	if result.IsError {
		finished.ErrorMessage = result.Content
	}
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
		IsError:    result.IsError,
	}, &finished, nil
}

func toolNames(r *tools.Registry) []string {
	defs := r.Definitions()
	names := make([]string, 0, len(defs))
	for _, d := range defs {
		names = append(names, d.Name)
	}
	return names
}
