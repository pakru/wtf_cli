# Resize PTY When Chat Sidebar Toggles

When the chat sidebar is shown or hidden, the viewport width is updated correctly (via `applyLayout()`), but the **PTY is never resized**. The shell continues to think it owns the full terminal width, so line wrapping and output remain at the original width — meaning content gets clipped behind the sidebar.

## Root Cause

`applyLayout()` in `pkg/ui/model.go` correctly adjusts the viewport and sidebar component sizes, but **explicitly skips PTY resize** (see comment on line 1332). PTY resize currently occurs in only two places:

1. **`resizeApplyMsg`** — triggered by `tea.WindowSizeMsg` (actual terminal resize), debounced.
2. **`enterFullScreen`** — calls `applyLayout()` after setting `fullScreenMode = true`, which takes the full-screen branch that does resize directly.

`exitFullScreen()` sets `fullScreenMode = false` and then calls `applyLayout()`, but since the non-fullscreen path of `applyLayout()` **currently skips PTY resize**, exiting full-screen also does not resize the PTY to the viewport width.

None of these paths fire when the sidebar is toggled via `Ctrl+T`, `Shift+Tab`, `/explain`, `/chat`, or `Esc`.

## Secondary Issue: No Dimension Guards on ResizePTY

`terminal.ResizePTY()` casts `int` arguments to `uint16` with no validation:

```go
// pkg/ui/terminal/resize.go
size := &pty.Winsize{
    Rows: uint16(height),
    Cols: uint16(width),
}
```

If `applyLayout()` ever runs before the first `tea.WindowSizeMsg`, `m.height` is `0`, which makes `viewportHeight := m.height - 1` equal to `-1`. Cast to `uint16`, that wraps to `65535`. Our `ResizePTY` call would then set the child PTY to 65535 rows/cols via `ioctl TIOCSWINSZ` and send `SIGWINCH` to the child shell, which may misbehave or crash.

The same unguarded calls exist in the `resizeApplyMsg` handler — both code paths share this risk.

## Proposed Changes

### 1. Extract a `resizePTYViewport` helper on `Model`

Centralise the guard, the call, and error handling in one place so both `applyLayout()` and `resizeApplyMsg` stay consistent.

```go
// resizePTYViewport resizes the PTY to the given viewport dimensions.
// It is a no-op if the PTY is unset or either dimension is non-positive,
// which guards against uint16 overflow before the first WindowSizeMsg.
func (m *Model) resizePTYViewport(w, h int) {
	if m.ptyFile == nil || w <= 0 || h <= 0 {
		return
	}
	if err := terminal.ResizePTY(m.ptyFile, w, h); err != nil {
		slog.Warn("pty_resize_failed", "width", w, "height", h, "error", err)
	}
}
```

### 2. Call the helper from `applyLayout()`

After computing the effective viewport dimensions, call the helper instead of the raw `terminal.ResizePTY`:

```diff
 	m.viewport.SetSize(viewportWidth, viewportHeight)
 	m.palette.SetSize(m.width, m.height)
+
+	// Resize PTY to match the effective viewport width so the shell's line
+	// wrapping stays in sync when the sidebar is toggled.
+	m.resizePTYViewport(viewportWidth, viewportHeight)
+
 	resultHeight := m.height
```

> **Note:** This does **not** suppress prompt reprints (via `m.resizeTime`). A sidebar toggle is user-initiated and infrequent, so a prompt reprint from the shell is acceptable and expected — the shell needs to redraw at the new width.

### 3. Migrate `resizeApplyMsg` to use the helper

Replace the existing unguarded `terminal.ResizePTY` call in the `resizeApplyMsg` handler:

```diff
-			terminal.ResizePTY(m.ptyFile, viewportWidth, viewportHeight)
-			// Track resize time to suppress prompt reprint output
+			m.resizePTYViewport(viewportWidth, viewportHeight)
+			// Track resize time to suppress prompt reprint output
```

The `resizeApplyMsg` handler keeps its own concern — updating `m.resizeTime` and `m.initialResize` — as those are specific to external terminal resizes (window drag/snap) and don't belong in the shared helper.

## Full-Screen App Safety

Full-screen apps (`vim`, `htop`, etc.) are **not affected**:

