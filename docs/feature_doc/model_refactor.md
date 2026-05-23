# `model.go` Decomposition Refactor

## Overview

`pkg/ui/model.go` has grown to **2 223 lines** and mixes six distinct concerns in one file. This makes the Bubble Tea model difficult to navigate, review, and extend. The goal is to split it into focused sibling files — all sharing `package ui` — so no public API changes, no interface churn, and no test rewriting.

The existing split pattern (`pty_batch.go`, `mouse_filter.go`) establishes the approach: methods on `Model` and package-level helpers live in separate files within the same package.

**Primary goal:** split `model.go` into focused same-package files and extract the large `Update` cases into small handler methods.

**Non-goals:**
- No new packages.
- No new interfaces for overlays, PTY, or streaming in the first pass.
- No new exported symbols (`NewModel`, `Init`, `Update`, `View`, `Render` are the only ones that should remain exported, as today).
- No behavior change. Golden tests must not move.

---

## Design Direction

- Keep all refactored files in `package ui`.
- Preserve the public API: `NewModel`, `Model.Init`, `Model.Update`, `Model.View`, `Model.Render`.
- Preserve unexported helper names where tests already use them.
- **Mechanical first, behavior cleanups last.** Phases 1–3 only move code and extract handlers. All consolidation/refactoring of duplicated logic lands in Phase 4. This is the single biggest discipline for bisectability: any failure in 1–3 is a move bug; any failure in 4 is a behavior change.
- **Handler signature convention** — every extracted `Update` case handler uses a **value receiver** and returns `(Model, tea.Cmd)` to match `func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd)`:

  ```go
  func (m Model) handlePaste(msg tea.PasteMsg) (Model, tea.Cmd)
  ```

  Use a pointer receiver only for mutation helpers that the handler itself calls (e.g. `applyLayout`, `setScrollMode`). This mirrors the existing convention in `model.go`.
- Co-locate `tea.Cmd` factories with their consumer file (e.g. `fetchOpenAIModelsCmd` next to the settings handler that consumes its `providerModelsRefreshMsg`). Do **not** centralize all cmds in one bucket — cohesion beats categorization.
- Verify with `make check` after each phase.

---

## Problem Analysis

The 2 223 lines break down into six logical concerns:

| Concern | Approx lines | Notes |
|---|---|---|
| Model struct + constructor + Bubble Tea lifecycle (`Init`, `View`, `Render`) | ~280 | Already correct home in `model.go` |
| `Update()` dispatch switch (~40 message types) | **~1 150** | Dominant problem — single switch taller than the rest of the file combined |
| Layout, focus, fullscreen, scroll mode, overlay-block checks | ~200 | Spatial/geometric concerns |
| `renderCanvas` + lipgloss layer composition | ~110 | Rendering concern separate from routing |
| `tea.Cmd` factories, msg types (PTY, stream, providers, update-check, copilot) | ~280 | External I/O initiators |
| Config helpers, format functions, paste utilities | ~200 | Pure utility — no `Model` dependency |

---

## Target File Layout

All files in `pkg/ui/`, `package ui`. Cmd factories are co-located with their consumer file.

**Size budgets are guards, not goals.** The useful target is not an exact line count; it is that no file becomes a new dumping ground and `model.go` stops hiding the state machine.

