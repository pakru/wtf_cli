# Status Bar Enhancement - Full Width with Smart Path Truncation

Enhance the `wtf_cli` status bar to automatically adjust to full screen width with proper alignment: current directory path on the left, LLM model and "Press / for command" on the right. Long paths should be truncated intelligently in the middle (e.g., `/home/../current/dir`).

## Current State

**Current format:**
```
[wtf_cli] {path} | [llm]: {model} | Press / for commands
```

**Issues:**
- All content concatenated as a single string
- Simple end-truncation when path is too long
- No left/right alignment layout

---

## Proposed Changes

### Status Bar Component

#### [MODIFY] [statusbar_view.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/statusbar_view.go)

**New layout structure:**

```
┌─────────────────────────────────────────────────────────────────────────┐
│ [wtf_cli] /home/../current/dir        [llm]: model | Press / for commands │
│ └────── LEFT ALIGNED ──────┘          └────────── RIGHT ALIGNED ─────────┘│
└─────────────────────────────────────────────────────────────────────────┘
```

**Changes:**
1. **Add `truncatePath()` helper function** - Smart middle-truncation:
   - If path fits available space, show full path
   - Otherwise, preserve first segment + last 2 segments + ellipsis
   - Example: `/home/user/projects/wtf_cli/pkg/ui/components` → `/home/../pkg/ui/components`

2. **Redesign `Render()` method:**
   - Build right section first: `[llm]: {model} | Press / for commands`
   - Calculate remaining width for left section
   - Build left section: `[wtf_cli] {truncated_path}`
   - Fill middle with spaces for full-width bar
   - Apply lipgloss styling across entire width

3. **Message mode adjustment:**
   - When message is set, show: `[wtf_cli] {message}` (left) + `[llm]: {model}` (right)

---

#### [MODIFY] [statusbar.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/statusbar.go)

Apply the same layout logic changes for consistency with `StatusBarView`. This ensures both the direct ANSI-based statusbar and the lipgloss-based view use the same layout.

---

## Verification Plan

### Automated Tests

**Run existing statusbar tests to check for regressions:**
```bash
cd /home/dev/project/wtf_cli/wtf_cli && go test ./pkg/ui/components/statusbar/... -v
```

**New tests to add in [statusbar_view_test.go](file:///home/dev/project/wtf_cli/wtf_cli/pkg/ui/components/statusbar/statusbar_view_test.go):**
- `TestTruncatePath_ShortPath` - Verify short paths are not truncated
- `TestTruncatePath_LongPath` - Verify long paths get middle-truncation
- `TestStatusBarView_LayoutAlignment` - Verify left/right alignment works
- `TestStatusBarView_FullWidth` - Verify status bar fills exact terminal width

### Manual Verification

After implementation, run `wtf_cli` and verify:
1. Status bar spans the full terminal width
2. Path appears on the left side
3. LLM model and "Press / for commands" appear on the right side
4. Resize terminal to test:
   - Wide terminal: full path visible
   - Narrow terminal: path truncates with `..` in the middle
5. `cd` to a very long path and verify truncation looks like `/home/../current/dir`
