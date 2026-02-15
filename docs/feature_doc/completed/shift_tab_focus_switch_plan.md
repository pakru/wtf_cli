# Shift+Tab Focus Switch Between Analysis Window and Terminal (Issue #28)

Add `Shift+Tab` to toggle focus between the chat analysis sidebar and the terminal. Only one cursor visible at a time.

## Proposed Changes

### Input Handler

#### [MODIFY] [input.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/input/input.go)

1. **Add `FocusSwitchMsg`** — empty struct message type
2. **Add `"shift+tab"` case** in `HandleKey` switch — emit `FocusSwitchMsg`
3. **Add `"shift+tab"` case** in `sendKeyToPTY` — write `\x1b[Z` (CSI Z = reverse tab) for fullscreen/secret passthrough

---

### UI Model — Focus State & Routing

#### [MODIFY] [model.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go)

1. **Add `terminalFocused bool` field** (default `true`)

2. **Add private helper `setTerminalFocused(focused bool)`** that:
   - **Idempotent:** early return if `m.terminalFocused == focused`
   - **Guard:** only manipulate sidebar focus when sidebar is visible + chat mode
   - Sets `m.terminalFocused = focused`
   - Calls `m.viewport.SetCursorVisible(focused)` to show/hide terminal cursor
   - **Deterministic sidebar focus** (not `ToggleFocus`):
     - `focused == true` → `sidebar.BlurInput()` (new method, sets `FocusViewport` + blurs textarea)
     - `focused == false` → `sidebar.FocusInput()` (existing method)

#### [MODIFY] [sidebar.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/sidebar/sidebar.go)

3. **Add `BlurInput()` method** — sets `focused = FocusViewport`, calls `textarea.Blur()`. Counterpart to existing `FocusInput()`.

4. **Handle `input.FocusSwitchMsg`** in `Update()`:
   - **Guard:** If any overlay is visible (settings/palette/history/result/pickers) → no-op
   - If sidebar **visible and in chat mode** → call `setTerminalFocused(!m.terminalFocused)`
   - If sidebar **not visible** → open sidebar in chat mode (like Ctrl+T), call `setTerminalFocused(false)`

5. **Sync `terminalFocused` on all sidebar-open paths** to prevent drift:
   - `ToggleChatMsg` open (line 468) → add `m.setTerminalFocused(false)` 
   - `ResultActionToggleChat` open (line 520) → add `m.setTerminalFocused(false)`
   - Streaming handler open (line 538) → add `m.setTerminalFocused(false)`
   - All three close paths → add `m.setTerminalFocused(true)`
   
   > Ctrl+T open continues to focus chat input immediately (existing `FocusInput()` call preserved).

6. **Detect sidebar close from Esc in chat mode** — after Priority 3 sidebar chat routing (line 418), check `!m.sidebar.IsVisible()`. If sidebar was just closed by Esc, call `m.setTerminalFocused(true)` + `m.applyLayout()`.

7. **Update key routing** in `tea.KeyPressMsg`:
   - **Intercept `shift+tab` before sidebar/PTY routing** (after overlay checks, before Priority 3) — emit `FocusSwitchMsg`
   - When `terminalFocused == true` and sidebar visible in chat mode → skip sidebar key handling, fall through to PTY input handler

---

### Viewport Cursor Visibility

#### [MODIFY] [viewport.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/viewport/viewport.go)

1. **Add `showCursor bool` field** (default `true`)
2. **Add `SetCursorVisible(visible bool)` method** — sets flag and **immediately re-renders** cursor overlay on current content (not waiting for new PTY output)
3. **In `AppendOutput`**: pass `"█"` or `""` to `RenderCursorOverlay()` based on `showCursor`

---

### Welcome Message & Help

#### [MODIFY] [welcome.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/welcome/welcome.go)

Add `{"Shift+Tab", "Switch focus to chat panel"}` to shortcuts list.

#### [MODIFY] [handlers.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/commands/handlers.go)

Add `Shift+Tab  - Switch focus to chat panel` to `/help` shortcut list (line 403).

## Verification Plan

### Automated Tests

**New tests:**

1. **`TestInputHandler_HandleKey_ShiftTab`** — emits `FocusSwitchMsg`; bypassed in fullscreen (writes `\x1b[Z`); bypassed in secret mode
2. **`TestModel_FocusSwitch_ShiftTab`** — toggles focus state, cursor visibility, sidebar focus; no-op when overlay visible; opens sidebar if not visible; Ctrl+T open syncs `terminalFocused=false`; Esc close resets `terminalFocused=true`

```bash
cd /home/dev/project/wtf_cli/wtf_cli && go test ./... -count=1
```
