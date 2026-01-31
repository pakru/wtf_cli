# Chat Panel Scroll Without Focus Switch

Enable Up/Down arrow keys to always scroll the chat panel viewport, eliminating the need to press Tab to switch focus between the input area and viewport.

## Current State

**Current behavior:**
1. When chat panel is open, focus defaults to the text input (`FocusInput`)
2. Up/Down keys are routed to the textarea component (cursor movement within input)
3. User must press **Tab** to switch focus to viewport (`FocusViewport`)
4. Only then can Up/Down keys scroll the message history
5. User must press **Tab** again to return to input for typing

**Code flow (current):**
```
User presses Up/Down
  → model.go (line 397): sidebar.Update(msg)
  → sidebar.go (line 150): chatMode && focused == FocusInput
  → sidebar.go (line 167-171): Route to textarea.Update(msg)
  → Textarea handles Up/Down for cursor movement
```

**Issues:**
- Requires Tab to toggle focus just to scroll
- Disrupts workflow when reviewing long AI responses
- Unintuitive UX - users expect Up/Down to scroll visible content

---

## Proposed Changes

### Overview

Intercept Up/Down/PgUp/PgDown keys when focus is on input and route them directly to viewport scrolling, bypassing the textarea component.

---

### [MODIFY] [sidebar.go](pkg/ui/components/sidebar/sidebar.go)

#### Change 1: Update `Update()` method (lines 143-223)

**Current code (lines 149-173):**
```go
// Chat mode: handle input focus
if s.chatMode && s.focused == FocusInput {
    switch msg.String() {
    case "enter":
        // ... submit handling
    case "esc":
        // ... toggle focus
    default:
        // Route to textarea
        var cmd tea.Cmd
        s.textarea, cmd = s.textarea.Update(msg)
        return cmd
    }
}
```

**Modified code:**
```go
// Chat mode: handle input focus
if s.chatMode && s.focused == FocusInput {
    switch msg.String() {
    case "enter":
        // ... submit handling (unchanged)
    case "esc":
        // ... toggle focus (unchanged)
    case "up", "down", "pgup", "pgdown":
        // Route scroll keys to viewport, NOT textarea
        return s.handleScroll(msg.String())
    default:
        // Route other keys to textarea
        var cmd tea.Cmd
        s.textarea, cmd = s.textarea.Update(msg)
        return cmd
    }
}
```

#### Change 2: Extract scroll logic to helper method

**Add new method after `Update()`:**
```go
// handleScroll processes scroll key events and returns nil command.
func (s *Sidebar) handleScroll(key string) tea.Cmd {
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

    return nil
}
```

#### Change 3: Simplify viewport focus handling (lines 175-212)

**Refactor to reuse `handleScroll()`:**
```go
// Regular sidebar navigation (non-chat or viewport focused)
keyStr := msg.String()

switch keyStr {
case "esc":
    s.Hide()
    return nil
case "up", "down", "pgup", "pgdown":
    return s.handleScroll(keyStr)
case "q":
    s.Hide()
    return nil
case "y":
    return s.copyToClipboard()
}

return nil
```

---

### [MODIFY] [sidebar.go](pkg/ui/components/sidebar/sidebar.go) - Footer label

#### Change 4: Update footer label (line 23)

**Current:**
```go
sidebarFooterLabel = "Up/Down Scroll | y Copy | Esc/q Close"
```

**Modified (for chat mode):**
Consider adding a chat-specific footer or making it dynamic:
```go
sidebarFooterLabelNormal = "Up/Down Scroll | y Copy | Esc/q Close"
sidebarFooterLabelChat   = "Up/Down Scroll | Enter Send | Esc Focus | Tab Toggle"
```

**Note:** This is optional and can be implemented in a follow-up if desired.

---

## Behavior After Changes

**New flow:**
```
User presses Up/Down (with input focused)
  → model.go (line 397): sidebar.Update(msg)
  → sidebar.go: chatMode && focused == FocusInput
  → sidebar.go: case "up", "down" → handleScroll()
  → Viewport scrolls, focus stays on input
  → User continues typing without Tab switching
```

**Key mappings in chat mode (input focused):**

