# Status Bar Enhancement - Implementation Tasks

**Feature:** Full-width status bar with left/right alignment and smart path truncation  
**Related Plan:** [statusbar_enhancement_plan.md](./statusbar_enhancement_plan.md)

---

## Task 1: Create `truncatePath` Helper Function

**Goal:** Implement smart middle-truncation for directory paths that preserves readability.

**File:** `pkg/ui/components/statusbar/statusbar_view.go`

**Implementation Details:**
- Create function: `func truncatePath(path string, maxWidth int) string`
- If `len(path) <= maxWidth`, return path unchanged
- Split path by `/` separator
- Preserve: first segment (e.g., `/home` or `~`) + `..` + last 2-3 segments
- Adjust number of trailing segments to fit within `maxWidth`
- Handle edge cases: root path `/`, home `~`, single segment paths

**Examples:**
| Input | Max Width | Output |
|-------|-----------|--------|
| `/home/user/projects/wtf_cli/pkg/ui` | 30 | `/home/../wtf_cli/pkg/ui` |
| `/home/user/a` | 50 | `/home/user/a` (no truncation) |
| `~/projects/very/long/nested/path` | 25 | `~/../nested/path` |

**Definition of Done:**
- [x] Function implemented in `statusbar_view.go`
- [x] Unit test `TestTruncatePath_ShortPath` passes - verifies no truncation for short paths
- [x] Unit test `TestTruncatePath_LongPath` passes - verifies middle truncation works
- [x] Unit test `TestTruncatePath_EdgeCases` passes - handles `/`, `~`, empty string

---

## Task 2: Redesign Status Bar Layout in `StatusBarView`

**Goal:** Change from single concatenated string to left/right aligned layout.

**File:** `pkg/ui/components/statusbar/statusbar_view.go`

**Implementation Details:**

1. **Build right section first:**
   ```go
   rightContent := fmt.Sprintf("[llm]: %s | Press / for commands", modelLabel)
   rightWidth := ansi.StringWidth(rightContent)
   ```

2. **Calculate available width for left section:**
   ```go
   leftMaxWidth := s.width - rightWidth - 2  // 2 for minimum gap
   ```

3. **Build left section with truncated path:**
   ```go
   truncatedPath := truncatePath(s.currentDir, leftMaxWidth - len("[wtf_cli] "))
   leftContent := fmt.Sprintf("[wtf_cli] %s", truncatedPath)
   ```

4. **Calculate middle padding:**
   ```go
   gap := s.width - ansi.StringWidth(leftContent) - rightWidth
   padding := strings.Repeat(" ", gap)
   ```

5. **Combine and style:**
   ```go
   fullContent := leftContent + padding + rightContent
   return statusStyle.Width(s.width).Render(fullContent)
   ```

6. **Message mode:** When `s.message != ""`, replace path with message in left section

**Definition of Done:**
- [x] `Render()` method produces left-aligned path section
- [x] `Render()` method produces right-aligned model + hint section
- [x] Status bar fills exact terminal width (no overflow, no underflow)
- [x] Existing test `TestStatusBarView_Render` passes
- [x] New test `TestStatusBarView_LayoutAlignment` passes

---

## Task 3: Update `StatusBar` (ANSI-based) for Consistency

**Goal:** Apply same layout logic to the direct ANSI-based `StatusBar` struct.

**File:** `pkg/ui/components/statusbar/statusbar.go`

**Implementation Details:**
- Add `truncatePath` function (or import from shared location)
- Update `Render()` method to use left/right alignment
- Maintain ANSI escape sequence output format

**Definition of Done:**
- [x] `StatusBar.Render()` produces same layout as `StatusBarView.Render()`
- [x] Existing test `TestStatusBarRender` passes
- [x] ANSI positioning still works correctly (cursor save/restore)

---

## Task 4: Add Comprehensive Unit Tests

**Goal:** Ensure all new functionality is properly tested.

**File:** `pkg/ui/components/statusbar/statusbar_view_test.go`

**New Tests to Add:**

| Test Name | Purpose |
|-----------|---------|
| `TestTruncatePath_ShortPath` | Path shorter than max width → no change |
| `TestTruncatePath_LongPath` | Path longer than max width → middle truncation |
| `TestTruncatePath_EdgeCases` | Root `/`, home `~`, empty, single segment |
| `TestStatusBarView_LayoutAlignment` | Left content on left, right content on right |
| `TestStatusBarView_FullWidth` | Output width equals exactly `s.width` |
| `TestStatusBarView_NarrowTerminal` | Path truncates gracefully at 40 char width |

**Definition of Done:**
- [x] All new tests implemented
- [x] All tests pass: `go test ./pkg/ui/components/statusbar/... -v`
- [x] No regressions in existing tests

---

## Task 5: Integration Testing & Manual Verification

**Goal:** Verify the enhancement works correctly in the running application.

**Steps:**
1. Build and run `wtf_cli`
2. Verify status bar spans full terminal width
3. Verify path is on the left, model + hint on the right
4. Test path truncation:
   ```bash
   cd /home/user/very/long/deeply/nested/directory/path/here
   ```
5. Resize terminal window and verify layout adapts
6. Test message display (trigger an action that shows a message)

**Definition of Done:**
- [ ] Status bar visually correct at 80 columns
- [ ] Status bar visually correct at 120 columns
- [ ] Status bar visually correct at 40 columns (narrow)
- [ ] Path truncation shows `..` in middle, not end
- [ ] No visual artifacts or misalignment
- [x] Application builds without errors: `make build`

---

## Summary Checklist

- [x] Task 1: `truncatePath` helper function
- [x] Task 2: `StatusBarView` layout redesign
- [x] Task 3: `StatusBar` consistency update
- [x] Task 4: Unit tests
- [ ] Task 5: Integration testing
