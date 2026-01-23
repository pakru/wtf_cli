# Paste Support (Bracketed Paste) - Task List

## Overview
Add reliable paste support for the shell PTY by handling Bubble Tea v2 paste messages and forwarding content to the PTY, preserving bracketed-paste behavior when the shell enables it.

## Goals
- Pasted text reaches the PTY in normal and full-screen modes.
- Bracketed paste is respected when the shell enables it (CSI ? 2004 h/l).
- Pasted text does not trigger palette/history shortcuts.
- Line buffer tracking remains correct after paste (single and multi-line).

## Tasks

### 1) Input Path Audit
- [x] Confirm Bubble Tea v2 emits `tea.PasteMsg` (and optionally `PasteStartMsg` / `PasteEndMsg`) for bracketed paste.
- [x] Verify `pkg/ui/model.go` only handles `tea.KeyPressMsg` today and does not handle paste messages.
- [x] Verify no explicit disablement of bracketed paste in program setup.

### 2) Input Handler: Bracketed Paste Mode
- [x] Extend `pkg/ui/input/input.go` to track bracketed paste mode by parsing `CSI ? 2004 h` (enable) and `CSI ? 2004 l` (disable) in `UpdateTerminalModes`.
- [x] Add a `HandlePaste(content string)` method that writes paste content to the PTY, wrapping with `\x1b[200~` and `\x1b[201~` if bracketed paste mode is on.
- [x] Update line buffer tracking after paste: if content contains newlines, set line buffer to the last line; set `atLineStart` based on whether the last character is a newline.

### 3) Model Wiring
- [x] Handle `tea.PasteMsg` in `pkg/ui/model.go`.
- [x] When overlays are inactive, route paste to `InputHandler.HandlePaste`.
- [x] In full-screen mode, pass paste directly to the PTY (bypass overlays).
- [x] Ensure paste does not open palette/history picker.

### 4) Tests
- [x] Add tests in `pkg/ui/input/input_test.go` for paste:
  - [x] Single-line paste updates `lineBuffer` and sends raw bytes.
  - [x] Multi-line paste updates `lineBuffer` to the final line and `atLineStart` correctly.
  - [x] Bracketed paste on/off wraps content as expected.
- [x] Add a model-level test to confirm `tea.PasteMsg` is routed to the PTY and does not trigger overlays.

### 5) Validation and Docs
- [ ] Run `make check`.
- [ ] Update any relevant docs/notes if we want to mention bracketed paste behavior.

## Definition of Done
- Paste works in the shell prompt and full-screen apps.
- Bracketed paste mode is honored when enabled by the shell.
- Tests for paste behavior pass.