| File | What lives here | Target range |
|---|---|---|
| `model.go` | `Model` struct, `NewModel`, `chatHandler`, `installApproverFactories`, `Init`, the `Update` dispatcher (thin switch only), `exitConfirmTimeoutMsg` | 250–350 |
| `render.go` | `View`, `Render`, `renderCanvas`, `addOverlayLayer` | 120–180 |
| `update_layout.go` | `handleWindowSize`, `handleResizeApply`, `resizeApplyMsg`, `applyLayout`, `splitSidebarWidths`, `resizePTYViewport` | 200–280 |
| `update_input.go` | `handleKeyPress`, `handlePaste`, `handleTerminalScrollKey`, `routeKeyToVisibleOverlay`, `applyPasteToOverlay` | 250–350 |
| `update_mouse.go` | `handleMouseWheel`, `handleMouseClick`, `handleMouseMotion`, `handleMouseRelease`, `copySelectedText`, `clearTextSelections`, `focusSidebarInputFromMouse`, `focusTerminalFromMouse`, `clearStatusMsgMsg` | 180–260 |
| `update_commands.go` | Palette + history picker + sidebar exec + command-submitted handlers | 200–300 |
| `update_settings.go` | Settings/copilot/model-picker/option-picker handlers, `providerModelsRefreshMsg`, `refreshModelCacheCmd`, `fetchOpenAIModelsCmd`, `fetchAnthropicModelsCmd`, `fetchGoogleModelsCmd`, `fetchCopilotModelsCmd`, `copilotAuthStatusMsg`, `fetchCopilotAuthStatusCmd` | 300–450 |
| `stream.go` | `WtfStreamEvent`/`ChatSubmitMsg`/`streamStartResultMsg`/`streamThrottleFlushMsg` handlers, `listenToWtfStream`, `startExplainStreamCmd`, `startChatStreamCmd`, `buildExplainUserMessage`, placeholder helpers, `formatToolCallStart/Suffix`, stream msg types | 280–400 |
| `pty.go` | `ptyOutputMsg`, `ptyErrorMsg`, `listenToPTY`, `appendNormalizedLines`, `captureCommandFromLine` | 80–160 |
| `pty_batch.go` (existing) | `ptyBatchFlushMsg`, `flushPTYBatch` — unchanged | ~60 |
| `fullscreen.go` | `enterFullScreen`, `exitFullScreen`, `hasFutureEnter` | 60–100 |
| `focus.go` | `hasBlockingOverlay`, `setTerminalFocused`, `setScrollMode` | 80–140 |
| `status_update.go` | `tickDirectory`, `directoryUpdateMsg`, `gitBranchMsg`, `resolveGitBranchCmd`, `updateCheckMsg`, `fetchUpdateCheckCmd`, `handleDirectoryUpdate`, `handleGitBranch`, `handleUpdateCheck` | 150–230 |
| `model_config.go` | `getCurrentDir`, `loadProviderAndModelFromConfig`, `getProviderAndModel`, `getModelForProvider`, `formatCopilotAuthStatus` | 100–160 |

**Net result:** `model.go` shrinks from 2 223 → ~280 lines. No other file exceeds ~450.

---

## Documenting Invariants

Some behavior is non-trivial and will be lost if not documented during the move. Capture these as header comments in their owning file — they are checklist items for the refactor, not optional.

### Overlay/input priority (header of `update_input.go`)

The key dispatch priority — easy to break, hard to reverse from code:

1. Fullscreen passthrough
2. Secret input passthrough
3. Exit confirmation cancellation
4. Tool approval modal
5. Pickers / settings / palette / history overlays
6. Result panel
7. Focus switch
8. Sidebar input
9. Terminal scroll keys
10. PTY input handler

### Stream invariants (header of `stream.go`)

- First delta refreshes immediately; follow-up deltas are batched until `streamThrottleFlushMsg`.
- Stream listener is re-armed after every stream event, including tool-approval events.
- Tool approval modal remains topmost overlay.

### Rendering invariants (header of `render.go`)

- Fullscreen mode uses the Bubble Tea alt screen.
- Normal mode enables `tea.MouseModeCellMotion`.
- Golden-test output must not change.

---

## Implementation Plan

### Phase 1 — Rendering and pure-leaf extraction (mechanical)

Lowest risk: these files contain no state-routing logic. **No behavior cleanups in this phase** except deleting confirmed-dead code (cleanup C1 below).

1. **`render.go`** — move `View`, `Render`, `renderCanvas`, `addOverlayLayer`. Add rendering-invariants header comment.
2. **`model_config.go`** — move `getCurrentDir`, `loadProviderAndModelFromConfig`, `getProviderAndModel`, `getModelForProvider`, `formatCopilotAuthStatus`. Apply cleanup C1 (delete `loadModelFromConfig` after `rg loadModelFromConfig pkg/` confirms zero references).
3. **`pty.go`** — move `ptyOutputMsg`, `ptyErrorMsg`, `listenToPTY`, `appendNormalizedLines`, `captureCommandFromLine`. (Keep `ptyBatchFlushMsg` and `flushPTYBatch` in existing `pty_batch.go`.)
4. **`fullscreen.go`** — move `enterFullScreen`, `exitFullScreen`, `hasFutureEnter`.
5. **`status_update.go`** — move `tickDirectory`, directory/git-branch/update-check msg types and their cmd factories. Handlers stay inline in `Update` for this phase.

