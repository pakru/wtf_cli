# Issue 61: Sidebar Command Scroll Behavior

GitHub issue: https://github.com/pakru/wtf_cli/issues/61

## Problem

When an assistant message contains one or more `<cmd>...</cmd>` entries, the chat sidebar can become anchored to the selected command instead of scrolling naturally through the message text.

The issue page currently has no body or comments, so this plan is based on the reported behavior: text scrolling in the sidebar stops working correctly once a command is selected, and the view appears stuck on that selected command.

The user-visible symptom should appear after streaming finishes, not during streaming. Command selection is disabled while `s.streaming` is true, so wheel events can still scroll normally until the completed response enables command selection.

## Current Behavior Review

Relevant code:

- `pkg/ui/components/sidebar/sidebar.go`
  - `RefreshView()` reflows messages, then when `follow` is true it sets `scrollY` to `maxScroll()` and immediately calls `selectLastCommand()`.
  - `selectLastCommand()` calls `revealSelectedCommand()`, which mutates `scrollY` to center the command line.
  - `commandSelectionEnabled()` returns true only when streaming is finished, the textarea is empty, and at least one command exists.
  - `handleScroll()` treats only `up` and `down` as command navigation whenever `commandSelectionEnabled()` is true.
  - `pgup` and `pgdown` are not hijacked today; they already fall through to the viewport-scroll branch.
  - `HandleWheel()` maps mouse wheel up/down directly to `handleScroll("up")` and `handleScroll("down")`.

This means command selection can control scroll position in two paths:

1. During message refresh after a command-bearing response, `follow` mode should keep the viewport at the bottom, but `selectLastCommand()` can pull the viewport back to the latest command instead.
2. After streaming ends, mouse wheel scrolling can step command selection instead of changing `scrollY`. With a single command, there may be no previous/next command, so the wheel appears to do nothing.

Existing tests encoded part of the old command-navigation behavior before this fix:

- `TestSidebar_ArrowKeysStepCommandsWhenScrollSelectionStaysStable`
- `TestSidebar_RefreshView_SelectsLastCommandWhenFollowing`
- `TestSidebar_NavigatesAcrossAllRenderedCommands`

Those tests should be revised carefully rather than deleted blindly, because command application is still a useful feature. The implemented direction replaces arrow-key command navigation with viewport scrolling and passive visible-command selection.

## Desired Behavior

Scrolling and command selection should be separate concerns.

- Follow mode should follow new chat content to the bottom.
- `Up` and `Down` should scroll the sidebar text even when commands are selectable.
- Mouse wheel scrolling should always scroll the sidebar text.
- `PgUp` and `PgDown` should keep their current viewport-scroll behavior.
- Passive command highlighting may track visible commands as the viewport moves, but it should not move the viewport.
- Enter should apply a command only when the current command selection is valid and understandable to the user.

## Proposed Implementation

### 1. Split viewport scrolling from command navigation

In `pkg/ui/components/sidebar/sidebar.go`, replace the overloaded `handleScroll(key string)` behavior with separate helpers:

- `scrollViewport(key string)`
  - Updates `scrollY`.
  - Updates `follow`.
  - Calls a passive command-selection sync method.
  - Handles `up`, `down`, `pgup`, and `pgdown`.

Update `HandleWheel()` to call the viewport scroll helper directly. `PgUp` and `PgDown` already scroll text today, so the implementation should preserve that behavior rather than treating it as a new fix. `Up` and `Down` should also use this helper, instead of command navigation.

### 2. Fix follow-mode refresh

Change `RefreshView()` so follow mode means bottom anchoring only:

```go
if s.follow {
    s.scrollY = s.maxScroll()
    s.updateActiveCommand()
    return
}
```

Do not call `selectLastCommand()` from the follow path if that method reveals the command and mutates `scrollY`.

`reflow()` already calls `updateActiveCommand()`, but the call in this follow-mode branch is still needed because `scrollY` changes after `reflow()` returns.

