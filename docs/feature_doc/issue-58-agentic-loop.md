# Agentic Tool Execution for WTF Analysis (Issue #58)

## Overview

WTF Analysis (`/explain`) and `/chat` today are single-turn: build messages → stream LLM response → render. The model can only reason from the terminal output captured at the moment of invocation. This feature enables the model to *act* — call tools that gather extra information, observe results, and iterate before producing a final answer.

This is the foundation for an agentic loop. The first tool is `read_file`, which lets the model pull a slice of any file under the current working directory into the conversation. Concrete example: `/explain` sees a stack trace pointing at `main.go:142`, decides it needs more context, calls `read_file(path="main.go", start_line=120, end_line=160)`, then explains the bug.

**Goal:** ship a base agentic loop (provider tool-calling support, registry, approval UX, system-prompt update) plus the first tool `read_file`, applied to both `/explain` and `/chat`.

---

## Confirmed Design Decisions

- Native provider tool-calling (OpenAI `tool_calls`, Anthropic `tool_use`, Google `functionCall`). No XML-tag hack.
- Loop applies to **both** `/explain` and `/chat`.
- `read_file` is **CWD-restricted** (symlink-resolved containment check).
- `read_file` takes **line-range** params (`start_line`, `end_line`, 1-indexed) with sane defaults.
- Approval UX: a **modal popup** with three options — *Allow once* / *Allow always this session* / *Deny*. Granularity is **per tool name** (allowing `read_file` once approves all subsequent `read_file` calls regardless of path; path-safety still enforced).
- No master feature flag. Loop is on by default. Per-tool enable/disable lives in config.

---

## Architecture

The cleanest seam: keep the existing `ai.Provider` / `ChatStream` abstraction as the *transport* for one provider call, and put the agent loop *above* it. The existing `WtfStreamEvent` channel remains the boundary between background work and the Bubble Tea UI; it is extended with new variants (tool-call lifecycle + approval request) rather than introducing a parallel channel.

### Flow for a single `/explain` invocation with tools enabled

```
ExplainHandler.StartStream
  └── prepareAgentRun (load config, build provider, build initial messages+tools)
  └── go RunAgentLoop(ctx, provider, req, cfg, ch):
        for iter < MaxIterations:
          stream := provider.CreateChatCompletionStream(req)
          forward text deltas → ch
          on stream end: toolCalls := stream.ToolCalls()
          if toolCalls empty: send Done; return
          append assistant turn (text + tool_calls) to req.Messages
          for each toolCall:
            ch <- ToolCallStart
            decision := approver.Approve(ctx, req)   # popup or session-cached
            if denied: append "User denied" tool message; continue
            result := registry.Execute(toolCall)
            append tool result message
            ch <- ToolCallFinished
        send Done
```

---

## Implementation Phases

### Phase 1 — Core types in `pkg/ai`

**Modify** [pkg/ai/provider.go](../../pkg/ai/provider.go):

- Add `ToolDefinition`, `ToolCall`, `ProviderCapabilities` (moved into [pkg/ai/tools.go](../../pkg/ai/tools.go)).
- Extend `Message` with `ToolCalls []ToolCall`, `ToolCallID string`, `Name string`.
- Extend `ChatRequest` with `Tools []ToolDefinition`, `ToolChoice string` (default `"auto"`).
- Extend `ChatResponse` with `ToolCalls []ToolCall`, `StopReason string`.
- Extend `ChatStream` interface with `ToolCalls() []ToolCall` and `StopReason() string` exposed after `Next()` returns false.
- Extend `Provider` interface with `Capabilities() ProviderCapabilities`.

Backward compatibility: empty `Tools` ⇒ provider behaves as today.

### Phase 2 — Tool registry and `read_file`

**Add** [pkg/ai/tools/registry.go](../../pkg/ai/tools/registry.go), [pkg/ai/tools/read_file.go](../../pkg/ai/tools/read_file.go), and tests.

```go
type Tool interface {
    Name() string
    Definition() ai.ToolDefinition
    Execute(ctx context.Context, args json.RawMessage) (Result, error)
}
type Result struct {
    Content string
    IsError bool   // soft error → returned to LLM as tool message
}
```

`read_file` enforces:
1. JSON-decode args → `{Path, StartLine, EndLine}`. On decode failure return `Result{IsError:true}`.
2. Resolve absolute path: if relative, join against the *shell* cwd snapshotted at loop start.
3. `filepath.EvalSymlinks` on both root and path. Use `filepath.Rel` to confirm the resolved path is inside the resolved root; reject `..`, errors, or absolute-outside-root.
4. Default `start_line=1`, `end_line=start_line+199`. Clamp to file length, hard cap at `MaxLines` and `MaxBytes` (config-driven). Append `[truncated: showed N of M lines]` footer if cut.

Hard error vs soft error: Go `error` only when the loop must abort (ctx canceled). Bad path, missing file, decode failure → `Result{IsError:true}` so the model can recover.

### Phase 3 — Per-provider tool mapping