**Checkpoint:** `make check` must pass.

### Phase 2 — Layout, focus, and stream helpers (mechanical)

Still mechanical. No consolidation yet.

6. **`focus.go`** — move `hasBlockingOverlay`, `setTerminalFocused`, `setScrollMode`.
7. **`update_layout.go`** (helpers only) — move `applyLayout`, `splitSidebarWidths`, `resizePTYViewport`. The `handleWindowSize` and `handleResizeApply` handlers land in this file in Phase 3.
8. **`stream.go`** (helpers + cmd factories only) — move `listenToWtfStream`, `startExplainStreamCmd`, `startChatStreamCmd`, `buildExplainUserMessage`, placeholder helpers, `formatToolCallStart/Suffix`, stream msg types. The `Update` cases land here in Phase 3. Add stream-invariants header comment.

**Checkpoint:** `make check`.

### Phase 3 — `Update()` peel (mechanical handler extraction)

Each step moves one message family out of the `Update` switch into a `handle{Family}` method in its owning file, with `Update` calling it. **No deduplication or consolidation in this phase** — that's Phase 4. Work inside-out, least entangled first. Each step is its own commit with its own `make check`.

9. **stream handlers → `stream.go`** — `handleWtfStreamEvent`, `handleChatSubmit`, `handleStreamStartResult`, `handleStreamThrottleFlush`.
10. **PTY/status handlers → `pty.go` and `status_update.go`** — `handlePTYOutput`, `handlePTYBatchFlush`, `handlePTYError`, `handleDirectoryUpdate`, `handleGitBranch`, `handleUpdateCheck`.
11. **mouse handlers → `update_mouse.go`** — `handleMouseWheel/Click/Motion/Release`, plus `copySelectedText`, `clearTextSelections`, `focusSidebarInputFromMouse`, `focusTerminalFromMouse`, `clearStatusMsgMsg`.
12. **key/paste handlers → `update_input.go`** — `handleKeyPress`, `handlePaste`, `handleTerminalScrollKey`, `routeKeyToVisibleOverlay`, `applyPasteToOverlay`. Add overlay/input priority header comment.
13. **palette/history/sidebar exec → `update_commands.go`** — `handleShowPalette`, `handlePaletteSelect`, `handlePaletteCancel`, `handleShowHistoryPicker`, `handleHistoryPickerSelect`, `handleHistoryPickerCancel`, `handleSidebarCommandExecute`, `handleCommandSubmitted`.
14. **settings/copilot/pickers → `update_settings.go`** — `handleSettingsClose`, `handleSettingsSave`, `handleStartCopilotAuth`, `handleCopilotAuthStatus`, `handleOpenModelPicker`, `handleModelPickerSelect`, `handleOpenOptionPicker`, `handleOptionPickerSelect`, `handleModelPickerRefresh`, `handleProviderModelsRefresh`, plus `providerModelsRefreshMsg`, `refreshModelCacheCmd`, all four `fetch{OpenAI,Anthropic,Google,Copilot}ModelsCmd`, `copilotAuthStatusMsg`, `fetchCopilotAuthStatusCmd`.
15. **layout handlers → `update_layout.go`** — `handleWindowSize`, `handleResizeApply`, `resizeApplyMsg`.

**Checkpoint after each step:** `make check`.

### Phase 4 — Behavior-preserving consolidations

Once the move is done and `model.go` is thin, duplications across the new files become obvious. Each cleanup below is its own commit. **Every commit in this phase is a behavior-preserving consolidation; if anything regresses, this is where to bisect.**

See *Cleanup Catalogue* below for the full list with code shapes and the invariants each consolidation must preserve.

**Checkpoint after each cleanup:** `make check`.

### Phase 5 — Slim `model.go` (final shape check)

16. Confirm `model.go` is ~280 lines of struct + lifecycle + thin dispatcher only. The `Update` switch should now read like a routing index — one case per family, each delegating to its `handleXxx`.

### Phase 6 — Optional state grouping (separate PR)

Only after Phases 1–5 are merged and stable. Bundle the ~30 `Model` fields into embedded sub-state structs to reduce cognitive load:

