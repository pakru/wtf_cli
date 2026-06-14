# Improve Behavior on Multiple Tool-Request Streaks (Issue #74)

## Problem

When the model chains tool calls, the agent loop ([pkg/commands/agent_loop.go](../../pkg/commands/agent_loop.go)) keeps iterating until it hits `MaxIterations` (config default **5**, [pkg/config/config.go:118](../../pkg/config/config.go#L118)). On the iteration after the cap it aborts with a hard error:

```go
slog.Warn("agent_loop_max_iterations", ...)
out <- WtfStreamEvent{Err: ErrMaxIterations, Done: true}   // agent_loop.go:219-220
```

The UI renders that raw error in the sidebar ([pkg/ui/stream.go:135-152](../../pkg/ui/stream.go#L135-L152)), discarding any partial answer. Two problems:

1. **The cap (5) is too low** for legitimate multi-step tasks, even though it *is* technically configurable.
2. **Hitting the cap is a dead end** — the user can't say "keep going," and the failure surfaces as a scary error rather than a choice.

The issue asks for two things:

1. Set more reasonable loop limits, and keep them configurable.
2. Show a dialog asking whether to continue the tool-request loop instead of erroring out.

## Design

The loop already has the exact primitive we need: the **approval popup**. A tool approval is a blocking request (`ApprovalRequest`) sent over the `WtfStreamEvent` channel; the loop parks on a `Reply` channel while a modal ([pkg/ui/components/toolapproval/panel.go](../../pkg/ui/components/toolapproval/panel.go)) collects the user's choice. We mirror that pattern for a **continuation prompt**.

### One configurable limit + a continue dialog

| Config key | Meaning | New default |
|---|---|---|
| `agent.max_iterations` | Batch size, counted in **tool calls**. After the model has run this many tool calls (whether spread across turns or batched into one) the loop *pauses and asks* "continue?". No longer a hard error. | `100` (was 5) |

There is **no hard ceiling** — the user is the stop control. Reaching `max_iterations` tool calls shows the continuation dialog; **Continue** grants another batch of `max_iterations`, **Stop** ends the loop gracefully. The partial answer is always preserved, and the loop also still stops the moment the model returns a turn with no tool calls.

> **Why tool calls, not round-trips:** a model can return several tool calls in a single turn (one provider round-trip). Counting round-trips would let, say, 3 `read_file` calls in one turn slip past a limit of 2. Counting tool calls matches the user's mental model — "ask me after N tool calls."

### Decision flow

```
RunAgentLoop:
  toolCallsThisBatch = 0
  for iter := 0; ; iter++:
    if toolCallsThisBatch >= maxIterations:
        cont := continuer.Continue(ctx, req)           # popup or auto-stop
        if !cont: emit Done; return                    # graceful stop
        toolCallsThisBatch = 0                         # granted another batch
    stream := provider.CreateChatCompletionStream(req)
    forward deltas; toolCalls := stream.ToolCalls()
    if toolCalls empty: emit Done; return
    append assistant turn; execute tools (existing path)
    toolCallsThisBatch += len(toolCalls)
```

A turn's tool calls always run together; the prompt gates the *next* provider call, so the user is asked once the batch is full, before more money is spent.

### Continuer abstraction (mirrors Approver)

The continuation prompt is interactive, so — exactly like `Approver`/`UIApprover` — the loop talks to a `Continuer` interface. The UI injects a popup-driven implementation; headless/test flows use an auto-stop default.

```go
type ContinuationRequest struct {
    ToolCalls  int            // total tool calls so far (for the message)
    Iterations int            // round-trips so far
    Reply      chan ContinuationDecision
}

type ContinuationDecision struct {
    Continue bool
}

type Continuer interface {
    Continue(ctx context.Context, req *ContinuationRequest) (ContinuationDecision, error)
}

// AutoStopContinuer is the default when no UI continuer is wired (headless,
// tests). It stops gracefully at the batch cap rather than erroring.
type AutoStopContinuer struct{}
```

This keeps `RunAgentLoop` provider/UI-agnostic and the new behavior fully unit-testable, consistent with how `Approver` was introduced in [issue-58](issue-58-agentic-loop.md).

---

## Implementation Phases

### Phase 1 — Config (`pkg/config/config.go`)

- Constants (~L116-121): bump `defaultAgentMaxIterations = 5` → `100`.
- No new field or presence/merge changes — `agent.max_iterations` already exists and is wired through `applyAgentDefaults` (L517-519). Only the default value changes.

### Phase 2 — Loop core (`pkg/commands/agent_loop.go`)

- Add `Continuer` interface, `ContinuationRequest`, `ContinuationDecision`, `AutoStopContinuer` (alongside the `Approver` types, L21-63).
- `AgentLoopConfig` (L67-87): add `Continuer Continuer`.
- In `RunAgentLoop` (L100-221):
  - Default `cfg.Continuer = AutoStopContinuer{}` when nil (next to the `Approver` nil-check at L109).
  - Replace the bounded `for iter := 0; iter < cfg.MaxIterations; iter++` (L132) with an unbounded loop carrying `toolCallsThisBatch` (advanced by `len(toolCalls)` each turn), implementing the decision flow above.
  - At the batch checkpoint, call `cfg.Continuer.Continue`; on `false`/error emit `Done` (graceful) and return — never an error event.
  - Delete the terminal `out <- WtfStreamEvent{Err: ErrMaxIterations, Done: true}` (L219-220); the loop now only exits on no-tool-calls, a "stop" decision, or ctx cancel.
- `ErrMaxIterations` is no longer sent to the UI. Delete the sentinel and update references — a grep shows it is only referenced here and in the test.

### Phase 3 — UI Continuer bridge (`pkg/commands/ui_continuer.go`, new)

Copy the shape of [pkg/commands/ui_approver.go](../../pkg/commands/ui_approver.go):

```go
type UIContinuer struct{ out chan<- WtfStreamEvent }

func (c *UIContinuer) Continue(ctx context.Context, req *ContinuationRequest) (ContinuationDecision, error) {
    if req.Reply == nil { req.Reply = make(chan ContinuationDecision, 1) }
    select {
    case c.out <- WtfStreamEvent{ContinuePrompt: req}:
    case <-ctx.Done(): return ContinuationDecision{}, ctx.Err()
    }
    select {
    case d := <-req.Reply: return d, nil
    case <-ctx.Done(): return ContinuationDecision{}, ctx.Err()
    }
}
```

Same buffered-channel / `ctx.Done()` contract as `UIApprover` (avoids the deadlock documented at [ui_approver.go:56-68](../../pkg/commands/ui_approver.go#L56-L68)). The `ctx.Done()` branch is defensive only — today nothing cancels `loopCtx` from the UI (it derives from `context.Background()`), so in practice the user answers via the modal and the decision returns over `Reply`. The continue feature does **not** depend on ctx cancellation: a "stop" decision is a normal reply that makes the loop emit a graceful `Done`.

### Phase 4 — Stream event + handler wiring

- **`pkg/commands/handlers.go`**
  - `WtfStreamEvent` (L58-68): add `ContinuePrompt *ContinuationRequest`. Update the field-order doc comment (L52-57).
  - Add `ContinuerFactory func(out chan<- WtfStreamEvent) Continuer` to `ExplainHandler` (and the equivalent on `ChatHandler`), mirroring `ApproverFactory` (L17-31).
  - `agentRunPrep` / `prepareAgentRun` (L183-226): `maxIterations` already flows through — no change needed there.
  - In `StartStream` (L155-167) add a `resolveContinuer(ch)` helper (fallback `AutoStopContinuer{}`) and pass `Continuer` into `AgentLoopConfig`.
- **`pkg/commands/chat_handler.go`** (L93-99): same `Continuer` wiring.

### Phase 5 — Continue-prompt modal (`pkg/ui/components/continueprompt/panel.go`, new)

A two-option confirm modal styled with the same `styles.*` primitives as the approval panel:

- `Panel` with `Show(req *commands.ContinuationRequest)`, `Hide()`, `IsVisible()`, `SetSize()`, `Update(tea.KeyPressMsg) tea.Cmd`, `View() string`.
- Buttons: **1. Continue** / **2. Stop** (`enter` confirms cursor, `esc`/`n` ⇒ Stop for safety, `y` ⇒ Continue).
- Body text: "The assistant has made {ToolCalls} tool calls. Continue?" using `req.ToolCalls`/`req.Iterations`.
- `DecisionMsg{ Request *commands.ContinuationRequest; Continue bool }`.

(Reuse vs. new component: the existing `toolapproval.Panel` is specialized for 3 options + tool-arg rendering; a small sibling component is cleaner than overloading it. Shared rendering helpers like `renderApprovalHeader`/`renderApprovalHelp` can be factored out if duplication bothers us, but that's optional.)

### Phase 6 — Model integration (`pkg/ui/`)

- **`model.go`**: add `continuePrompt *continueprompt.Panel` field (next to `toolApproval`, L53); init in the constructor (L145); set `ExplainHandler.ContinuerFactory` / `ChatHandler.ContinuerFactory` wherever `ApproverFactory` is set; add the `case continueprompt.DecisionMsg` dispatch (next to `toolapproval.DecisionMsg`, L315-316).
- **`stream.go`**:
  - In `handleWtfStreamEvent` (L135), add a `msg.ContinuePrompt != nil` branch (before/after the `ToolApproval` branch at L158) that shows the panel and re-arms the listener with `m.continueStreamListen()`.
  - Add `handleContinuePromptDecision` mirroring `handleToolApprovalDecision` (L75-110): hide panel, non-blocking send `ContinuationDecision{Continue: ...}` on `msg.Request.Reply`.
- **`update_input.go`**: route keys/pastes to the continue panel when visible, at the same modal priority as `toolApproval` (L68-71 paste, L147-153 keys).
- **`view.go`**: render `continuePrompt.View()` as a topmost overlay; give it a layer at/above `toolApprovalLayer` (L61, L127-128). Only one of the two modals can be visible at a time.

### Phase 7 — Tests & docs

- **`pkg/commands/agent_loop_test.go`**
  - Rewrite `TestRunAgentLoop_MaxIterationsErr` (L338-372) → `TestRunAgentLoop_BatchCapStops`: with `AutoStopContinuer`, assert a graceful `Done` (no `ErrMaxIterations`) and exactly `MaxIterations` provider calls.
  - Add `TestRunAgentLoop_ContinuerGrantsAnotherBatch`: a fake continuer returning `Continue:true` once then `false`; assert provider calls exceed `MaxIterations` and the loop stops gracefully.
- **`pkg/config/config_test.go`**: update the expected `max_iterations` default (5 → 100).
- Add `pkg/commands/ui_continuer_test.go` mirroring any `UIApprover` test (ctx-cancel + reply paths).
- Add `pkg/ui/components/continueprompt/panel_test.go` for key handling / decision emission.
- **Docs**: this file; update the config block in [issue-58-agentic-loop.md](issue-58-agentic-loop.md) and the README/AGENTS config reference for the new `max_iterations` default and the continue-dialog behavior.

## Verification Plan

- `go test ./...` (new + updated unit tests above).
- Manual: set `agent.max_iterations: 2`, run `/explain` against output that forces repeated `read_file` calls → continuation dialog appears after 2 tool calls (including when both are batched into one turn); **Continue** grants another batch (dialog reappears after 2 more); **Stop** (and **Esc**) keep the partial answer with no error line.
- Note: **Stop/Esc end the loop via the `Reply` channel** (a graceful `Done`), *not* via ctx cancellation. `Ctrl+C` does **not** abort a running loop today — it writes ETX to the PTY ([input.go:135-137](../../pkg/ui/input/input.go#L135-L137)), and while a modal is up the modal absorbs the key. That pre-existing gap is out of scope here (see below).

## Out of Scope

- **Real loop cancellation from the UI** (`Ctrl+C` / sidebar-close aborting a running agent loop). This is a pre-existing gap — `loopCtx` derives from `context.Background()` and its `cancel` is never stored on the Model, so the `ctx.Done()` branches in `UIApprover` (and the new `UIContinuer`) never fire from user action. The continue dialog's **Stop** button is the intended escape hatch for the streak problem and works without it. Wiring true cancellation (store `cancel` on the Model; trigger on `Ctrl+C` and sidebar-close) is a worthwhile separate follow-up.
- Surfacing the new limits in the `/settings` panel (agent config is file-only today).
- A "continue and don't ask again this run" persistent option — easy to add later via `ContinuationDecision.Persistent`, mirroring `ApprovalDecision.Persistent`.