- `applyLayout()` has an **early `return`** when `m.fullScreenMode == true` (before the new call site).
- All key presses in full-screen mode bypass sidebar shortcuts and return immediately, so `ToggleChatMsg`/`FocusSwitchMsg` are never generated.
- `exitFullScreen()` sets `m.fullScreenMode = false` before calling `applyLayout()`, so the helper correctly resizes to the viewport width (accounting for any open sidebar) on exit — this is a bonus fix.

## Files Changed

| File | Change |
|------|--------|
| `pkg/ui/model.go` | Add `resizePTYViewport` helper; call it from `applyLayout()` and `resizeApplyMsg` |

## Verification Plan

### Automated Tests
- `make check` — all existing tests must pass (including golden tests).

#### New unit tests (`pkg/ui/model_test.go`)

**`TestModel_ResizePTYViewport_NilPTY`** — verifies the helper is a no-op when `m.ptyFile == nil`:
```go
m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
m.resizePTYViewport(80, 24) // must not panic
```

**`TestModel_ResizePTYViewport_Guard`** — verifies invalid dimensions are rejected even with a real PTY. Uses `pty.Open()` so the nil-PTY early-return doesn't mask the dimension check:
```go
ptm, pts, err := pty.Open()
if err != nil {
    t.Skip("pty.Open not available:", err)
}
defer ptm.Close(); defer pts.Close()

// Set a known baseline size
terminal.ResizePTY(ptm, 80, 24)

m := NewModel(ptm, buffer.New(100), capture.NewSessionContext(), nil)
m.resizePTYViewport(0, 24)   // zero width  → must be rejected
m.resizePTYViewport(80, 0)   // zero height → must be rejected
m.resizePTYViewport(-1, 24)  // negative    → must be rejected

// PTY size must remain at the 80×24 baseline
w, h, err := terminal.GetPTYSize(ptm)
if err != nil {
    t.Fatalf("GetPTYSize: %v", err)
}
if w != 80 || h != 24 {
    t.Errorf("Expected PTY size 80×24, got %d×%d", w, h)
}
```

**`TestModel_ApplyLayout_ResizesPTYOnSidebarToggle`** — verifies the actual PTY cols change on sidebar open/close. Uses a real PTY and `terminal.GetPTYSize()` to assert the kernel-level size, not just the viewport struct:
```go
ptm, pts, err := pty.Open()
if err != nil {
    t.Skip("pty.Open not available:", err)
}
defer ptm.Close(); defer pts.Close()

m := NewModel(ptm, buffer.New(100), capture.NewSessionContext(), nil)
newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
// drive resizeApplyMsg to apply the initial PTY size
newModel, _ = newModel.Update(resizeApplyMsg{id: 1, width: 120, height: 30})
m = newModel.(Model)
m.resizeDebounceID = 1

// Open sidebar → PTY cols should become splitSidebarWidths(120).left
newModel, _ = m.Update(input.ToggleChatMsg{})
m = newModel.(Model)
left, _ := splitSidebarWidths(120)
w, _, err := terminal.GetPTYSize(ptm)
if err != nil {
    t.Fatalf("GetPTYSize: %v", err)
}
if w != left {
    t.Errorf("Expected PTY cols %d with sidebar open, got %d", left, w)
}

// Close sidebar → PTY cols should return to 120
newModel, _ = m.Update(input.ToggleChatMsg{})
m = newModel.(Model)
w, _, err = terminal.GetPTYSize(ptm)
if err != nil {
    t.Fatalf("GetPTYSize: %v", err)
}
if w != 120 {
    t.Errorf("Expected PTY cols 120 with sidebar closed, got %d", w)
}
```

### Manual Verification
1. `make run`
2. Note the initial column count: run `tput cols` or `stty size` in the shell.
3. Open sidebar (`Ctrl+T`) — run `tput cols` again and confirm it decreased to the left-pane width.
4. Type a long command — verify it wraps within the visible pane, not behind the sidebar.
5. Close sidebar (`Ctrl+T`) — run `tput cols` and confirm it returned to the original value.
6. Run wide output (`ls -la`) with sidebar open — verify output stays within the visible pane.
7. Launch `vim` with sidebar open — full-screen renders correctly, no corruption.
8. Exit `vim` with sidebar still open — run `tput cols` and confirm the shell sees the sidebar-adjusted width.
