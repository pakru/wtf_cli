# Mouse Click Focus Switching

## Summary

Add mouse-based focus switching between the terminal viewport and the chat sidebar. Left-clicking the terminal focuses shell input; left-clicking anywhere in the visible sidebar focuses the sidebar chat input. Existing drag selection, mouse wheel scrolling, keyboard focus switching, and full-screen passthrough must keep working.

## Implementation Changes

- In `pkg/ui/model.go`, update `tea.MouseClickMsg` handling:
  - Ignore blocking overlays, full-screen mode, non-left clicks, status bar clicks, and out-of-bounds clicks.
  - When sidebar is visible, use `splitSidebarWidths(m.width)` to classify the click as terminal pane or sidebar pane.
  - Terminal-pane click calls `m.setTerminalFocused(true)`, clears sidebar selection, and preserves existing viewport selection start.
  - Sidebar-pane click calls `m.setTerminalFocused(false)` and `m.sidebar.FocusInput()` so any sidebar click deterministically focuses chat input, even if sidebar focus was previously on its message viewport.
  - Preserve existing sidebar text selection by still calling `SelectionPoint()` and `StartSelection()` when the click lands in selectable message content.

- Keep routing behavior unchanged elsewhere:
  - Mouse wheel remains focus-based.
  - Mouse drag/release selection continues to auto-copy.
  - `Shift+Tab`, `Ctrl+T`, paste routing, scroll mode, and full-screen input passthrough remain unchanged.

## Test Plan

- Add tests in `pkg/ui/model_test.go`:
  - Click terminal pane while sidebar is focused: `terminalFocused=true`, sidebar input blurred, terminal selection can still start.
  - Click sidebar message area while terminal is focused: `terminalFocused=false`, sidebar input focused, terminal cursor hidden, sidebar selection can still start.
  - Click sidebar chrome/non-selectable area: focuses sidebar input without starting selection.
  - Click status bar: focus does not change.
  - Click while a blocking overlay is visible: focus does not change.
  - Right-click does not change focus.

- Run:
  - `make check`

## Assumptions

- "Konsole" means the terminal viewport inside `wtf_cli`, not the KDE Konsole application window.
- Mouse click focus switching only applies when the sidebar is already visible.
- Sidebar clicks always focus the chat input, per selected behavior.
