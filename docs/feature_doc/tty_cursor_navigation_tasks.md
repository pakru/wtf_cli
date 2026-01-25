# TTY Cursor Navigation (Left/Right) - Task List

## Problem Summary
In normal (non–full-screen) shell mode, pressing the left arrow appears to delete characters (backspace behavior) instead of moving the cursor. Runtime logs show the shell emits **0x08 (BS)** for cursor-left in the line editor, and the current PTY display pipeline treats **BS/DEL as deletion** rather than cursor movement. The cursor overlay also renders only at the end of the line.

## Key Observations (from code review + runtime logs)
- **Input path** (`pkg/ui/input/input.go`): left/right arrows send `ESC [ D/C` in normal mode. Full-screen uses app-mode variants based on `cursorKeysAppMode`. Input is likely fine.
- **Runtime logs**: left arrow triggers `backspace=true` in PTY output (single-byte 0x08), right arrow emits `ESC [ C` (3 bytes). This matches typical readline behavior.
- **Viewport rendering** (`pkg/ui/components/viewport/viewport.go` + `pkg/ui/terminal/pty_normalize.go`): `AppendPTYContent` treats CSI `D` as backspace, which removes characters from the line. This makes cursor-left look like deletion.
- **Cursor overlay** (`pkg/ui/terminal/cursor.go`): cursor tracking exists but `RenderCursorOverlay` always places the cursor at the end of content, ignoring tracked `row/col`.
- **Normalizer for buffer/LLM** (`pkg/ui/terminal/normalizer.go`): also treats CSI `D` as deletion; this can corrupt reconstructed command lines when users edit within a line.

## Goals
- Left/right arrows move the cursor in the prompt line (no deletion).
- Edits after moving left/right overwrite or insert in-place like a real terminal.
- Cursor indicator renders at the correct position.
- Buffer/LLM normalization keeps accurate command reconstruction.

---

## Tasks

### 1) Reproduce & Instrument (Short-lived)
- [x] Add temporary debug logging for `tea.KeyPressMsg` (string, code, mod, text) when left/right/backspace are pressed.
- [x] Log PTY output bytes around left/right presses to confirm the shell emits `ESC [ D/C` (or app-mode `ESC O D/C`).
- [ ] Remove/disable temporary logs once behavior is confirmed and fix is validated.

### 2) Input Path Verification
- [x] Confirm `UpdateTerminalModes` receives `CSI ? 1 h/l` (DECCKM) and `cursorKeysAppMode` reflects shell state in normal mode (covered by unit tests).
- [x] Update `HandleKey` to use cursor sequence selection in normal mode (same `cursorSeq(normal, app)` as full-screen).
- [x] Ensure overlays do not intercept arrow keys when they should be routed to PTY (verified by code path review).

### 3) Replace AppendPTYContent With Line-Aware Renderer
Create a minimal line editor/renderer for normal shell mode.
- [x] Introduce `pkg/ui/terminal/line_renderer.go` (or similar) with:
  - [x] `lines []string` scrollback (or incremental buffer)
  - [x] `cursorRow`, `cursorCol`
  - [x] Parsing for: printable chars, CR/LF, backspace/DEL, CSI `C`/`D` (cursor right/left), and `CSI K` (clear to EOL)
  - [x] Overwrite/insert logic when cursor is not at line end
- [x] Replace `terminal.AppendPTYContent` usage in `pkg/ui/components/viewport/viewport.go` with the new renderer.
- [x] Keep ANSI styling stripping/preservation decisions consistent with current viewport expectations.

### 4) Cursor Rendering at True Position
- [x] Update `pkg/ui/terminal/cursor.go` to render cursor at `row/col` instead of always appending to the end.
- [x] Ensure cursor position is clamped to line lengths and padded with spaces if needed.
- [x] Make the cursor overlay compatible with the new line renderer (or merge cursor rendering into it and remove `CursorTracker` if redundant).

### 5) Buffer/LLM Normalization Consistency
- [x] Decide whether to reuse the line renderer (or a shared parser) for `pkg/ui/terminal/normalizer.go` so command reconstruction respects cursor movement.
- [x] Update `Normalizer` to treat CSI `D/C` as cursor moves, not deletions.
- [x] Add/adjust prompt extraction tests to cover in-line edits.

### 6) Tests
- [x] Add unit tests for the new line renderer:
  - [x] `abc`, left, left, `X` => `aXc` (overwrite) or `aXbc` (insert) depending on intended behavior
  - [x] cursor-left/right with no deletion
  - [x] backspace at various positions
  - [x] CR/LF line wrapping behavior
- [x] Add viewport tests for correct cursor placement with left/right movements.
- [x] Add normalization tests to ensure command lines are reconstructed correctly after cursor moves.

### 7) Validation
- [x] Run `make check`.
- [ ] Manually verify left/right navigation in bash/zsh and with paste/selection interactions.

## Definition of Done
- Left/right arrows move the cursor as expected in the prompt line.
- The cursor indicator appears at the correct position when editing.
- No regressions in prompt parsing or history capture.
- All tests pass.