| Key | Action |
|-----|--------|
| Up | Scroll viewport up 1 line |
| Down | Scroll viewport down 1 line |
| PgUp | Scroll viewport up 10 lines |
| PgDown | Scroll viewport down 10 lines |
| Enter | Submit message |
| Esc | Toggle focus to viewport |
| Tab | Toggle focus to viewport |
| Other | Route to textarea (typing) |

**Key mappings in chat mode (viewport focused):**

| Key | Action |
|-----|--------|
| Up/Down | Scroll 1 line |
| PgUp/PgDown | Scroll 10 lines |
| Esc/q | Close sidebar |
| y | Copy to clipboard |
| Tab | Toggle focus to input |

---

## Impact Analysis

### Trade-offs

**Pros:**
- Eliminates Tab-switching friction for scrolling
- More intuitive UX matching expectations
- Single-hand operation (no Tab required)

**Cons:**
- Textarea loses Up/Down for multi-line cursor navigation
- Users expecting textarea cursor movement may be surprised

**Mitigation:**
- Chat input is typically single-line; multi-line cursor nav rarely needed
- Tab still available for explicit viewport focus if needed
- Could add Ctrl+Up/Ctrl+Down for textarea cursor movement if users request it

---

## Files Changed Summary

| File | Type | Description |
|------|------|-------------|
| [sidebar.go](pkg/ui/components/sidebar/sidebar.go) | MODIFY | Intercept scroll keys in input focus mode, extract `handleScroll()` helper |
| [sidebar_chat_test.go](pkg/ui/components/sidebar/sidebar_chat_test.go) | MODIFY | Add scroll/follow behavior tests for input-focused mode |

---

## Verification Plan

### Automated Tests

**Run existing sidebar tests:**
```bash
cd /home/dev/project/wtf_cli/wtf_cli && go test ./pkg/ui/components/sidebar/... -v
```

**New test cases to add in [sidebar_chat_test.go](pkg/ui/components/sidebar/sidebar_chat_test.go):**

Uses existing `testutils` package for key event creation.

**Required import:**
```go
import (
    "strings"
    "testing"

    "wtf_cli/pkg/ai"
    "wtf_cli/pkg/ui/components/testutils"
)
```

#### Test 1: `TestChatMode_UpDownScrollsViewport_InputFocused`

Asserts that Up/Down keys scroll the viewport (change `scrollY`) while `focused == FocusInput` and textarea content remains unchanged.

```go
func TestChatMode_UpDownScrollsViewport_InputFocused(t *testing.T) {
    s := NewSidebar()
    s.SetSize(40, 20)
    s.EnableChatMode()
    s.Show("Chat", "")

    // Add enough content to enable scrolling
    s.AppendUserMessage("Question")
    s.StartAssistantMessage()
    longContent := strings.Repeat("Line\n", 50) // 50 lines
    s.UpdateLastMessage(longContent)
    s.RefreshView()

    // Set textarea content to verify it won't change
    s.textarea.SetValue("my input")
    initialValue := s.textarea.Value()

    // Ensure focus on input
    s.FocusInput()
    if !s.IsFocusedOnInput() {
        t.Fatal("Expected focus on input")
    }

    // Record initial scroll position (should be at bottom due to follow)
    initialScrollY := s.scrollY

    // Press Up - should scroll viewport up
    s.Update(testutils.TestKeyUp)

    if s.scrollY >= initialScrollY {
        t.Errorf("Expected scrollY to decrease, got before=%d after=%d", initialScrollY, s.scrollY)
    }

    // Verify textarea unchanged
    if s.textarea.Value() != initialValue {
        t.Errorf("Expected textarea unchanged %q, got %q", initialValue, s.textarea.Value())
    }

    // Verify still focused on input
    if !s.IsFocusedOnInput() {
        t.Error("Expected focus to remain on input after Up key")
    }
}
```

#### Test 2: `TestChatMode_DownScrollsViewport_InputFocused`