```go
type Model struct {
    ptyFile *os.File
    cwdFunc func() (string, error)

    components      uiComponents
    commandState    commandState
    sessionState    sessionState
    streamState     streamState
    layoutState     layoutState
    ptyBatchState   ptyBatchState
    fullscreenState fullscreenState
    startupState    startupState
}
```

Recommended first groupings:

- `streamState` — `wtfStream`, `streamPlaceholderActive`, `streamStartPending`, `toolCallNewTurnNeeded`, `streamThrottlePending`, `streamThrottleDelay`.
- `ptyBatchState` — `ptyBatchBuffer`, `ptyBatchTimer`, `ptyBatchMaxSize`, `ptyBatchMaxWait`.
- `layoutState` — `width`, `height`, `ready`, `terminalFocused`, `scrollMode`, `resizeDebounceID`, `resizeTime`, `initialResize`.
- `fullscreenState` — `fullScreenMode`, `fullScreenPanel`, `altScreenState`.

This touches many call sites for little behavior change — keep it in its own PR so review noise is isolated.

---

## Cleanup Catalogue

Numbered C1–C8. C1 happens in Phase 1 (it's just code removal). C2–C8 all happen in Phase 4.

### C1 — Remove unused `loadModelFromConfig` *(Phase 1)*

[model.go:1684–1687](../../pkg/ui/model.go#L1684) — only forwards to `loadProviderAndModelFromConfig`. Verify with `rg loadModelFromConfig pkg/`, then delete. This is pure removal, not consolidation.

### C2 — Consolidate API-key provider fetch commands *(Phase 4)*

[model.go:1755–1809](../../pkg/ui/model.go#L1755) — `fetchOpenAIModelsCmd`, `fetchAnthropicModelsCmd`, `fetchGoogleModelsCmd` are structurally identical: empty `apiKey` → nil, otherwise context-with-timeout + `fetch(ctx, apiKey)` → `providerModelsRefreshMsg`. Consolidate **only these three**:

```go
func fetchAPIKeyProviderModelsCmd(fieldKey, apiKey string, fetch func(context.Context, string) ([]ai.ModelInfo, error)) tea.Cmd {
    if apiKey == "" {
        return nil
    }
    return func() tea.Msg {
        ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
        defer cancel()
        models, err := fetch(ctx, apiKey)
        return providerModelsRefreshMsg{Models: models, FieldKey: fieldKey, Err: err}
    }
}
```

**Leave `fetchCopilotModelsCmd` explicit.** Copilot's auth model is different (OAuth token, not an API key, with its own validity check). Folding it into the helper requires a special-case conditional that defeats the abstraction.

**Invariants to preserve:**
- Empty `apiKey` returns nil (not a no-op cmd).
- 20-second timeout per provider.
- `FieldKey` in the result message matches the field key the picker dispatched.

### C3 — Extract paste-route trace helper *(Phase 4)*

[model.go:445–534](../../pkg/ui/model.go#L445) — the same 4-line logger block appears 7 times in the paste handler. Extract into `update_input.go`:

```go
func tracePasteRoute(target string, n int) {
    logger := slog.Default()
    if logger.Enabled(context.Background(), logging.LevelTrace) {
        logger.Log(context.Background(), logging.LevelTrace, "paste_route", "target", target, "len", n)
    }
}
```

**Invariants to preserve:**
- Trace-level guard (`Enabled` check) — do not unconditionally log; some hot-path tests assume zero allocations when trace is off.
- Each call site's `target` string is unchanged (telemetry depends on these labels).

### C4 — Sidebar show/hide helpers *(Phase 4)*

[model.go:704–721](../../pkg/ui/model.go#L704) (Ctrl+T) and [model.go:773–788](../../pkg/ui/model.go#L773) (`/chat`) duplicate sidebar show/hide + focus + layout. The two call sites have slightly different intent — `/chat` always wants the sidebar visible, Ctrl+T toggles. Extract separate helpers in `focus.go`:

```go
func (m *Model) showSidebar(reason string)
func (m *Model) hideSidebar(reason string)
func (m *Model) toggleSidebar(reason string)  // calls show/hide based on current state
```

**Invariants to preserve:**
- The `reason` string is what `slog` records at debug level; do not change the existing values (`"ctrl_t"`, `"chat_command"`, etc.) — log analyzers may key on them.
- Focus transfer behavior: showing the sidebar focuses its input; hiding returns focus to the terminal.
- `applyLayout` is called exactly once per toggle, after the state mutation.

### C5 — `inSecretMode` predicate *(Phase 4)*

[model.go:452–460](../../pkg/ui/model.go#L452) (paste) and [model.go:552–563](../../pkg/ui/model.go#L552) (key) — same secret-mode check pattern. Extract into `update_input.go`:

```go
func (m Model) inSecretMode() bool
```

**Invariants to preserve:**
- Both nil checks: `m.ptyFile != nil && m.secretDetector != nil` before consulting the detector. Either being nil must return `false`, not panic.
- The detector method called is unchanged; do not "modernize" the check.

### C6 — `continueStreamListen` helper *(Phase 4)*

[model.go:1213–1296](../../pkg/ui/model.go#L1213) — the `WtfStreamEvent` case returns `listenToWtfStream(m.wtfStream)` 6 times. After `handleWtfStreamEvent` is extracted in step 9, collapse the tail:

```go
func (m Model) continueStreamListen() tea.Cmd {
    if m.wtfStream == nil {
        return nil
    }
    return listenToWtfStream(m.wtfStream)
}
```

**Invariants to preserve:**
- Returns nil (not an empty cmd) when `wtfStream == nil`. Bubble Tea distinguishes nil from `tea.Sequence()` for batching.
- Listener re-armed after **every** stream event, including tool-approval events. Test `stream_throttle_test.go` covers this; do not skip its assertion.

### C7 — `resizeComponents` helper *(Phase 4)*

`tea.WindowSizeMsg` (in `handleWindowSize`) and `applyLayout` both compute per-component sizes from `(width, height)`. Extract:

```go
func (m *Model) resizeComponents(width, height int)
```

**Invariants to preserve:**
- The PTY resize is still debounced; only the *component* sizing is consolidated. Do not collapse the debounce into this helper.
- The order of operations (sidebar widths first, then terminal, then result panel) must match `applyLayout`'s current order — some components read each other's widths during resize.

### C8 — `replacePromptCommand` helper *(Phase 4)*

History-picker select and sidebar-command-execute both clear the current shell line, send command text to the PTY, and update the input buffer. Extract:

```go
func (m *Model) replacePromptCommand(cmd string)
```

**Invariants to preserve:**
- Keep command sanitization at the **caller** when the source is untrusted (history is trusted; sidebar input is not). Do not add sanitization inside the helper.
- The PTY write sequence (Ctrl+U to clear, then the command bytes, no trailing newline) is unchanged.

---

## Source Line Mapping

These line numbers refer to the **current pre-refactor shape** of `pkg/ui/model.go`. They will drift as soon as Phase 1 starts; use this as a starting map, not a permanent reference. After each commit, the next implementer should `rg` for the moved symbol rather than trusting these ranges.

| What to move | Current location |
|---|---|
| Struct, `NewModel`, `Init`, `View`, `Render` | 1–123, 125–175, 212–220, 1421–1461 |
| `copySelectedText`, `clearTextSelections` | 1399–1419 |
| `tickDirectory`, directoryUpdateMsg and friends | 222–246 |
| Mouse handlers in `Update` | 325–435 |
| Paste handler in `Update` | 437–535 |
| Key handler in `Update` | 537–695 |
| Palette / chat toggle / focus / history / execute msgs | 697–889 |
| Settings / copilot / model picker / option picker | 891–1084 |
| `streamStartResultMsg`, `ResultPanelCloseMsg`, `DecisionMsg`, `ChatSubmitMsg`, `WtfStreamEvent`, `streamThrottleFlushMsg` | 1086–1306 |
| `updateCheckMsg`, `ptyOutputMsg/BatchFlushMsg`, `directoryUpdateMsg`, `gitBranchMsg` | 1308–1397 |
| `enterFullScreen`, `exitFullScreen`, `hasFutureEnter` | 1463–1497 |
| `hasBlockingOverlay`, focus helpers, `setScrollMode`, `resizePTYViewport`, `applyLayout` | 1500–1617 |
| `splitSidebarWidths`, `getCurrentDir` | 1619–1667 |
| `loadProviderAndModelFromConfig`, `getProviderAndModel`, `getModelForProvider` | 1669–1730 |
| Provider model fetch commands | 1732–1809 |
| `renderCanvas`, `resolveGitBranchCmd`, `addOverlayLayer` | 1811–1916 |
| PTY + stream msg types | 1918–1944 |
| `buildExplainUserMessage`, `listenToPTY`, `listenToWtfStream`, stream cmd factories | 1946–2010 |
| Stream placeholder helpers | 2012–2045 |
| `formatToolCallStart/Suffix` | 2047–2071 |
| `appendNormalizedLines`, `captureCommandFromLine` | 2073–2109 |
| `applyPasteToOverlay` | 2111–2126 |
| `updateCheckMsg`, `fetchUpdateCheckCmd` | 2128–2166 |
| Copilot auth helpers + msg | 2168–2223 |

---

## Testing Strategy

After each phase step:

```bash
make check
```

Pay particular attention to existing tests that exercise private UI behavior:

- `pkg/ui/model_test.go`
- `pkg/ui/model_overlay_test.go`
- `pkg/ui/model_golden_test.go`
- `pkg/ui/pty_batch_test.go`
- `pkg/ui/stream_throttle_test.go`
- `pkg/ui/tool_call_display_test.go`

**Do not regenerate golden files.** This refactor must not change rendered output. If a golden test fails, the refactor introduced a behavior change — fix the code, don't update the fixture.

### Manual smoke test (before declaring done)

- Open palette with `/`.
- Open history picker with Ctrl+R.
- Toggle sidebar with Ctrl+T.
- Switch focus with Shift+Tab.
- Paste into terminal, palette, settings input, and sidebar input.
- Enter and exit a fullscreen app (`vim`, `htop`).
- Trigger an AI stream that includes a tool-approval modal; exercise both approve and reject paths.
- Resize the terminal window during an active stream.

---

## Review Checklist

- `model.go` is ≤350 lines: struct + lifecycle + thin dispatcher only.
- `Update` reads as a routing index — one case per family, delegating to `handleXxx`.
- Each new file owns one coherent concern (see Target File Layout).
- No new exported symbols.
- No test file changes beyond imports forced by the move.
- Overlay/input priority preserved (header comment in `update_input.go` matches behavior).
- Fullscreen mode still bypasses normal shortcuts.
- Secret input mode still routes directly to PTY (C5 preserves both nil checks).
- Stream listener re-armed on every event (C6 returns nil only when `wtfStream == nil`).
- Tool approval modal remains topmost overlay.
- PTY resize debounce and prompt-reprint suppression preserved (C7 consolidates components only, not debounce).
- Sidebar `slog` reason strings unchanged (C4).
- Paste-route trace labels unchanged (C3).
- Golden tests pass without updates.
- `make check` passes.

---

## Suggested Commit Breakdown

Phase 1 (mechanical, low risk):

1. `ui: extract rendering helpers into render.go`
2. `ui: extract config helpers into model_config.go (remove unused loadModelFromConfig)`
3. `ui: extract pty helpers into pty.go`
4. `ui: extract fullscreen helpers into fullscreen.go`
5. `ui: extract status-update helpers into status_update.go`

Phase 2 (mechanical, helpers only):

6. `ui: extract focus and layout helpers`
7. `ui: extract stream helpers and cmd factories into stream.go`

Phase 3 (mechanical handler extraction — one per commit):

8. `ui: peel Update — stream handlers`
9. `ui: peel Update — pty/status handlers`
10. `ui: peel Update — mouse handlers`
11. `ui: peel Update — key/paste handlers`
12. `ui: peel Update — palette/history/sidebar handlers`
13. `ui: peel Update — settings and provider handlers`
14. `ui: peel Update — layout handlers; finalize thin dispatcher`

Phase 4 (behavior-preserving consolidations — one per commit, easy to bisect):

15. `ui: consolidate API-key provider fetch cmds (C2)`
16. `ui: extract tracePasteRoute helper (C3)`
17. `ui: extract sidebar show/hide/toggle helpers (C4)`
18. `ui: extract inSecretMode predicate (C5)`
19. `ui: extract continueStreamListen helper (C6)`
20. `ui: extract resizeComponents helper (C7)`
21. `ui: extract replacePromptCommand helper (C8)`

Phase 6 (optional, separate PR):

22. `ui: group Model fields by responsibility`