**Modify** each provider in [pkg/ai/providers/](../../pkg/ai/providers/):

- **OpenAI** / **OpenRouter**: map `req.Tools` → `params.Tools`; handle `role="tool"` messages and assistant messages carrying `ToolCalls`. Stream wrapper accumulates `chunk.Choices[0].Delta.ToolCalls` keyed by index — final chunk has `FinishReason="tool_calls"`. `Capabilities().Tools = true`.
- **Anthropic**: tool definitions go in top-level `tools` field. Tool result messages are user-role with content blocks `{"type":"tool_result","tool_use_id":...,"content":...,"is_error":bool}`. Streaming: accumulate `content_block_start` of type `tool_use` + `input_json_delta` deltas → `ToolCall` at `content_block_stop`. **Requires reshaping `anthropicMessage.Content` from `string` to a polymorphic type.** `Capabilities().Tools = true`.
- **Google**: translate to `*genai.Tool` with `FunctionDeclarations`; tool calls arrive as `genai.FunctionCall` parts; results sent as `genai.FunctionResponse` parts. `Capabilities().Tools = true` after smoke test.
- **Copilot**: API is OpenAI-shaped; same translation as OpenAI. `Capabilities().Tools = true` after smoke test.

**Graceful degradation:** the agent loop checks `provider.Capabilities().Tools` before passing `req.Tools`. Providers that return false fall back to today's single-turn behavior with no code path divergence.

### Phase 4 — The agent loop

**Add** [pkg/commands/agent_loop.go](../../pkg/commands/agent_loop.go).

Extend `WtfStreamEvent` in [pkg/commands/handlers.go](../../pkg/commands/handlers.go):

```go
type WtfStreamEvent struct {
    Delta string
    Done  bool
    Err   error

    // tool-call lifecycle
    ToolCallStart    *ToolCallInfo
    ToolApproval     *ApprovalRequest
    ToolCallFinished *ToolCallInfo
}
```

Public entry point:

```go
func RunAgentLoop(
    ctx context.Context,
    provider ai.Provider,
    req ai.ChatRequest,
    cfg AgentLoopConfig,   // {Registry, Approver, MaxIterations, Cwd}
    out chan<- WtfStreamEvent,
)
```

Key details:

- **Cancellation:** caller-supplied ctx flows through. UI cancels via the existing pattern.
- **Per-iteration vs total timeout:** each `provider.CreateChatCompletionStream` call is wrapped in `context.WithTimeout(ctx, perCallTimeout)`.
- **Denials count toward MaxIterations** to bound total round-trips.
- **Hallucinated tool name:** model calls a tool not in the registry → inject a `Result{IsError:true, Content:"unknown tool: X"}` tool message, do NOT abort.
- **Iteration cap:** when reached, send `Err: ErrMaxIterations` and `Done: true` so UI renders a clear "stopped after N rounds" notice.

**Refactor:** `ExplainHandler.StartStream` and `ChatHandler.StartChatStream` both reduce to: build messages, build request (now including `Tools` and `ToolChoice`), kick off `RunAgentLoop` in a goroutine, return `ch`.

### Phase 5 — Approval popup in Bubble Tea ✅

**Add** [pkg/ui/components/toolapproval/panel.go](../../pkg/ui/components/toolapproval/panel.go) — the modal overlay.
**Add** [pkg/commands/ui_approver.go](../../pkg/commands/ui_approver.go) — `UIApprover` and `SessionApprovals`.
**Modify** [pkg/ui/model.go](../../pkg/ui/model.go) — panel wired as topmost overlay layer.

Approval bridge between agent goroutine and UI thread:

```go
func (a *UIApprover) Approve(ctx context.Context, req *ApprovalRequest) (ApprovalDecision, error) {
    if a.policy != nil && a.policy.IsAllowed(req.Name) {
        return ApprovalDecision{Allow: true, Persistent: true}, nil
    }
    req.Reply = make(chan ApprovalDecision, 1)
    a.out <- WtfStreamEvent{ToolApproval: req}
    select {
    case <-ctx.Done():
        return ApprovalDecision{}, ctx.Err()
    case d := <-req.Reply:
        if d.Allow && d.Persistent && a.policy != nil {
            a.policy.Allow(req.Name)
        }
        return d, nil
    }
}
```

UI sequence:
1. Model receives `WtfStreamEvent{ToolApproval: req}` via the existing `listenToWtfStream` pattern.
2. Model shows popup, **re-arms `listenToWtfStream`** so subsequent events keep flowing.
3. User presses `1`/`y` (once), `2`/`a` (always), `3`/`n`/`Esc` (deny). Panel emits `toolapproval.DecisionMsg`.
4. Model dispatches decision on `req.Reply` (capacity 1, non-blocking), hides popup.

**Concurrency invariants:**
- `ch` is **buffered** (capacity 16). Agent sends `ToolApproval` and blocks on `req.Reply`; unbuffered would deadlock.
- `req.Reply` is **buffered capacity 1**. UI sends exactly once.
- Listener is **always re-armed** after delivering a `ToolApproval` event.
- Agent treats `ctx.Done()` as a way out of the `req.Reply` wait.

