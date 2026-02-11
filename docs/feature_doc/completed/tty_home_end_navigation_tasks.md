# TTY Home/End Navigation - Task List

## Problem Summary
Home/End should move the cursor to the start/end of the current input line in normal (non–full-screen) shell mode, matching standard terminal behavior. Today the rendering and normalization layers do not track or render home/end correctly in normal mode.

## Goals
- Home moves the cursor to column 0 of the current line.
- End moves the cursor to the end of the current line.
- Cursor indicator renders at the correct position.
- Buffer/LLM normalization respects home/end moves and in-line edits.
- Full-screen apps continue to work (pass-through behavior unchanged).

---

## Tasks

### 1) Reproduce & Instrument (Short-lived)
- [x] Add temporary trace logging for `home`/`end` keypresses (key string, code, mod) and PTY output sequences.
- [x] Verify whether the shell emits `ESC [ H/F`, `ESC O H/F`, or other sequences for home/end in normal mode (observed no PTY output sequences; indicates keys aren’t forwarded in normal mode).
- [x] Remove/disable temporary logs after behavior is confirmed.

### 2) Input Path Verification
- [x] Confirm Bubble Tea v2 emits `home`/`end` in `tea.KeyPressMsg` and `InputHandler.HandleKey` routes them to the PTY in normal mode.
- [x] If needed, use cursor app-mode selection in normal mode (same `cursorSeq(normal, app)` used for arrows).
- [x] Ensure overlays (palette/history/settings) still intercept home/end appropriately when active.

### 3) Line Renderer Support (Normal Mode)
- [x] Update `pkg/ui/terminal/line_renderer.go` to handle CSI `H`/`F`:
  - [x] `H` moves cursor to column 0 on the current line (home).
  - [x] `F` moves cursor to end of the current line (end).
  - [x] Keep cursor clamped and allow movement past line end when appropriate (padding on write).
- [ ] Add support for other common sequences if observed (e.g., `ESC [ 1~`, `ESC [ 4~`).

### 4) Cursor Overlay Alignment
- [x] Ensure `CursorTracker`/renderer uses the updated cursor position after home/end.
- [x] Add tests that place cursor at start/end and verify overlay placement in viewport content.

### 5) Buffer/LLM Normalization
- [x] Update `pkg/ui/terminal/normalizer.go` to treat home/end as cursor moves (not deletion).
- [x] Add prompt reconstruction tests with in-line edits after home/end movement.

### 6) Tests
- [x] Add line renderer tests:
  - [x] `abc`, home, `X` => `Xbc` (overwrite at start).
  - [x] `abc`, end, `X` => `abcX`.
- [x] Add viewport tests for cursor placement at start/end.
- [x] Add normalization tests for commands edited using home/end.

### 7) Validation
- [x] Run `make check`.
- [ ] Manually verify Home/End in bash/zsh and with paste/history interactions.

## Definition of Done
- Home/End move the cursor correctly in normal shell input.
- Cursor indicator renders at the correct position.
- Command reconstruction in buffer/LLM is accurate after home/end edits.
- Tests pass.
