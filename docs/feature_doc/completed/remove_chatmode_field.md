# Remove Obsolete `chatMode` Field from Sidebar (Issue #30)

The sidebar always operates in chat mode — every code path that opens the sidebar calls `EnableChatMode()` first. The `chatMode` bool and its ~30 guard checks across `sidebar.go` and `model.go` are dead branching that adds complexity with no functional purpose. This refactoring removes the field and simplifies the code.

## Proposed Changes

### Sidebar Component

#### [MODIFY] [sidebar.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/sidebar/sidebar.go)

1. **Remove `chatMode` field** from `Sidebar` struct (L47)
2. **Move textarea initialization** from `EnableChatMode()` into `NewSidebar()` — the sidebar should be ready for chat from creation
3. **Remove `EnableChatMode()` method** (L367–377) — no longer needed
4. **Remove `IsChatMode()` method** (L379–382) — always returned `true`
5. **Simplify `Show()`** (L59–72) — remove `if s.chatMode && len(s.messages) > 0` guard, keep `RefreshView()` logic based solely on `len(s.messages) > 0`
6. **Simplify `ShouldHandleKey()`** (L105–141) — remove `if s.chatMode` check at L112; the chat-mode key handling becomes the only path. The "Non-chat mode" block (L133–140) becomes dead code and is removed
7. **Simplify `Update()`** (L143–193) — remove `if s.chatMode` check at L150; chat-mode input handling becomes the default. The "Regular sidebar navigation" block (L177–192) is removed
8. **Simplify `View()`** (L232–288) — remove `if s.chatMode` branch at L242; always call `renderChatView()`. Delete the non-chat rendering block (L246–287)
9. **Simplify `RefreshView()`** (L490–498) — remove `if s.chatMode` guard
10. **Simplify `maxScroll()`** (L577–596) — remove the `if s.chatMode` branch; always use the chat calculation

---

### Main Model

#### [MODIFY] [model.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go)

1. **Remove all `EnableChatMode()` calls** — 4 locations (L502, L528, L581, L600)
2. **Remove all `IsChatMode()` checks** — ~14 locations. Simplify conditions like `m.sidebar.IsVisible() && m.sidebar.IsChatMode()` → `m.sidebar.IsVisible()`
3. **Remove dead "Priority 4" block** (L460–470) — this non-chat sidebar key handling path is never reached
4. **Remove dead non-chat streaming path** (L999–1045) — the `else` branch in `wtfStreamMsg` handling that uses `SetContent(m.wtfContent)` is unreachable. Keep only the chat-mode branch that uses `UpdateLastMessage()`/`RefreshView()`
5. **Remove non-chat branch in `streamThrottleFlushMsg`** (L1058–1060) — simplify to always call `RefreshView()`
6. **Remove `wtfContent` field** (L74) — only used by the dead non-chat streaming path. Also remove any remaining references to `m.wtfContent` (L608, L621, L1033)

---

### Tests

#### [MODIFY] [sidebar_chat_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/sidebar/sidebar_chat_test.go)

1. **Delete `TestSidebar_ChatMode`** (L13–26) — tests the removed `IsChatMode()`/`EnableChatMode()` API
2. **Remove `s.EnableChatMode()` calls** from all remaining tests (~20 occurrences) — `NewSidebar()` will now initialize the textarea automatically
3. **Add/verify model-level tests** for key routing and focus management:
   - Confirm keys route to sidebar `Update()` (not PTY) when sidebar is visible and terminal is unfocused ([setTerminalFocused](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go#L1232-L1247))
   - Confirm `setTerminalFocused` correctly blurs/focuses sidebar input after `IsChatMode()` guard removal
   - Confirm chat sidebar key routing at [Priority 3 block](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/model.go#L441-L458) still intercepts keys without `IsChatMode()` check

## Verification Plan

### Automated Tests

```bash
# Full pre-commit validation: fmt + vet + build + test
cd /home/dev/project/wtf_cli/wtf_cli && make check
```
