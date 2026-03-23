# Issue #46 Implementation Plan: Move Active LLM Label from Status Bar to Chat Panel Footer

## Issue Summary

GitHub issue [#46](https://github.com/pakru/wtf_cli/issues/46) requests moving the currently selected LLM indicator from the global status bar into the bottom area of the chat sidebar panel.

## Current State (Code Review)

### 1) Status bar currently owns active-model display
- `StatusBarView.Render()` always injects `[llm]: <model>` into right-side status content.
- When there is no temporary status message, it appends `| Press / for commands`.

### 2) Model updates status bar model value when config changes
- `NewModel()` initializes status bar model from config.
- `config.ConfigUpdatedMsg` path updates status bar model through `getModelForProvider()`.

### 3) Chat sidebar already has a bottom footer area
- Sidebar renders a footer row at the bottom when command extraction is active.
- Footer currently shows keyboard hint text only (send/apply + navigation shortcuts).
- This makes the chat footer the best location for contextual LLM info.

## Goals

1. Remove active LLM display from status bar.
2. Show active LLM in chat panel bottom footer.
3. Keep existing chat footer interaction hints (or improve readability while adding model info).
4. Keep behavior correct across provider/model changes in settings.

## Non-Goals

- No provider-selection workflow changes.
- No AI backend request format changes.
- No redesign of global status bar layout beyond removing LLM segment.

## Proposed Design

### A) Status bar simplification

**Files:**
- `pkg/ui/components/statusbar/statusbar_view.go`
- `pkg/ui/components/statusbar/statusbar.go` (legacy/parallel implementation kept in repo)

**Change:**
- Remove `[llm]: ...` segment from `rightContent` composition.
- Keep command hint behavior in status bar (e.g., `Press / for commands`) when no transient message is present.
- Preserve width/truncation alignment logic after right-content shrink.

### B) Sidebar gains active-model footer metadata

**Files:**
- `pkg/ui/components/sidebar/sidebar.go`
- (optional) `pkg/ui/styles/theme.go` if a dedicated style token is needed.

**Change:**
- Add an `activeModel string` field to `Sidebar` state.
- Add `SetActiveModel(model string)` method.
- Extend footer text generation so bottom area includes model indicator, for example:
  - `LLM: gpt-4.1-mini · Enter Send | Up/Down Scroll | Shift+Tab TTY | Ctrl+T Hide`
  - or two-line footer if width is constrained.
- Ensure footer renders even when command list is empty (if we want model visible at all times while sidebar is open).

### C) Wire model updates from top-level UI model

**File:**
- `pkg/ui/model.go`

**Change:**
- On `NewModel()`, initialize sidebar active model from config using existing helper (`loadModelFromConfig()` or `getModelForProvider` equivalent).
- On config update message, call `m.sidebar.SetActiveModel(getModelForProvider(msg.Config))`.
- Keep status bar model updates removed or no-op (depending on cleanup approach).

## UX Decision Points

1. **Footer always shown vs conditional**
   - Recommended: always show footer when sidebar is visible so model is consistently visible.
2. **One-line vs two-line footer**
   - Recommended: one-line with truncation first; switch to two lines only if tests show poor readability on narrow widths.
3. **Label format**
   - Recommended: `LLM: <provider>-<model>` so both provider and selected model are visible.

## Implementation Tasks

### Phase 1: Sidebar model metadata plumbing
- [ ] Add `activeProvider` + `activeModel` fields and setter(s) (or one combined setter) for tests/UI wiring.
- [ ] Integrate model text into footer composition.
- [ ] Ensure safe fallback when provider/model is empty (`unknown-unknown` or equivalent).

### Phase 2: Top-level wiring
- [ ] Set sidebar active provider+model during model initialization.
- [ ] Update sidebar active provider+model on runtime config changes.

### Phase 3: Remove statusbar LLM segment
- [ ] Remove model-specific right-side statusbar rendering.
- [ ] Keep `/` command hint and transient message behavior.

### Phase 4: Tests
- [ ] Update status bar tests to assert LLM label is absent.
- [ ] Add/adjust sidebar tests to verify active model appears in footer.
- [ ] Add behavior tests for config updates propagating to sidebar provider-model text.

## Test Plan

Run full project checks before merge:

```bash
make check
```

Targeted tests for faster iteration:

```bash
go test ./pkg/ui/components/statusbar ./pkg/ui/components/sidebar ./pkg/ui
```

If UI rendering expectations/goldens are impacted:

```bash
go test ./pkg/ui/... -update
```

## Risks and Mitigations

1. **Narrow terminal width clipping footer text**
   - Mitigation: prioritize truncating hints before truncating model name, or use compact separators.
2. **Model indicator stale after settings changes**
   - Mitigation: explicit update on `ConfigUpdatedMsg` and coverage in tests.
3. **Regression in status bar spacing/alignment**
   - Mitigation: keep existing width math and update statusbar unit tests.

## Acceptance Criteria

- Status bar no longer displays `[llm]: ...`.
- Opening chat sidebar always shows active `LLM: <provider>-<model>` in the bottom footer area.
- Changing provider/model in settings updates sidebar model label without restart.
- Existing command footer shortcuts remain visible and usable.
- `make check` passes.
