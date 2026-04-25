# Issue #15 Implementation Plan: Add Support for Terminal Screen Scroll

## Issue Summary

GitHub issue [#15](https://github.com/pakru/wtf_cli/issues/15) requests scroll support for the main terminal viewport — the ability to scroll back through command output without leaving the terminal or opening a pager.

## Current State (Code Review)

### 1) PTYViewport scroll API exists but is unwired

`pkg/ui/components/viewport/viewport.go` already exposes:
- `ScrollUp()` / `ScrollDown()` — scroll one line
- `PageUp()` / `PageDown()` — scroll one full page
- `IsAtBottom()` — returns true when at the bottom
- `Stats()` — total lines, visible lines, scroll percentage

None of these are called from any keyboard handler or model path today.

### 2) Arrow keys go directly to PTY; PageUp/PageDown are ignored in normal mode

`pkg/ui/input/input.go` routes `up` / `down` straight to the PTY as ANSI cursor sequences (`\x1b[A` / `\x1b[B`). `pgup` and `pgdown` have no case in the normal `HandleKey` switch and are returned as unhandled — they are only forwarded to the PTY in the full-screen and secret bypass paths (which call `sendKeyToPTY` directly). There is no "scroll mode" concept anywhere in the input handler or model today.

### 3) Auto-scroll is always on

`AppendOutput()` in `viewport.go` calls `v.Viewport.GotoBottom()` on every PTY chunk. While a user is scrolled back, new output would snap them back to the bottom — this must be suppressed during scroll mode.

### 4) Mouse wheel is intentionally disabled

`model.go` line 1113–1114 has a TODO comment explicitly disabling mouse mode until wheel scrolling is properly implemented.

---

## Goals

1. Let the user scroll back through PTY output with keyboard shortcuts.
2. Pause auto-scroll while the user is scrolled up; resume on reaching the bottom.
3. Exit scroll mode cleanly with `Esc` or automatically when the user scrolls back to the bottom.
4. Show a visible indicator in the status bar when auto-scroll is paused.
5. Keep full-screen apps (vim, htop) unaffected — scroll mode must be disabled when `fullScreenMode` is active.
6. Keep sidebar / chat focus unaffected — scroll mode applies only when the terminal viewport has focus.
## Non-Goals

- Mouse wheel scrolling (tracked separately per the existing TODO comment).
- Infinite scrollback — the existing `LineRenderer` content storage is sufficient; no scrollback size increase is required.
- Searchable scrollback (separate feature).
- Scrolling inside the chat sidebar (separate viewport with its own key handling).

---

## Proposed Design

### A) New "scroll mode" state in the model

**File:** `pkg/ui/model.go`

Add a boolean field `scrollMode bool` to the `Model` struct.

Semantics:
- `scrollMode = true` — user is browsing scrollback; auto-scroll is paused.
- `scrollMode = false` (default) — normal operation; new output auto-scrolls to bottom.

Transition rules:
| Trigger | Transition |
|---------|-----------|
| `alt+up` / `PageUp` pressed while terminal has focus and not in full-screen mode | `false → true` |
| `shift+down` / `PageDown` reaches bottom while in scroll mode | `true → false` |
| `Esc` pressed while in scroll mode | `true → false`, scroll to bottom |

> PTY output never exits scroll mode by itself — only explicit user scroll-to-bottom does.

> **Key choice rationale:** Plain arrow keys must continue going to PTY (shell history navigation, readline). `alt+up` / `alt+down` are used instead of `shift+up` / `shift+down` because Konsole (KDE) and most terminal emulators intercept the Shift variants for their own scrollback buffer — those keys never reach the running application. Alt+arrow keys are passed through by virtually all terminals and are not bound by bash readline.

### B) Scroll-key interception in model.go (not in InputHandler)

**File:** `pkg/ui/model.go`

Intercept scroll keys in the `Update` method's `tea.KeyPressMsg` branch, **after** overlay/sidebar routing and **before** the call to `inputHandler.HandleKey`. Gate on `m.terminalFocused && !m.fullScreenMode` to ensure scroll keys cannot fire while the chat sidebar has focus:

```go
case tea.KeyPressMsg:
    // Overlays / sidebar handle their own keys first (existing logic) …

    // Scroll-key interception — terminal focus only, no full-screen
    if m.terminalFocused && !m.fullScreenMode {
        switch msg.String() {
        case "shift+up":
            return m.doScrollUp()
        case "shift+down":
            return m.doScrollDown()
        case "pgup":
            return m.doPageUp()
        case "pgdown":
            return m.doPageDown()
        case "esc":
            if m.scrollMode {
                return m.setScrollMode(false)
            }
        }
    }
    // … existing inputHandler.HandleKey call
```

This avoids adding a `scrollMode` mirror to `InputHandler` and removes the need for `SetScrollMode` synchronization on the handler.

**New message types** are no longer needed — scrolling is handled synchronously inside the `Update` case above.

> **Full-screen entry resets scroll mode:** `enterFullScreen` must call `setScrollMode(false)` before enabling `fullScreenMode`. This ensures that if the user was scrolled back when they launched vim or htop, scroll mode and auto-scroll are cleanly reset on alt-screen activation.

### C) Scroll helpers in model.go

**File:** `pkg/ui/model.go`

Add a `setScrollMode` helper that keeps all three pieces of state in sync atomically:

```go
func (m *Model) setScrollMode(active bool) (Model, tea.Cmd) {
    m.scrollMode = active
    m.viewport.SetAutoScroll(!active)
    m.statusBar.SetScrollMode(active)
    return *m, nil
}
```

Add directional helpers. **Scroll first, then enter scroll mode only if the viewport actually moved off the bottom** — this prevents disabling auto-scroll on output that is shorter than the viewport:

```go
func (m *Model) doScrollUp() (Model, tea.Cmd) {
    m.viewport.ScrollUp()
    if !m.viewport.IsAtBottom() {
        return m.setScrollMode(true)
    }
    return *m, nil
}

func (m *Model) doScrollDown() (Model, tea.Cmd) {
    m.viewport.ScrollDown()
    if m.viewport.IsAtBottom() {
        return m.setScrollMode(false)
    }
    return *m, nil
}

func (m *Model) doPageUp() (Model, tea.Cmd) {
    m.viewport.PageUp()
    if !m.viewport.IsAtBottom() {
        return m.setScrollMode(true)
    }
    return *m, nil
}

func (m *Model) doPageDown() (Model, tea.Cmd) {
    m.viewport.PageDown()
    if m.viewport.IsAtBottom() {
        return m.setScrollMode(false)
    }
    return *m, nil
}
```

`setScrollMode(false)` is also called from the `Esc` intercept (section B) when `m.scrollMode` is true.

### D) Suppress auto-scroll during scroll mode

**File:** `pkg/ui/components/viewport/viewport.go` (or call site in `model.go`)

Change `AppendOutput` so that `GotoBottom()` is only called when not in scroll mode. Two options:

**Option 1 (preferred):** Add a `pauseAutoScroll bool` field to `PTYViewport`:

```go
func (v *PTYViewport) SetAutoScroll(enabled bool) {
    v.pauseAutoScroll = !enabled
    if enabled {
        v.Viewport.GotoBottom()
    }
}
```

In `AppendOutput`, replace the unconditional `v.Viewport.GotoBottom()` with:
```go
if !v.pauseAutoScroll {
    v.Viewport.GotoBottom()
}
```

The model calls `m.viewport.SetAutoScroll(!m.scrollMode)` whenever `scrollMode` changes.

**Option 2:** Let the model call `GotoBottom()` explicitly rather than inside `AppendOutput`. More invasive refactor — prefer Option 1.

### E) Status bar: auto-scroll paused indicator

**File:** `pkg/ui/components/statusbar/statusbar_view.go`

Add a `scrollMode bool` field to `StatusBarView` and a setter:

```go
func (s *StatusBarView) SetScrollMode(active bool) {
    s.scrollMode = active
}
```

In `Render()`, when `scrollMode` is true, prepend a fixed badge to `rightContent` in place of the normal hint:

```go
if s.scrollMode {
    rightContent = "[SCROLL PAUSED]  Esc to resume"
} else if s.message == "" {
    rightContent = "Press / for commands"
}
```

This reuses the existing right-side content slot with no new layout changes. The model calls `m.statusBar.SetScrollMode(m.scrollMode)` inside `setScrollMode`.

---

## Affected Files

| File | Change |
|------|--------|
| `pkg/ui/model.go` | Add `scrollMode bool`, `setScrollMode`/`doScroll*` helpers, intercept scroll keys after overlay/sidebar routing gated on `m.terminalFocused && !m.fullScreenMode` |
| `pkg/ui/components/viewport/viewport.go` | Add `pauseAutoScroll` field and `SetAutoScroll()` method; gate `GotoBottom()` call |
| `pkg/ui/components/statusbar/statusbar_view.go` | Add `scrollMode bool` field, `SetScrollMode()` setter, show `[SCROLL PAUSED]` badge in right content when active |


---

## Verification Plan

### Pre-submit validation
```bash
make check
```

### Manual tests

| Scenario | Expected |
|----------|---------|
| `alt+up` in normal shell | Enters scroll mode; viewport scrolls up one line; status bar shows `[SCROLL PAUSED]` |
| `PageUp` in normal shell | Enters scroll mode; viewport scrolls up one page; status bar shows `[SCROLL PAUSED]` |
| `alt+down` while scrolled up | Scrolls down; exits scroll mode when bottom reached |
| `Esc` while in scroll mode | Exits scroll mode immediately; snaps to bottom; status bar badge disappears |
| Type a command while in scroll mode | Key sent to PTY normally (scroll mode is transparent to PTY I/O) |
| New output while in scroll mode | Viewport does NOT auto-scroll |
| New output while NOT in scroll mode | Viewport auto-scrolls to bottom as before |
| Enter vim / htop | Scroll mode cannot be activated (full-screen mode guard) |
| Arrow keys (plain `up` / `down`) | Always sent to PTY — no regression in shell history navigation |
| `pgup` / `pgdown` in normal shell | Intercepted (were previously ignored in normal mode; full-screen/secret paths still forward `\x1b[5~` / `\x1b[6~` to PTY unaffected) |
| `alt+up` / `alt+down` while chat sidebar has focus | NOT intercepted — sidebar focus guard prevents scroll mode from activating |

### Unit tests

Add focused tests alongside existing coverage:

| Test file | Cases to add |
|-----------|-------------|
| `pkg/ui/model_test.go` | `alt+up` on short output stays at bottom (no scroll mode); scroll mode auto-exits when `doScrollDown` reaches bottom; `Esc` exits scroll mode; scroll keys do nothing when sidebar has focus; scroll keys do nothing in full-screen mode |
| `pkg/ui/components/viewport/viewport_test.go` | `SetAutoScroll(false)` suppresses `GotoBottom` on `AppendOutput`; `SetAutoScroll(true)` re-enables and snaps to bottom |
| `pkg/ui/components/statusbar/statusbar_view_test.go` | `SetScrollMode(true)` renders `[SCROLL PAUSED]` badge; `SetScrollMode(false)` reverts to normal hint |
| `pkg/ui/pty_batch_test.go` | New PTY output while `scrollMode=true` does not move viewport position |

If rendering changes affect golden snapshots, regenerate:
```bash
go test ./pkg/ui/... -update
```
