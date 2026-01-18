# Full-Screen Terminal App Support - Implementation Tasks

## Overview
Enable running full-screen terminal applications (vim, nano, htop) inside wtf_cli without breaking terminal output or polluting the LLM buffer.

---

## Tasks

### Phase 1: Add Midterm Dependency
- [x] Add `github.com/vito/midterm` to `go.mod`
- [x] Run `go mod tidy`
- [x] Verify builds successfully

**Definition of Done:**
- `go build ./...` completes without errors
- `go.sum` contains midterm package

---

### Phase 2: Alternate Screen Detection
- [x] Create `pkg/ui/altscreen.go` with detection functions
- [x] Detect `ESC[?1049h` (smcup) - entering alternate screen
- [x] Detect `ESC[?1049l` (rmcup) - exiting alternate screen
- [x] Handle split sequences across read boundaries
- [x] Add unit tests for detection

**Definition of Done:**
- `DetectAltScreen(data []byte) (entering, exiting bool)` function exists
- Unit tests pass for: smcup only, rmcup only, both, split across chunks
- Test coverage > 80%

---

### Phase 3: Full-Screen Panel Component
- [x] Create `pkg/ui/fullscreen_panel.go`
- [x] Initialize midterm terminal emulator
- [x] Implement `Write(data []byte)` to process PTY output
- [x] Implement `View() string` to render buffer to Bubble Tea
- [x] Implement `Resize(width, height int)`
- [x] Add Show/Hide methods
- [x] Add unit tests

**Definition of Done:**
- FullScreenPanel struct with all methods implemented
- `View()` returns string renderable by Bubble Tea
- Unit tests verify write/view/resize behavior

---

### Phase 4: Model Integration
- [x] Add `fullScreenMode bool` state to Model
- [x] Add `fullScreenPanel *FullScreenPanel` to Model
- [x] On smcup detection: switch to full-screen mode
- [x] On rmcup detection: restore normal mode
- [x] Route PTY output based on mode

**Definition of Done:**
- Model correctly switches between normal and full-screen modes
- PTY output routed to correct component based on mode
- Status bar hidden when in full-screen mode

---

### Phase 5: Input Bypass
- [x] Modify `input.go` to check full-screen mode
- [x] In full-screen mode: disable `/` palette trigger
- [x] In full-screen mode: disable all custom shortcuts
- [x] Route ALL keyboard input directly to PTY
- [x] Add unit tests

**Definition of Done:**
- Pressing `/` in full-screen mode sends `/` to PTY (not palette)
- Ctrl+D in full-screen mode sends to PTY (not exit prompt)
- All keys in full-screen mode go directly to PTY

---

### Phase 6: Buffer Isolation
- [x] In full-screen mode: skip writing to circular buffer
- [x] In normal mode: continue writing to buffer for LLM context
- [x] Add test to verify buffer is not polluted

**Definition of Done:**
- After running vim and exiting, buffer contains no escape sequences
- `/wtf` command receives clean context without vim garbage

---

### Phase 7: Testing
- [ ] Manual test with `vim`
- [ ] Manual test with `nano`
- [ ] Manual test with `htop`
- [ ] Manual test with `less`
- [ ] Manual test with `man`
- [ ] Verify `/wtf` command still works after exiting full-screen app
- [ ] Verify status bar reappears after exit

**Definition of Done:**
- All 5 apps work correctly inside wtf_cli
- No terminal corruption after exiting any app
- Status bar visible after exiting each app

---

## Success Criteria
- [ ] vim opens and works correctly inside wtf_cli
- [ ] All vim keybindings work (no interference from wtf_cli)
- [ ] Exiting vim restores normal wtf_cli UI
- [ ] LLM buffer contains only shell commands, not vim garbage