```go
func TestChatMode_DownScrollsViewport_InputFocused(t *testing.T) {
    s := NewSidebar()
    s.SetSize(40, 20)
    s.EnableChatMode()
    s.Show("Chat", "")

    // Add content and scroll to top
    s.AppendUserMessage("Q")
    s.StartAssistantMessage()
    s.UpdateLastMessage(strings.Repeat("Line\n", 50))
    s.RefreshView()
    s.scrollY = 0 // Force scroll to top

    s.textarea.SetValue("test")
    initialValue := s.textarea.Value()
    s.FocusInput()

    initialScrollY := s.scrollY

    // Press Down
    s.Update(testutils.TestKeyDown)

    if s.scrollY <= initialScrollY {
        t.Errorf("Expected scrollY to increase, got before=%d after=%d", initialScrollY, s.scrollY)
    }

    if s.textarea.Value() != initialValue {
        t.Errorf("Textarea content changed unexpectedly")
    }
}
```

#### Test 3: `TestChatMode_PgUpPgDownScrollsViewport_InputFocused`

```go
func TestChatMode_PgUpPgDownScrollsViewport_InputFocused(t *testing.T) {
    s := NewSidebar()
    s.SetSize(40, 30)
    s.EnableChatMode()
    s.Show("Chat", "")

    s.AppendUserMessage("Q")
    s.StartAssistantMessage()
    s.UpdateLastMessage(strings.Repeat("Line\n", 100))
    s.RefreshView()

    // Start in middle
    s.scrollY = 50
    s.FocusInput()

    // PgDown should jump ~10 lines
    beforePgDown := s.scrollY
    s.Update(testutils.TestKeyPgDown)
    if s.scrollY != beforePgDown+10 {
        t.Errorf("Expected scrollY=%d after PgDown, got %d", beforePgDown+10, s.scrollY)
    }

    // PgUp should jump back
    beforePgUp := s.scrollY
    s.Update(testutils.TestKeyPgUp)
    if s.scrollY != beforePgUp-10 {
        t.Errorf("Expected scrollY=%d after PgUp, got %d", beforePgUp-10, s.scrollY)
    }
}
```

#### Test 4: `TestChatMode_TextInputStillWorks_InputFocused`

```go
func TestChatMode_TextInputStillWorks_InputFocused(t *testing.T) {
    s := NewSidebar()
    s.SetSize(40, 20)
    s.EnableChatMode()
    s.Show("Chat", "")
    s.FocusInput()

    // Type some characters
    s.Update(testutils.NewTextKeyPressMsg("h"))
    s.Update(testutils.NewTextKeyPressMsg("i"))

    if !strings.Contains(s.textarea.Value(), "h") {
        t.Error("Expected textarea to receive text input")
    }
}
```

#### Test 5: `TestChatMode_FollowDisabledOnScroll`

```go
func TestChatMode_FollowDisabledOnScroll(t *testing.T) {
    s := NewSidebar()
    s.SetSize(40, 20)
    s.EnableChatMode()
    s.Show("Chat", "")

    s.AppendUserMessage("Q")
    s.StartAssistantMessage()
    s.UpdateLastMessage(strings.Repeat("Line\n", 50))
    s.RefreshView()

    // follow should be true initially (at bottom)
    s.FocusInput()

    // Scroll up should disable follow
    s.Update(testutils.TestKeyUp)
    if s.follow {
        t.Error("Expected follow=false after scrolling up")
    }
}
```

### Manual Verification

After implementation:
1. Run `wtf_cli` and press `Ctrl+T` to open chat
2. Type a question and press Enter
3. Wait for AI response (long response preferred)
4. **Verify:** Press Up/Down arrows - viewport should scroll
5. **Verify:** Type characters - they appear in input (no focus lost)
6. **Verify:** Press Tab - focus toggles to viewport
7. **Verify:** Press Esc in viewport - sidebar closes
8. **Verify:** Press Tab again - focus returns to input

---

## Implementation Order

1. Add new test cases to `sidebar_chat_test.go` (tests will fail initially)
2. Extract `handleScroll()` helper method from existing scroll logic
3. Add `case "up", "down", "pgup", "pgdown"` to input-focus branch
4. Refactor viewport-focus branch to use `handleScroll()`
5. Run tests - all should pass now
6. Manual verification
7. (Optional) Update footer label for chat mode