If keeping "latest command selected after a response" is still desired, add a selector that does not scroll, for example `selectLastVisibleCommand()` or an updated `updateActiveCommand()` that only chooses commands currently visible in the viewport.

### 3. Make active command selection viewport-aware

Revise `updateActiveCommand()` so it does not select commands that are completely outside the visible viewport.

Recommended behavior:

- Look only at command rendered lines between `scrollY` and `scrollY + viewportHeight() - 1`.
- Select the last visible command in the viewport.
- Set `cmdSelectedIdx = -1` when no command is visible.

This prevents the footer from advertising `Enter Apply` for an off-screen command.

### 4. Update footer hint

When a visible command is applyable, the footer should advertise scrolling instead of command navigation:

```text
Enter Apply | Up/Down Scroll | Shift+Tab TTY | Ctrl+T Hide
```

## Test Plan

Add or update sidebar component tests in `pkg/ui/components/sidebar/sidebar_chat_test.go`.

Recommended tests:

- `TestSidebar_MouseWheelScrollsWhenCommandSelectableAfterStreaming`
  - Create scrollable content with a single `<cmd>...</cmd>`.
  - Set `streaming = false`.
  - Ensure input is empty and command selection is enabled.
  - Call `HandleWheel()` for up/down.
  - Assert `scrollY` changes instead of remaining pinned to the command.

- `TestSidebar_MouseWheelScrollsWhileStreamingWithCommands`
  - Create scrollable content with one or more `<cmd>...</cmd>` entries.
  - Set `streaming = true`.
  - Call `HandleWheel()`.
  - Assert `scrollY` changes, preserving the behavior that already works during streaming.
  - This guards the current working state so the fix does not accidentally make streaming behavior worse.

- `TestSidebar_WheelUpBreaksFollowMode`
  - Start with scrollable content and `follow = true` at the bottom.
  - Call `HandleWheel()` with wheel-up.
  - Assert `scrollY` moves upward and `follow == false`.
  - This preserves the existing "user scrolled away from bottom" behavior after the scroll helper split.

- `TestSidebar_PageKeysStillScrollWhenCommandSelectable`
  - Create scrollable content with commands.
  - Ensure input is empty and command selection is enabled.
  - Call `Update()` with `PgUp`/`PgDown`.
  - Assert `scrollY` changes through the existing viewport-scroll branch.

- `TestSidebar_ArrowKeysScrollWhenCommandSelectable`
  - Create scrollable content with commands.
  - Ensure input is empty and command selection is enabled.
  - Call `Update()` with `Up`/`Down`.
  - Assert `scrollY` changes instead of command selection stepping.

- `TestSidebar_RefreshViewFollowStaysAtBottomWithCommandAboveBottom`
  - Create an assistant message with a command followed by enough text to scroll.
  - Set `follow = true`.
  - Call `RefreshView()`.
  - Assert `scrollY == maxScroll()`.
  - If viewport-aware command selection ships, also assert `cmdSelectedIdx` points to the last visible command in the bottom viewport, not necessarily the absolute last command in the full transcript.

- `TestSidebar_UpdateActiveCommandIgnoresOffscreenCommands`
  - Create command lines outside the visible viewport.
  - Call `updateActiveCommand()`.
  - Assert `cmdSelectedIdx == -1`.

Run:

```bash
go test ./pkg/ui/components/sidebar
make check
```

## Acceptance Criteria

- Sidebar mouse wheel scrolling works when assistant messages contain `<cmd>` entries.
- `Up`/`Down` scroll text with command selection enabled.
- Mouse wheel scrolling works both during streaming and after streaming finishes.
- `PgUp`/`PgDown` keep scrolling text with command selection enabled.
- Follow mode stays at the latest chat content during streaming and refresh.
- Command highlighting does not force scroll position during passive refresh or wheel/page scrolling.
- Command execution via Enter still works for a visible selected command.
- Footer hints match the final keyboard behavior.
