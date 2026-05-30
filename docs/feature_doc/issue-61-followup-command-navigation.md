# Issue 61 Follow-up: Sidebar Command Navigation When Content Fits

Predecessor: `docs/feature_doc/issue-61-sidebar-command-scroll.md`
GitHub issue: follow-up to https://github.com/pakru/wtf_cli/issues/61 (regression found after the issue-61 fix shipped).

## Problem

After the issue-61 fix decoupled scrolling from command selection, a new regression
appeared: when an assistant message contains **several `<cmd>...</cmd>` entries but the
whole message fits in the viewport (no scrolling)**, the user cannot select an upper
command. The selection is permanently stuck on the **last** command, so the upper
commands can never be highlighted or applied with `Enter`.

The symptom appears after streaming finishes, when command selection becomes enabled.

## Current Behavior Review

Relevant code in `pkg/ui/components/sidebar/sidebar.go`:

- `handleScroll(key)` now forwards every arrow/page key straight to `scrollViewport(key)`.
  Command navigation was removed in the issue-61 fix.
- `scrollViewport(key)` mutates `scrollY`/`follow`, then calls `updateActiveCommand()`.
- `updateActiveCommand()` is a **passive** tracker: among the commands whose rendered
  line is inside `[scrollY, scrollY + viewportHeight - 1]`, it always selects the one
  with the **highest** rendered line (the last visible command). It sets
  `cmdSelectedIdx = -1` when no command is visible.
- `commandSelectionEnabled()` is true only when streaming is finished, the textarea is
  empty, and at least one command exists.

Why it breaks when the message fits on screen:

- `maxScroll()` returns `0`, so `scrollViewport("up")` / `scrollViewport("down")` cannot
  change `scrollY`.
- After the no-op scroll, `updateActiveCommand()` runs again and re-selects the **last
  visible command**.
- Result: Up/Down can never land on an upper command; selection snaps back to the last
  one every time.

This is a direct consequence of the issue-61 decision to make Up/Down pure scrolling.
That decision is correct for long, scrollable messages but leaves short multi-command
messages with no way to move the selection.

## Desired Behavior

Up/Down should become a **command-step-then-scroll** hybrid. Wheel, PgUp, and PgDown stay
pure viewport scrolling (unchanged from issue-61).

1. If there is another **visible** command in the pressed direction (relative to the
   current selection), move the selection to it. Do not scroll.
2. If there is no further visible command in that direction, scroll the text one line and
   **keep the current selection** as long as it stays on screen. Only when the selected
   command scrolls out of view does the passive last-visible tracker take over again.

Edge cases:

- Up on the topmost command with `scrollY == 0` is a no-op: the selection stays put and
  must never jump back to the last command. (This is the actual regression.)
- Up on the topmost *visible* command while text remains above scrolls the text up one
  line and keeps the selection (which is still visible).
- Down on the last command with no scroll room is a no-op (selection stays on the last
  command).
- While streaming, or when the input box has text, `commandSelectionEnabled()` is false,
  so Up/Down remain pure scrolling exactly as today.

## Proposed Implementation

All changes are in `pkg/ui/components/sidebar/sidebar.go`.

### 1. Split scroll mechanics from passive selection

Extract the `switch` body of `scrollViewport()` into `applyScroll(key)`, which only
mutates `scrollY` and `follow`. Keep `scrollViewport()` as `applyScroll()` followed by
`updateActiveCommand()`, so the wheel/page-key paths behave exactly as they do now.

```go
func (s *Sidebar) scrollViewport(key string) {
    s.applyScroll(key)
    s.updateActiveCommand()
}

func (s *Sidebar) applyScroll(key string) {
    maxScroll := s.maxScroll()
    switch key {
    case "up":
        if s.scrollY > 0 {
            s.scrollY--
            s.follow = false
        }
    case "down":
        if s.scrollY < maxScroll {
            s.scrollY++
        }
        s.follow = s.scrollY >= maxScroll
    case "pgup":
        s.scrollY -= 10
        if s.scrollY < 0 {
            s.scrollY = 0
        }
        s.follow = false
    case "pgdown":
        s.scrollY += 10
        if s.scrollY > maxScroll {
            s.scrollY = maxScroll
        }
        s.follow = s.scrollY >= maxScroll
    }
}
```

### 2. Branch Up/Down through the hybrid navigator

```go
func (s *Sidebar) handleScroll(key string) tea.Cmd {
    if s.commandSelectionEnabled() && (key == "up" || key == "down") {
        s.navigateCommandsOrScroll(key)
        return nil
    }
    s.scrollViewport(key) // wheel/pgup/pgdown, or streaming / input-has-text path
    return nil
}
```

`handleScroll` is already the single entry point for both the FocusInput and
FocusViewport key branches in `Update()`, so no other call sites change. `HandleWheel()`
keeps calling `scrollViewport()` directly (pure scroll).

### 3. Add the hybrid navigator (keep-selection on scroll)