### Phase 6 — System prompt enhancement ✅

**Modify** [pkg/ai/context.go](../../pkg/ai/context.go).

`AppendToolInstructions(prompt, tools)` appends to the system prompt when `tools` is non-empty:

> You have access to tools that can gather more information than what is shown in the terminal output. Prefer the terminal output already provided; only call tools when you need content not visible above. When a tool is needed, call it directly — do not ask the user for permission, the harness will. Read narrow ranges (a few hundred lines max) per call.

Both `ExplainHandler.StartStream` and `ChatHandler.StartChatStream` call this before constructing the request.

### Phase 7 — Configuration

**Modify** [pkg/config/config.go](../../pkg/config/config.go).

```go
type AgentConfig struct {
    MaxIterations int        `json:"max_iterations"`
    Tools         AgentTools `json:"tools"`
}
type AgentTools struct {
    ReadFile ReadFileToolConfig `json:"read_file"`
}
type ReadFileToolConfig struct {
    Enabled  bool `json:"enabled"`
    MaxLines int  `json:"max_lines"`
    MaxBytes int  `json:"max_bytes"`
}
```

Defaults: `MaxIterations=5`, `ReadFile.Enabled=true`, `MaxLines=500`, `MaxBytes=65536`.

When building the registry: only register tools whose `Enabled=true`. If the registry is empty, skip passing `Tools` to the provider (saves tokens).

### Phase 8 — Logging

Match existing slog style. Add events:

- `agent_iteration_start` — iter, message_count
- `tool_call_request` — name, args_preview
- `tool_approval_request` / `tool_approval_decision` — name, decision
- `tool_call_executed` — name, duration_ms, result_bytes, is_error
- `agent_loop_done` — iterations, total_tool_calls

### Phase 9 — Tests

- [pkg/ai/tools/read_file_test.go](../../pkg/ai/tools/read_file_test.go): `..` traversal, absolute outside cwd, symlink escape, happy path, partial range, line-cap, byte-cap, missing file, decode failure.
- [pkg/commands/agent_loop_test.go](../../pkg/commands/agent_loop_test.go): fake `ai.Provider` returning canned tool calls. Verify event sequence, message-history accumulation, max-iteration cap, denied tool, hallucinated tool, ctx cancellation.
- Provider-level unit tests for `role=tool` and assistant messages with `ToolCalls`.
- UI test driving popup keypress flow with a fake stream channel.

---

## Recommended PR Slicing

| PR | Phases | Description |
|----|--------|-------------|
| PR 1 | 1, 2 | Foundations — interface extensions + registry + read_file. No behavior change. |
| PR 2 | 3 (OpenAI/OpenRouter), 4, 7, 8 | First provider end-to-end with agent loop. Auto-allow approver (no UI). |
| PR 3 | 5, 6 | **UI approval popup + system-prompt update.** (This branch.) |
| PR 4 | 3 (Anthropic, Google, Copilot) | Remaining providers behind `Capabilities().Tools`. |

---

## Verification

1. `make check` passes.
2. `go test ./pkg/ai/tools/... ./pkg/commands/... ./pkg/ai/providers/... ./pkg/config/...` all green.
3. **Happy path:** start `./wtf_cli` in a directory with a known buggy file. Run something producing a stack trace, then `/explain`. Watch logs for `tool_call_request{name=read_file}`; verify popup appears; click *Allow once*; verify model's final answer references file content.
4. **Approval:** invoke again, click *Allow always this session*; invoke a third time and confirm no popup appears.
5. **Denial:** click *Deny*; confirm model produces a graceful response without file content.
6. **Path safety:** craft a prompt that tries to read `/etc/passwd` or `../../etc/passwd`. Confirm the tool returns `is_error=true`.
7. **Iteration cap:** doctored fake provider that always tool-calls confirms loop stops at `MaxIterations` with a clean error.
8. **Cancellation:** trigger a long loop and press Esc. Confirm the goroutine exits cleanly (use `goleak` in a focused test).

---

## Open Risks

1. **Anthropic content reshape** — `anthropicMessage.Content` must change from `string` to a polymorphic shape. Non-trivial JSON-marshaling change; will break serialization tests until updated.
2. **`/explain` redundant reads** — model may call `read_file` when terminal output already contains the file. Mitigated by system-prompt nudge; expect iteration on prompt phrasing.
3. **Cwd snapshot vs live** — cwd is snapshotted at agent-loop start so a mid-stream `cd` in the shell doesn't change which directory the tool reads from.
4. **Tool-call display in sidebar** — recommend rendering each tool call as a collapsed line ("→ read_file(path=foo.go, lines=120-160) — 41 lines, 1.2 KiB") so the user can see what the agent did.
5. **`developer` role + tools** — [pkg/commands/chat_handler.go](../../pkg/commands/chat_handler.go) uses `role="developer"` for terminal context. Verify `developer`-role messages aren't classified as needing a tool result on the next turn.
