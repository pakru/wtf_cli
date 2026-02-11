# PTY Normalizer Refactor - Implementation Plan

## Overview
We currently normalize PTY output in multiple places (`model.go` buffer path and `terminal.AppendPTYContent` for viewport), which causes drift and makes it hard to reason about how text reaches the LLM. This plan introduces a dedicated PTY normalizer to produce a single, consistent stream of “clean” lines for buffering/LLM, and an optional display stream for the viewport. The model will no longer implement ad-hoc PTY parsing.

## Goals
- Single source of truth for PTY normalization (CR/LF, backspace, CSI, OSC, tabs, etc.).
- Consistent output between viewport display and buffer/LLM context.
- Easier testing via deterministic fixtures.
- Keep model logic small and focused on orchestration.

## Non-Goals (for now)
- Full terminal emulator behavior.
- Multi-line command reconstruction beyond line-based parsing.
- Shell-specific prompt parsing beyond simple delimiters.

---

## Phase 1: Normalizer Core
- [ ] Create `pkg/ui/terminal/normalizer.go` with a `Normalizer` type.
- [ ] Add stateful parsing for:
  - [ ] CR/LF behavior
  - [ ] Backspace + DEL
  - [ ] CSI cursor-left (`ESC [ n D`)
  - [ ] OSC sequences (`ESC ] ... BEL` / `ESC ] ... ESC \\`)
  - [ ] Tabs -> spaces
- [ ] Define output API:
  - `Append(data []byte) (lines [][]byte)` or
  - `Append(data []byte) (displayDelta string, lines [][]byte)` if we keep display and plain outputs split.
- [ ] Ensure the normalizer is reusable across viewport and buffer.

**Definition of Done**
- Normalizer can process streamed PTY chunks and return clean lines.
- All parsing state (CR pending, CSI/OSC state) is fully encapsulated.

---

## Phase 2: Integrate with Buffer
- [ ] Replace `Model.appendPTYOutput` with normalizer usage.
- [ ] Write normalized lines to `CircularBuffer` only.
- [ ] Ensure prompt line capture (command extraction) consumes normalized lines, not raw output.

**Definition of Done**
- Buffer contains normalized lines without stray OSC or backspace artifacts.
- LLM context uses normalized lines exclusively.

---

## Phase 3: Integrate with Viewport
- [ ] Option A (preferred): Use the same normalizer output as viewport input, then re-apply ANSI for display if needed.
- [ ] Option B: Keep ANSI view rendering but make viewport input come from a single “display” stream emitted by the normalizer.
- [ ] Remove `terminal.AppendPTYContent` if the normalizer fully supersedes it (or keep it as a thin wrapper around the normalizer).

**Definition of Done**
- Viewport display remains correct.
- No duplicate normalization logic in viewport or model.

---

## Phase 4: Command Extraction
- [ ] Move prompt parsing into `pkg/capture/prompt_parser.go` (or similar).
- [ ] Provide a function like `ExtractCommandFromPrompt(line string) string`.
- [ ] Use normalized prompt lines from the buffer to update session history.
- [ ] Add tests for prompt extraction with `$ ` and `# ` delimiters.

**Definition of Done**
- Session history is updated based on normalized prompt lines.
- `last_command` reflects the true executed command even when recalled from shell history.

---

## Phase 5: Tests + Regression Coverage
- [x] Add unit tests for the normalizer (CR/LF, backspace, CSI, OSC, mixed sequences).
- [x] Add integration tests covering:
  - [x] OSC stripping in LLM context
  - [x] Backspace normalization (e.g., `git tashg` -> `git tag`)
  - [x] Prompt line extraction (e.g., `...$ ifconfig`)
- [x] Update any existing tests that rely on old behavior.

**Definition of Done**
- All new tests pass.
- No regressions in existing PTY or UI tests.

---

## Phase 6: Cleanup
- [x] Remove legacy code paths in `model.go` that parse PTY directly.
- [ ] Remove or simplify `terminal.AppendPTYContent` if redundant.
- [x] Update docs to reflect new architecture.

**Definition of Done**
- `model.go` only orchestrates PTY → normalizer → buffer/viewport.
- PTY parsing logic lives in a single module.

---

## Notes / Decisions
- We keep line-by-line history parsing for now.
- Prompt parsing is best-effort and should be conservative.
- If needed later, make prompt delimiter configurable in config.