```go
func (s *Sidebar) navigateCommandsOrScroll(key string) {
    dir := 1
    if key == "up" {
        dir = -1
    }
    if s.stepVisibleCommand(dir) {
        return // rule 1: moved to a visible command in this direction
    }
    // rule 2: scroll text, preserving the selection while it stays on screen
    s.applyScroll(key)
    if !s.commandVisible(s.cmdSelectedIdx) {
        s.updateActiveCommand()
    }
}
```

Because `applyScroll` only moves one line and the selected command sat at the viewport
edge in the opposite direction, it stays visible after the step and the selection is
preserved. The selection only resets (via `updateActiveCommand`) once it actually leaves
the viewport during longer scroll runs.

### 4. Add the two helpers

```go
// stepVisibleCommand selects the nearest on-screen command in direction dir
// (-1 = up, +1 = down) relative to the current selection's rendered line.
// Returns true when the selection moved.
func (s *Sidebar) stepVisibleCommand(dir int) bool {
    top := s.scrollY
    bottom := top + s.viewportHeight() - 1

    curLine := -1
    if s.cmdSelectedIdx >= 0 && s.cmdSelectedIdx < len(s.cmdRenderedLines) {
        curLine = s.cmdRenderedLines[s.cmdSelectedIdx]
    }

    bestIdx := -1
    bestLine := -1
    for i, lineIdx := range s.cmdRenderedLines {
        if i >= len(s.cmdList) || lineIdx < 0 {
            continue
        }
        if lineIdx < top || lineIdx > bottom {
            continue // visible commands only
        }
        if dir > 0 {
            if lineIdx <= curLine {
                continue
            }
            if bestLine == -1 || lineIdx < bestLine { // nearest below
                bestLine = lineIdx
                bestIdx = i
            }
        } else {
            if curLine >= 0 && lineIdx >= curLine {
                continue
            }
            if lineIdx > bestLine { // nearest above
                bestLine = lineIdx
                bestIdx = i
            }
        }
    }

    if bestIdx == -1 {
        return false
    }
    s.cmdSelectedIdx = bestIdx
    return true
}

// commandVisible reports whether the command at idx has a rendered line inside
// the current viewport window.
func (s *Sidebar) commandVisible(idx int) bool {
    if idx < 0 || idx >= len(s.cmdRenderedLines) {
        return false
    }
    lineIdx := s.cmdRenderedLines[idx]
    if lineIdx < 0 {
        return false
    }
    top := s.scrollY
    bottom := top + s.viewportHeight() - 1
    return lineIdx >= top && lineIdx <= bottom
}
```

Note: when `curLine == -1` (no current selection, e.g. selection scrolled into a gap with
no visible commands), a Down step picks the first visible command and an Up step picks the
last visible command. This is acceptable and self-consistent with the passive tracker.

### 5. Footer hint

Revert the footer hint so it advertises navigation again. In `commandFooterText()`:

```text
Enter Apply | Up/Down Navigate | Shift+Tab TTY | Ctrl+T Hide
```

No test asserts this string today (the `applyFooterHint` constant was removed in the
issue-61 change), so this is a free wording update.

### What stays unchanged

`updateActiveCommand`, `RefreshView`, follow-mode anchoring, `HandleWheel`, and the
PgUp/PgDown paths are untouched. The hybrid only activates for Up/Down while
`commandSelectionEnabled()` is true.

## Test Plan

Add to `pkg/ui/components/sidebar/sidebar_chat_test.go`:

- `TestSidebar_ArrowUpSelectsUpperCommandWhenNoScroll`
  - Three `<cmd>` entries that all fit on screen (`maxScroll() == 0`).
  - Ensure command selection is enabled; selection starts on the last command.
  - Press Up: selection steps last → middle → first.
  - Press Up again at the top: selection stays on the first command (regression guard;
    must not jump back to the last).

- `TestSidebar_ArrowDownStopsAtLastCommandWhenNoScroll`
  - Symmetric: from the first command, Down steps down and stops on the last command,
    `scrollY` stays `0`, no snap.

- `TestSidebar_ArrowNavigationStepsThenScrollsPreservingSelection`
  - Scrollable content with commands near one edge.
  - Step until commands run out in a direction, then assert the next press changes
    `scrollY` while `cmdSelectedIdx` stays on the same command (it is still visible).

- `TestSidebar_ArrowNavigationFallsBackToScrollWhileStreaming`
  - `SetStreaming(true)` ⇒ `commandSelectionEnabled()` is false ⇒ Up/Down change `scrollY`
    and do not step commands.

Existing issue-61 regression tests should keep passing unchanged, because they use a
single command at the top of scrollable content: `stepVisibleCommand` finds no
in-direction visible command and falls through to `applyScroll`, giving the same
observable `scrollY` result.

Run:

```bash
go test ./pkg/ui/...
make check
```

## Acceptance Criteria

- With multiple commands and no scrollbar, Up/Down move the selection between commands;
  the upper command is reachable and applyable with `Enter`.
- Up on the topmost command with no scroll room is a no-op and never jumps to the last
  command.
- When commands run out in a direction, the text scrolls one line and the current
  selection is preserved until it leaves the viewport.
- Wheel, PgUp/PgDown, follow-mode, and streaming behavior are unchanged from issue-61.
- Footer hint matches the restored Up/Down navigation behavior.
