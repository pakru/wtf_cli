# Bash History Quick Search - Implementation Tasks

## Overview
Implement a TUI dialog for quick search through bash command history, triggered by `Ctrl+R` shortcut or `/history` command. Replaces bash's native reverse-i-search with a richer experience.

---

## Tasks

### Phase 1: Bash History Reader Utility
- [x] Create `pkg/capture/bash_history.go`
- [x] Read `$HISTFILE` environment variable (fallback to `~/.bash_history`)
- [x] Parse history file (reverse order, most recent first)
- [x] Handle bash timestamps (line-by-line parsing for now)
- [x] Add function to merge with session history and deduplicate
- [x] Add unit tests

**Definition of Done:**
- ✅ `ReadBashHistory(maxLines int) ([]string, error)` function works
- ✅ Uses `$HISTFILE` env var, falls back to `~/.bash_history`
- ✅ `MergeHistory()` deduplicates and preserves order
- ✅ Unit tests pass with >80% coverage (94.9%)
- ✅ `go test ./pkg/capture/... -run BashHistory` passes

---

### Phase 2: History Picker Panel Component
- [x] Create `pkg/ui/components/historypicker/history_picker.go`
- [x] Implement `HistoryPickerPanel` struct with filter, scroll, selection state
- [x] Implement `Show(initialFilter string, commands []string)` with pre-filter
- [x] Implement `Update(msg tea.KeyPressMsg)` for keyboard navigation
- [x] Implement `View() string` with styled rendering
- [x] Add fuzzy/substring matching for filter
- [x] Add unit tests

**Definition of Done:**
- ✅ Picker displays list of commands with current selection highlighted
- ✅ Filter input narrows down list in real-time
- ✅ Arrow keys navigate, PgUp/PgDown for fast scroll
- ✅ Enter emits `HistoryPickerSelectMsg{Command string}`
- ✅ Esc emits `HistoryPickerCancelMsg{}`
- ✅ Unit tests verify navigation, filtering, edge cases (20 tests)
- ✅ `go test ./pkg/ui/components/historypicker/...` passes
- ✅ Test coverage: 89.1%

---

### Phase 3: Input Handler - Ctrl+R Interception
- [x] Add `ctrl+r` case in `HandleKey()` in `pkg/ui/input/input.go`
- [x] Track pre-typed text before `Ctrl+R` for initial filter
- [x] Emit `ShowHistoryPickerMsg{InitialFilter string}`
- [x] Add `historyPickerMode` state flag
- [x] Add unit tests

**Definition of Done:**
- ✅ Pressing `Ctrl+R` triggers history picker instead of sending to PTY
- ✅ Pre-typed text (if any) is passed as initial filter (TODO in Phase 4)
- ✅ `historyPickerMode` prevents key pass-through while picker is open
- ✅ Unit tests verify Ctrl+R handling (3 new tests)
- ✅ `go test ./pkg/ui/input/...` passes

---

### Phase 4: Model Integration
- [x] Add `historyPicker *historypicker.HistoryPickerPanel` to Model
- [x] Initialize picker in `NewModel()`
- [x] Handle `ShowHistoryPickerMsg` in `Update()`:
  - Load bash history + session history
  - Call `historyPicker.Show(initialFilter, commands)`
- [x] Handle `HistoryPickerSelectMsg`:
  - Write selected command to PTY
  - Close picker, update input handler state
- [x] Handle `HistoryPickerCancelMsg`:
  - Close picker, restore focus
- [x] Route keyboard to picker when visible (like palette)
- [x] Add picker overlay to `renderCanvas()`
- [x] Call `SetSize()` on resize

**Definition of Done:**
- History picker appears as centered overlay when triggered
- Selected command is written to PTY prompt
- Esc closes picker without action
- Picker respects window resize
- `go test ./pkg/ui/...` passes

---

### Phase 5: /history Command Update
- [x] Modify `HistoryHandler.Execute()` in `pkg/commands/handlers.go`
- [x] Return a result that triggers picker instead of text output
- [x] Add new result type or field for picker mode

**Definition of Done:**
- Typing `/history` opens the history picker dialog
- Same behavior as `Ctrl+R` with empty initial filter
- Existing tests still pass

---

### Phase 6: Manual Testing
- [ ] Test `Ctrl+R` on empty prompt
- [ ] Test `Ctrl+R` after typing partial command (pre-filter)
- [ ] Test `/history` command from palette
- [ ] Test arrow key navigation in long history
- [ ] Test filter matching (case-insensitive, substring)
- [ ] Test Enter selection → command appears in prompt
- [ ] Test Esc cancellation
- [ ] Test with empty history
- [ ] Test with very large history (1000+ entries)

**Definition of Done:**
- All manual tests work as expected
- No terminal corruption
- No crash on edge cases

---

## Success Criteria
- [x] `Ctrl+R` opens history picker with pre-typed text as filter
- [x] `/history` command opens the same picker
- [x] Filtering is fast and responsive
- [x] Selected command transfers to PTY prompt
- [ ] Feature works on both macOS and Linux
- [ ] All existing tests still pass (`make test`)
