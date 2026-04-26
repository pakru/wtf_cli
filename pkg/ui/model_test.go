package ui

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/ui/components/historypicker"
	"wtf_cli/pkg/ui/components/palette"
	"wtf_cli/pkg/ui/components/picker"
	"wtf_cli/pkg/ui/components/settings"
	"wtf_cli/pkg/ui/components/sidebar"
	"wtf_cli/pkg/ui/components/testutils"
	"wtf_cli/pkg/ui/input"
	"wtf_cli/pkg/ui/terminal"
	"wtf_cli/pkg/updatecheck"

	tea "charm.land/bubbletea/v2"
	cpty "github.com/creack/pty"
)

func TestNewModel(t *testing.T) {
	buf := buffer.New(100)
	sess := capture.NewSessionContext()

	m := NewModel(nil, buf, sess, nil)

	if m.buffer == nil {
		t.Error("Expected buffer to be set")
	}

	if m.session == nil {
		t.Error("Expected session to be set")
	}

	if m.currentDir == "" {
		t.Error("Expected currentDir to be set")
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	cmd := m.Init()
	if cmd == nil {
		t.Error("Expected Init() to return a command")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	// Send window size message (using actual Bubble Tea type)
	newModel, _ := m.Update(tea.WindowSizeMsg{
		Width:  80,
		Height: 24,
	})

	updated := newModel.(Model)

	if updated.width != 80 {
		t.Errorf("Expected width 80, got %d", updated.width)
	}

	if updated.height != 24 {
		t.Errorf("Expected height 24, got %d", updated.height)
	}

	if !updated.ready {
		t.Error("Expected ready to be true after window size")
	}

	// Viewport should be sized (height - 1 for status bar)
	if updated.viewport.Viewport.Height() != 23 {
		t.Errorf("Expected viewport height 23, got %d", updated.viewport.Viewport.Height())
	}
}

func TestModel_MouseDragCopiesViewportSelection(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	m = newModel.(Model)
	m.viewport.Clear()
	m.viewport.SetCursorVisible(false)
	m.viewport.AppendOutput([]byte("alpha\nbravo\ncharlie"))

	newModel, cmd := m.Update(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 0, Button: tea.MouseLeft}))
	if cmd != nil {
		t.Fatal("expected no command on mouse click")
	}
	m = newModel.(Model)
	if !m.viewport.HasActiveSelection() {
		t.Fatal("expected click to start viewport selection")
	}

	newModel, _ = m.Update(tea.MouseMotionMsg(tea.Mouse{X: 3, Y: 1, Button: tea.MouseLeft}))
	m = newModel.(Model)

	newModel, cmd = m.Update(tea.MouseReleaseMsg(tea.Mouse{X: 3, Y: 1, Button: tea.MouseLeft}))
	m = newModel.(Model)

	if cmd == nil {
		t.Fatal("expected copy command on mouse release")
	}
	if got := m.statusBar.GetMessage(); got != selectedTextCopiedMessage {
		t.Fatalf("expected status message %q, got %q", selectedTextCopiedMessage, got)
	}
	if m.viewport.HasActiveSelection() || m.viewport.HasSelection() {
		t.Fatal("expected release to clear viewport selection")
	}
}

func TestModel_RightClickDoesNotStartSelection(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 20, Height: 5})
	m = newModel.(Model)
	m.viewport.Clear()
	m.viewport.AppendOutput([]byte("alpha"))

	newModel, _ = m.Update(tea.MouseClickMsg(tea.Mouse{X: 1, Y: 0, Button: tea.MouseRight}))
	m = newModel.(Model)

	if m.viewport.HasActiveSelection() {
		t.Fatal("expected right click to be ignored")
	}
}

func TestModel_ClearSelectionStatusOnlyClearsOwnedMessage(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.statusBar.SetMessage(selectedTextCopiedMessage)

	newModel, _ := m.Update(clearStatusMsgMsg{})
	m = newModel.(Model)
	if got := m.statusBar.GetMessage(); got != "" {
		t.Fatalf("expected selection status to clear, got %q", got)
	}

	m.statusBar.SetMessage("Other message")
	newModel, _ = m.Update(clearStatusMsgMsg{})
	m = newModel.(Model)
	if got := m.statusBar.GetMessage(); got != "Other message" {
		t.Fatalf("expected unrelated status to remain, got %q", got)
	}
}

func TestMouseEventFilterThrottlesMotionButNotRelease(t *testing.T) {
	lastMouseEvent = time.Now()
	defer func() {
		lastMouseEvent = time.Time{}
	}()

	if got := MouseEventFilter(nil, tea.MouseMotionMsg(tea.Mouse{X: 1, Y: 1, Button: tea.MouseLeft})); got != nil {
		t.Fatalf("expected rapid motion event to be filtered, got %T", got)
	}
	if got := MouseEventFilter(nil, tea.MouseReleaseMsg(tea.Mouse{X: 1, Y: 1, Button: tea.MouseLeft})); got == nil {
		t.Fatal("expected release event to pass through filter")
	}

	lastMouseEvent = time.Now().Add(-mouseEventMinInterval)
	if got := MouseEventFilter(nil, tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown})); got == nil {
		t.Fatal("expected delayed wheel event to pass through filter")
	}
}

func TestModel_Update_PTYOutput(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	testData := []byte("test output")
	newModel, _ := m.Update(ptyOutputMsg{data: testData})
	// Trigger batch flush
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})

	updated := newModel.(Model)

	content := updated.viewport.GetContent()
	if !strings.Contains(content, "test output") {
		t.Errorf("Expected content to contain 'test output', got %q", content)
	}
}

func TestModel_Update_PTYOutput_BufferIsolation(t *testing.T) {
	buf := buffer.New(100)
	m := NewModel(nil, buf, capture.NewSessionContext(), nil)

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(Model)

	newModel, _ = m.Update(ptyOutputMsg{data: []byte("before\n")})
	// Trigger batch flush
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	altScreenData := []byte("\x1b[?1049hFULL\nSCREEN\n\x1b[?1049l")
	newModel, _ = m.Update(ptyOutputMsg{data: altScreenData})
	// Trigger batch flush
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	newModel, _ = m.Update(ptyOutputMsg{data: []byte("after\n")})
	// Trigger batch flush
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	text := buf.ExportAsText()
	if strings.Contains(text, "FULL") || strings.Contains(text, "SCREEN") || strings.Contains(text, "\x1b") {
		t.Errorf("Expected buffer to exclude full-screen output, got %q", text)
	}
	if !strings.Contains(text, "before") || !strings.Contains(text, "after") {
		t.Errorf("Expected buffer to contain normal output, got %q", text)
	}
}

func TestModel_Update_PasteMsg_RoutesToPTY(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	content := "/history\n"

	newModel, cmd := m.Update(tea.PasteMsg{Content: content})
	if cmd != nil {
		t.Fatal("Expected no command from PasteMsg")
	}
	m = newModel.(Model)

	if m.palette.IsVisible() {
		t.Error("Expected palette to remain hidden after paste")
	}
	if m.historyPicker != nil && m.historyPicker.IsVisible() {
		t.Error("Expected history picker to remain hidden after paste")
	}

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY output: %v", err)
	}
	if string(data) != content {
		t.Errorf("Expected PTY output %q, got %q", content, string(data))
	}
}

func TestModel_Update_PTYOutput_ExitSuppressedWithFutureEnter(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(Model)

	newModel, _ = m.Update(ptyOutputMsg{data: []byte("\x1b[?1049h")})
	// Trigger batch flush
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	if !m.fullScreenMode {
		t.Fatal("Expected fullScreenMode to be true after enter")
	}

	newModel, _ = m.Update(ptyOutputMsg{data: []byte("\x1b[?1049l\x1b[?1049h")})
	m = newModel.(Model)

	if !m.fullScreenMode {
		t.Error("Expected fullScreenMode to remain true when exit is followed by enter")
	}
}

func TestModel_View_NotReady(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	view := m.View()
	// View should have content set even when not ready
	if view.Content == "" {
		t.Error("Expected View.Content to be set")
	}
}

func TestModel_View_Ready(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)
	m.viewport.AppendOutput([]byte("hello world"))

	view := m.View()
	// View should have content set when ready
	if view.Content == "" {
		t.Error("Expected View.Content to be set")
	}
}

func TestModel_Update_WindowSize_Debounce(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	// First resize
	newModel, cmd := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(Model)

	if m.resizeDebounceID != 1 {
		t.Errorf("Expected resizeDebounceID 1, got %d", m.resizeDebounceID)
	}
	if cmd == nil {
		t.Error("Expected cmd for debounced resize")
	}

	// Second resize should increment debounce ID
	newModel, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = newModel.(Model)

	if m.resizeDebounceID != 2 {
		t.Errorf("Expected resizeDebounceID 2, got %d", m.resizeDebounceID)
	}

	// Stale resize message should be ignored
	newModel, _ = m.Update(resizeApplyMsg{id: 1, width: 80, height: 24})
	m = newModel.(Model)

	// initialResize should still be false because stale message was ignored
	if m.initialResize {
		t.Error("Expected initialResize to remain false after stale message")
	}
}

func TestResolveGitBranchCmd(t *testing.T) {
	called := false
	cmd := resolveGitBranchCmd("/tmp/repo", func(dir string) string {
		called = true
		if dir != "/tmp/repo" {
			t.Fatalf("resolver received %q, want %q", dir, "/tmp/repo")
		}
		return "main"
	})
	if cmd == nil {
		t.Fatal("expected non-nil command")
	}

	msg := cmd()
	branchMsg, ok := msg.(gitBranchMsg)
	if !ok {
		t.Fatalf("expected gitBranchMsg, got %T", msg)
	}
	if !called {
		t.Fatal("expected resolver to be called")
	}
	if branchMsg.dir != "/tmp/repo" {
		t.Fatalf("expected dir %q, got %q", "/tmp/repo", branchMsg.dir)
	}
	if branchMsg.branch != "main" {
		t.Fatalf("expected branch %q, got %q", "main", branchMsg.branch)
	}
}

func TestResolveGitBranchCmd_EmptyDir(t *testing.T) {
	cmd := resolveGitBranchCmd("  ", func(string) string {
		t.Fatal("resolver should not be called for empty dir")
		return ""
	})
	if cmd != nil {
		t.Fatal("expected nil command for empty dir")
	}
}

func TestModel_Update_DirectoryUpdate_AlwaysResolvesBranch(t *testing.T) {
	resolveCount := 0
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), func() (string, error) {
		return "/tmp/repo", nil
	})
	m.gitBranchResolver = func(dir string) string {
		resolveCount++
		return "main"
	}

	// First tick should trigger a resolve.
	newModel, cmd := m.Update(directoryUpdateMsg{})
	m = newModel.(Model)
	if cmd == nil {
		t.Fatal("expected cmd from directoryUpdateMsg")
	}

	// Second tick (same directory) should still trigger a resolve.
	resolveCount = 0
	newModel, cmd = m.Update(directoryUpdateMsg{})
	m = newModel.(Model)
	if cmd == nil {
		t.Fatal("expected cmd from second directoryUpdateMsg")
	}
}

func TestModel_Update_GitBranchMsgStaleGuard(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.currentDir = "/tmp/current"
	m.gitBranch = "main"

	newModel, _ := m.Update(gitBranchMsg{dir: "/tmp/other", branch: "feature"})
	m = newModel.(Model)
	if m.gitBranch != "main" {
		t.Fatalf("expected stale gitBranchMsg to be ignored, got %q", m.gitBranch)
	}

	newModel, _ = m.Update(gitBranchMsg{dir: "/tmp/current", branch: "feature"})
	m = newModel.(Model)
	if m.gitBranch != "feature" {
		t.Fatalf("expected matching gitBranchMsg to apply, got %q", m.gitBranch)
	}
}

func TestModel_Update_ResizeApply_SetsInitialResize(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.resizeDebounceID = 1

	// First resize apply processes the resize (no longer skipped)
	newModel, _ := m.Update(resizeApplyMsg{id: 1, width: 80, height: 24})
	m = newModel.(Model)

	// Without ptyFile, initialResize stays false (only set in if m.ptyFile != nil block)
	// The resize logic is still reached, just no PTY to resize
	if m.initialResize {
		t.Error("Expected initialResize to be false without ptyFile")
	}

	// resizeTime should be zero since no PTY
	if !m.resizeTime.IsZero() {
		t.Error("Expected resizeTime to be zero without ptyFile")
	}
}

func TestModel_ResizePTYViewport_NilPTY(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	m.resizePTYViewport(80, 24)
	m.resizePTYViewport(0, 24)
	m.resizePTYViewport(80, 0)
	m.resizePTYViewport(-1, 24)
}

func TestModel_ResizePTYViewport_Guard(t *testing.T) {
	ptm, pts := openTestPTY(t)
	defer ptm.Close()
	defer pts.Close()

	if err := terminal.ResizePTY(ptm, 80, 24); err != nil {
		t.Fatalf("ResizePTY baseline failed: %v", err)
	}

	m := NewModel(ptm, buffer.New(100), capture.NewSessionContext(), nil)
	m.resizePTYViewport(0, 24)
	m.resizePTYViewport(80, 0)
	m.resizePTYViewport(-1, 24)
	m.resizePTYViewport(80, -1)

	assertPTYSize(t, ptm, 80, 24)
}

func TestModel_ApplyLayout_ResizesPTYOnSidebarToggle(t *testing.T) {
	ptm, pts := openTestPTY(t)
	defer ptm.Close()
	defer pts.Close()

	m := NewModel(ptm, buffer.New(100), capture.NewSessionContext(), nil)

	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
	m = newModel.(Model)
	newModel, _ = m.Update(resizeApplyMsg{id: m.resizeDebounceID, width: 120, height: 30})
	m = newModel.(Model)
	assertPTYSize(t, ptm, 120, 29)

	newModel, _ = m.Update(input.ToggleChatMsg{})
	m = newModel.(Model)
	left, _ := splitSidebarWidths(120)
	assertPTYSize(t, ptm, left, 29)

	newModel, _ = m.Update(input.ToggleChatMsg{})
	m = newModel.(Model)
	assertPTYSize(t, ptm, 120, 29)
}

func openTestPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()

	ptm, pts, err := cpty.Open()
	if err != nil {
		t.Skipf("pty.Open not available: %v", err)
	}
	return ptm, pts
}

func assertPTYSize(t *testing.T, ptyFile *os.File, wantWidth, wantHeight int) {
	t.Helper()

	width, height, err := terminal.GetPTYSize(ptyFile)
	if err != nil {
		t.Fatalf("GetPTYSize failed: %v", err)
	}
	if width != wantWidth || height != wantHeight {
		t.Fatalf("Expected PTY size %dx%d, got %dx%d", wantWidth, wantHeight, width, height)
	}
}

func TestModel_Update_PTYOutput_SuppressedAfterResize(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	// Simulate recent resize
	m.resizeTime = time.Now()

	// PTY output should be suppressed
	testData := []byte("prompt$ ")
	newModel, cmd := m.Update(ptyOutputMsg{data: testData})
	m = newModel.(Model)

	// Output should NOT appear in viewport
	content := m.viewport.GetContent()
	if strings.Contains(content, "prompt$") {
		t.Error("Expected PTY output to be suppressed after resize")
	}
	// But cmd should schedule next read
	if cmd == nil {
		t.Error("Expected cmd to schedule next PTY read")
	}
}

func TestModel_BuildExplainUserMessage(t *testing.T) {
	buf := buffer.New(100)
	buf.Write([]byte("line one"))
	buf.Write([]byte("line two"))
	buf.Write([]byte("line three"))

	sess := capture.NewSessionContext()
	sess.AddCommand(capture.CommandRecord{Command: "git status"})

	m := NewModel(nil, buf, sess, nil)
	ctx := commands.NewContext(buf, sess, "/tmp")

	got := m.buildExplainUserMessage(ctx)
	expected := "[Asked to explain last 3 lines from terminal. Last command: `git status`]"
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestModel_BuildExplainUserMessage_NoCommand(t *testing.T) {
	buf := buffer.New(100)
	sess := capture.NewSessionContext()
	m := NewModel(nil, buf, sess, nil)
	ctx := commands.NewContext(buf, sess, "/tmp")

	got := m.buildExplainUserMessage(ctx)
	expected := "[Asked to explain last 0 lines from terminal. Last command: `N/A`]"
	if got != expected {
		t.Errorf("Expected %q, got %q", expected, got)
	}
}

func TestModel_ExplainAddsUserPrompt(t *testing.T) {
	buf := buffer.New(100)
	buf.Write([]byte("line one"))
	buf.Write([]byte("line two"))

	sess := capture.NewSessionContext()
	sess.AddCommand(capture.CommandRecord{Command: "ls -la"})

	m := NewModel(nil, buf, sess, nil)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(Model)

	newModel, cmd := m.Update(palette.PaletteSelectMsg{Command: "/explain"})
	if cmd == nil {
		t.Fatal("Expected command to start explain stream")
	}
	m = newModel.(Model)

	if m.sidebar == nil || !m.sidebar.IsVisible() {
		t.Fatal("Expected sidebar to be visible for /explain")
	}

	messages := m.sidebar.GetMessages()
	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(messages))
	}

	expected := "[Asked to explain last 2 lines from terminal. Last command: `ls -la`]"
	if messages[0].Role != "user" {
		t.Fatalf("Expected first message role 'user', got %q", messages[0].Role)
	}
	if messages[0].Content != expected {
		t.Errorf("Expected %q, got %q", expected, messages[0].Content)
	}

	if messages[1].Role != "assistant" {
		t.Fatalf("Expected second message role 'assistant', got %q", messages[1].Role)
	}
	if messages[1].Content != streamThinkingPlaceholder {
		t.Errorf("Expected placeholder %q, got %q", streamThinkingPlaceholder, messages[1].Content)
	}
}

func TestModel_HistoryPickerFlow_FromCommand(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, ".bash_history")
	historyContent := "echo one\necho two\n"
	if err := os.WriteFile(histFile, []byte(historyContent), 0o600); err != nil {
		t.Fatalf("Failed to create history file: %v", err)
	}

	originalHistFile := os.Getenv("HISTFILE")
	if err := os.Setenv("HISTFILE", histFile); err != nil {
		t.Fatalf("Failed to set HISTFILE: %v", err)
	}
	defer os.Setenv("HISTFILE", originalHistFile)

	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	m.inputHandler.SetLineBuffer("git status")

	newModel, cmd := m.Update(palette.PaletteSelectMsg{Command: "/history"})
	if cmd == nil {
		t.Fatal("Expected command to show history picker")
	}
	m = newModel.(Model)

	msg := cmd()
	showMsg, ok := msg.(input.ShowHistoryPickerMsg)
	if !ok {
		t.Fatalf("Expected ShowHistoryPickerMsg, got %T", msg)
	}
	if showMsg.InitialFilter != "" {
		t.Errorf("Expected empty initial filter, got %q", showMsg.InitialFilter)
	}

	newModel, _ = m.Update(showMsg)
	m = newModel.(Model)

	if m.historyPicker == nil || !m.historyPicker.IsVisible() {
		t.Fatal("Expected history picker to be visible")
	}
	if !m.inputHandler.IsHistoryPickerMode() {
		t.Error("Expected history picker mode to be active")
	}

	newModel, selectCmd := m.Update(testutils.TestKeyEnter)
	if selectCmd == nil {
		t.Fatal("Expected selection command after Enter")
	}
	m = newModel.(Model)

	selectMsg := selectCmd()
	selected, ok := selectMsg.(historypicker.HistoryPickerSelectMsg)
	if !ok {
		t.Fatalf("Expected HistoryPickerSelectMsg, got %T", selectMsg)
	}
	if selected.Command != "echo two" {
		t.Errorf("Expected selected command %q, got %q", "echo two", selected.Command)
	}

	newModel, _ = m.Update(selected)
	m = newModel.(Model)

	if m.inputHandler.IsHistoryPickerMode() {
		t.Error("Expected history picker mode to be disabled after selection")
	}

	handled, cmd := m.inputHandler.HandleKey(testutils.NewCtrlKeyPressMsg('r'))
	if !handled || cmd == nil {
		t.Fatal("Expected Ctrl+R to return ShowHistoryPickerMsg")
	}
	msg = cmd()
	showMsg, ok = msg.(input.ShowHistoryPickerMsg)
	if !ok {
		t.Fatalf("Expected ShowHistoryPickerMsg, got %T", msg)
	}
	if showMsg.InitialFilter != "echo two" {
		t.Errorf("Expected initial filter %q, got %q", "echo two", showMsg.InitialFilter)
	}

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY output: %v", err)
	}
	expected := append([]byte{21}, []byte("echo two")...)
	if !bytes.Equal(data, expected) {
		t.Errorf("Expected PTY output %q, got %q", expected, data)
	}
}

func TestModel_CommandExecuteMsg_AppliesCommandToPTY(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	m.setTerminalFocused(false)

	newModel, _ := m.Update(sidebar.CommandExecuteMsg{Command: "  git status  "})
	m = newModel.(Model)

	if !m.terminalFocused {
		t.Fatal("Expected terminal to be focused after command execute")
	}

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY output: %v", err)
	}

	expected := append([]byte{21}, []byte("git status")...)
	if !bytes.Equal(data, expected) {
		t.Fatalf("Expected PTY output %q, got %q", expected, data)
	}
}

func TestModel_CommandExecuteMsg_RejectsMultilineCommand(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)

	newModel, _ := m.Update(sidebar.CommandExecuteMsg{Command: "echo one\necho two"})
	m = newModel.(Model)

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY output: %v", err)
	}
	if len(data) != 0 {
		t.Fatalf("Expected no PTY output for invalid command, got %q", data)
	}
}

func TestModel_CommandSubmitted_ShowsInHistoryPicker(t *testing.T) {
	tmpDir := t.TempDir()
	histFile := filepath.Join(tmpDir, ".bash_history")
	if err := os.WriteFile(histFile, []byte(""), 0o600); err != nil {
		t.Fatalf("Failed to create history file: %v", err)
	}

	originalHistFile := os.Getenv("HISTFILE")
	if err := os.Setenv("HISTFILE", histFile); err != nil {
		t.Fatalf("Failed to set HISTFILE: %v", err)
	}
	defer os.Setenv("HISTFILE", originalHistFile)

	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	newModel, _ := m.Update(input.CommandSubmittedMsg{Command: "echo fresh"})
	m = newModel.(Model)

	newModel, _ = m.Update(input.ShowHistoryPickerMsg{InitialFilter: ""})
	m = newModel.(Model)

	if m.historyPicker == nil || !m.historyPicker.IsVisible() {
		t.Fatal("Expected history picker to be visible")
	}

	view := m.historyPicker.View()
	if !strings.Contains(view, "echo fresh") {
		t.Fatalf("Expected history picker view to include command, got %q", view)
	}
}

func TestModel_PTYOutput_BackspaceNormalization(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	data := []byte("git tashg\x08 \x08\x08 \x08\x08 \x08g\n")
	newModel, _ := m.Update(ptyOutputMsg{data: data})
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	lines := m.buffer.GetLastN(1)
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in buffer, got %d", len(lines))
	}
	if string(lines[0]) != "git tag" {
		t.Fatalf("Expected normalized line %q, got %q", "git tag", string(lines[0]))
	}
}

func TestModel_PTYOutput_StripsOSCAndCapturesCommand(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	data := []byte("\x1b]0;dev@host: ~/project\x07dev@host:~/project$ ifconfig \n")
	newModel, _ := m.Update(ptyOutputMsg{data: data})
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	lines := m.buffer.GetLastN(1)
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line in buffer, got %d", len(lines))
	}
	if strings.Contains(string(lines[0]), "0;") {
		t.Fatalf("Expected OSC sequence to be stripped, got %q", string(lines[0]))
	}

	last := m.session.GetLastN(1)
	if len(last) != 1 || last[0].Command != "ifconfig" {
		t.Fatalf("Expected last command %q, got %+v", "ifconfig", last)
	}
}

func TestModel_LLMContext_StripsOSC(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	data := []byte("\x1b]0;dev@host: ~/project\x07dev@host$ ls\n")
	newModel, _ := m.Update(ptyOutputMsg{data: data})
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
	lines := ctx.GetLastNLines(10)
	termCtx := ai.BuildTerminalContext(lines, ai.TerminalMetadata{})
	if strings.Contains(termCtx.Output, "0;") {
		t.Fatalf("Expected OSC payload to be stripped, got %q", termCtx.Output)
	}
}

func TestModel_LLMContext_BackspaceNormalized(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	data := []byte("git tashg\x08 \x08\x08 \x08\x08 \x08g\n")
	newModel, _ := m.Update(ptyOutputMsg{data: data})
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	ctx := commands.NewContext(m.buffer, m.session, m.currentDir)
	lines := ctx.GetLastNLines(10)
	termCtx := ai.BuildTerminalContext(lines, ai.TerminalMetadata{})
	if strings.Contains(termCtx.Output, "tashg") {
		t.Fatalf("Expected backspace-normalized output, got %q", termCtx.Output)
	}
	if !strings.Contains(termCtx.Output, "git tag") {
		t.Fatalf("Expected output to contain %q, got %q", "git tag", termCtx.Output)
	}
}

func TestModel_Update_PTYOutput_NotSuppressedAfterDelay(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	// Simulate old resize (more than 100ms ago)
	m.resizeTime = time.Now().Add(-200 * time.Millisecond)

	// PTY output should NOT be suppressed
	testData := []byte("normal output")
	newModel, _ := m.Update(ptyOutputMsg{data: testData})
	// Trigger batch flush
	newModel, _ = newModel.Update(ptyBatchFlushMsg{})
	m = newModel.(Model)

	content := m.viewport.GetContent()
	if !strings.Contains(content, "normal output") {
		t.Error("Expected PTY output to appear after resize delay")
	}
}

// TestModel_PasswordProtection_ClearLineBufferPreventsCapture verifies that
// calling ClearLineBuffer before HandleKey prevents password accumulation.
// This tests the mechanism used when echo is disabled (password entry mode).
func TestModel_PasswordProtection_ClearLineBufferPreventsCapture(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)

	// Step 1: Type a command and submit it
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("s"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("u"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("d"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("o"))

	_, submitCmd := m.inputHandler.HandleKey(testutils.TestKeyEnter)
	if submitCmd == nil {
		t.Fatal("Expected command from Enter")
	}
	msg := submitCmd()
	submitted, ok := msg.(input.CommandSubmittedMsg)
	if !ok {
		t.Fatalf("Expected CommandSubmittedMsg, got %T", msg)
	}
	if submitted.Command != "sudo" {
		t.Errorf("Expected 'sudo', got %q", submitted.Command)
	}

	// Process the submission in model
	newModel, _ := m.Update(submitted)
	m = newModel.(Model)

	// Verify command was captured
	last := m.session.GetLastN(1)
	if len(last) != 1 || last[0].Command != "sudo" {
		t.Fatalf("Expected last command 'sudo', got %+v", last)
	}

	// Step 2: Simulate password entry with ClearLineBuffer (echo-off protection)
	// Type password characters
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("s"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("e"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("c"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("r"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("e"))
	m.inputHandler.HandleKey(testutils.NewTextKeyPressMsg("t"))

	// Simulate echo-off detection: clear buffer before Enter
	m.inputHandler.ClearLineBuffer()

	// Submit with Enter
	_, submitCmd = m.inputHandler.HandleKey(testutils.TestKeyEnter)
	if submitCmd == nil {
		t.Fatal("Expected command from Enter")
	}
	msg = submitCmd()
	submitted, ok = msg.(input.CommandSubmittedMsg)
	if !ok {
		t.Fatalf("Expected CommandSubmittedMsg, got %T", msg)
	}

	// Password should NOT be captured (empty command)
	if submitted.Command != "" {
		t.Errorf("Expected empty command (password protected), got %q", submitted.Command)
	}

	// Process the empty submission
	newModel, _ = m.Update(submitted)
	m = newModel.(Model)

	// Last command should STILL be "sudo", NOT the password
	last = m.session.GetLastN(1)
	if len(last) != 1 || last[0].Command != "sudo" {
		t.Fatalf("Expected last command to remain 'sudo', got %+v", last)
	}
}

func TestModel_SecretMode_KeyBypassesPaletteRouting(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	m.secretDetector = func(*os.File) bool { return true }
	m.palette.Show()

	newModel, cmd := m.Update(testutils.NewTextKeyPressMsg("/"))
	m = newModel.(Model)
	if cmd != nil {
		t.Error("Expected no command when key is passed directly to PTY in secret mode")
	}

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY output: %v", err)
	}
	if string(data) != "/" {
		t.Fatalf("Expected PTY output %q, got %q", "/", string(data))
	}
}

func TestModel_SecretMode_PasteBypassesPaletteRouting(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	m.secretDetector = func(*os.File) bool { return true }
	m.palette.Show()

	pasteContent := "secret-paste"
	newModel, cmd := m.Update(tea.PasteMsg{Content: pasteContent})
	m = newModel.(Model)
	if cmd != nil {
		t.Error("Expected no command for paste in secret mode")
	}

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY output: %v", err)
	}
	if string(data) != pasteContent {
		t.Fatalf("Expected PTY output %q, got %q", pasteContent, string(data))
	}
}

func TestModel_SecretModeFalse_CommandCaptureStillWorks(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	m.secretDetector = func(*os.File) bool { return false }

	step := func(msg tea.Msg) {
		newModel, cmd := m.Update(msg)
		m = newModel.(Model)
		if cmd == nil {
			return
		}
		emitted := cmd()
		if emitted == nil {
			return
		}
		newModel, _ = m.Update(emitted)
		m = newModel.(Model)
	}

	step(testutils.NewTextKeyPressMsg("l"))
	step(testutils.NewTextKeyPressMsg("s"))
	step(testutils.TestKeyEnter)

	last := m.session.GetLastN(1)
	if len(last) != 1 {
		t.Fatalf("Expected one command in session, got %d", len(last))
	}
	if last[0].Command != "ls" {
		t.Fatalf("Expected last command %q, got %q", "ls", last[0].Command)
	}
}

func TestModel_Update_StartCopilotAuthMsg(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	// Send StartCopilotAuthMsg
	newModel, cmd := m.Update(settings.StartCopilotAuthMsg{})
	m = newModel.(Model)

	// Should return a command to start the auth flow
	if cmd == nil {
		t.Error("Expected cmd to start Copilot auth flow")
	}
}

func TestModel_Update_CopilotAuthStatusMsg_ShowsPrompt(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	m.settingsPanel.Show(config.Default(), config.GetConfigPath())

	msg := copilotAuthStatusMsg{
		Status: ai.CopilotAuthStatus{
			Authenticated: true,
			Login:         "octo",
			StatusMessage: "Authenticated",
		},
		ShowPrompt: true,
	}
	newModel, cmd := m.Update(msg)
	m = newModel.(Model)

	if cmd != nil {
		t.Error("Expected nil cmd after auth status update")
	}

	panelView := m.settingsPanel.View()
	if !strings.Contains(panelView, "GitHub Copilot Status") {
		t.Errorf("Expected settings panel to show status box, got %q", panelView)
	}
	if !strings.Contains(panelView, "octo") {
		t.Errorf("Expected settings panel to include login, got %q", panelView)
	}
	if m.statusBar.GetMessage() != "" {
		t.Errorf("Expected status bar to be empty, got %q", m.statusBar.GetMessage())
	}
}

func TestModel_Update_CopilotAuthStatusMsg_Error(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	m.settingsPanel.Show(config.Default(), config.GetConfigPath())

	testErr := fmt.Errorf("test auth error")
	msg := copilotAuthStatusMsg{Err: testErr, ShowPrompt: true}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	panelView := m.settingsPanel.View()
	if !strings.Contains(panelView, "test auth error") {
		t.Errorf("Expected settings panel to include error, got %q", panelView)
	}
}

func TestModel_Update_CopilotAuthStatusMsg_PreservesSettingsPanelEdits(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	m.settingsPanel.Show(config.Default(), config.GetConfigPath())
	m.settingsPanel.SetProviderValue("copilot")
	if !m.settingsPanel.HasChanges() {
		t.Fatal("Expected settings panel to have changes")
	}

	msg := copilotAuthStatusMsg{Status: ai.CopilotAuthStatus{Authenticated: true}}
	newModel, _ := m.Update(msg)
	m = newModel.(Model)

	if !m.settingsPanel.IsVisible() {
		t.Error("Expected settings panel to remain visible after auth status update")
	}
	if !m.settingsPanel.HasChanges() {
		t.Error("Expected unsaved changes to be preserved after auth status update")
	}
}

func TestModel_Update_SettingsSaveMsg_UpdatesSidebarLLMLabel(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	cfg := config.Default()
	cfg.LLMProvider = "openai"
	cfg.Providers.OpenAI.Model = "gpt-4.1-mini"
	cfgPath := filepath.Join(t.TempDir(), "config.json")

	newModel, _ := m.Update(settings.SettingsSaveMsg{ConfigPath: cfgPath, Config: cfg})
	m = newModel.(Model)

	if got := m.sidebar.ActiveLLMLabel(); got != "LLM: openai-gpt-4.1-mini" {
		t.Fatalf("Expected updated sidebar LLM label, got %q", got)
	}
}

func TestModel_FocusSwitch_ShiftTab(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(Model)

	if !m.terminalFocused {
		t.Fatal("Expected terminal to be focused by default")
	}

	// Ensure cursor rendering uses the block cursor when visible.
	m.viewport.AppendOutput([]byte("hi\x1b[3C"))
	if !strings.Contains(m.viewport.View(), "█") {
		t.Fatal("Expected terminal cursor block to be visible initially")
	}

	// Shift+Tab should emit FocusSwitchMsg.
	newModel, cmd := m.Update(testutils.TestKeyShiftTab)
	m = newModel.(Model)
	if cmd == nil {
		t.Fatal("Expected Shift+Tab to emit FocusSwitchMsg command")
	}
	msg := cmd()
	switchMsg, ok := msg.(input.FocusSwitchMsg)
	if !ok {
		t.Fatalf("Expected FocusSwitchMsg, got %T", msg)
	}

	// Process focus switch: open sidebar and focus chat input.
	newModel, _ = m.Update(switchMsg)
	m = newModel.(Model)
	if m.sidebar == nil || !m.sidebar.IsVisible() {
		t.Fatal("Expected sidebar to be visible after first Shift+Tab")
	}
	if m.terminalFocused {
		t.Fatal("Expected terminal focus to be false after first Shift+Tab")
	}
	if !m.sidebar.IsFocusedOnInput() {
		t.Fatal("Expected sidebar input to be focused after first Shift+Tab")
	}
	if strings.Contains(m.viewport.View(), "█") {
		t.Fatal("Expected terminal cursor block to be hidden when sidebar is focused")
	}

	// Overlay should block focus switching.
	m.resultPanel.Show("Result", "Content")
	newModel, _ = m.Update(input.FocusSwitchMsg{})
	m = newModel.(Model)
	if m.terminalFocused {
		t.Fatal("Expected focus switch to be blocked while result panel is visible")
	}
	if !m.sidebar.IsFocusedOnInput() {
		t.Fatal("Expected sidebar focus to remain unchanged while overlay is visible")
	}
	m.resultPanel.Hide()

	// Shift+Tab again should switch focus back to terminal.
	newModel, cmd = m.Update(testutils.TestKeyShiftTab)
	m = newModel.(Model)
	if cmd == nil {
		t.Fatal("Expected Shift+Tab to emit FocusSwitchMsg command")
	}
	msg = cmd()
	switchMsg, ok = msg.(input.FocusSwitchMsg)
	if !ok {
		t.Fatalf("Expected FocusSwitchMsg, got %T", msg)
	}
	newModel, _ = m.Update(switchMsg)
	m = newModel.(Model)
	if !m.terminalFocused {
		t.Fatal("Expected terminal focus after second Shift+Tab")
	}
	if m.sidebar.IsFocusedOnInput() {
		t.Fatal("Expected sidebar input to be blurred when terminal is focused")
	}
	if !strings.Contains(m.viewport.View(), "█") {
		t.Fatal("Expected terminal cursor block to be visible when terminal is focused")
	}

	// Ctrl+T close should restore terminal focus.
	newModel, _ = m.Update(input.ToggleChatMsg{})
	m = newModel.(Model)
	if m.sidebar.IsVisible() {
		t.Fatal("Expected sidebar to be hidden after Ctrl+T close")
	}
	if !m.terminalFocused {
		t.Fatal("Expected terminal to remain focused after Ctrl+T close")
	}

	// Ctrl+T open path should sync terminalFocused=false.
	newModel, _ = m.Update(input.ToggleChatMsg{})
	m = newModel.(Model)
	if !m.sidebar.IsVisible() {
		t.Fatal("Expected sidebar to be visible after Ctrl+T open")
	}
	if m.terminalFocused {
		t.Fatal("Expected terminal focus to be false after Ctrl+T open")
	}

	// Esc in chat should close sidebar and restore terminal focus.
	newModel, _ = m.Update(testutils.TestKeyEsc)
	m = newModel.(Model)
	if m.sidebar.IsVisible() {
		t.Fatal("Expected sidebar to be hidden after Esc in chat mode")
	}
	if !m.terminalFocused {
		t.Fatal("Expected terminal focus after Esc closes sidebar")
	}
}

func TestModel_FocusSwitch_TerminalFocusedRoutesKeysToPTY(t *testing.T) {
	tmpDir := t.TempDir()
	ptyFile, err := os.CreateTemp(tmpDir, "pty")
	if err != nil {
		t.Fatalf("Failed to create temp PTY file: %v", err)
	}
	defer ptyFile.Close()

	m := NewModel(ptyFile, buffer.New(100), capture.NewSessionContext(), nil)
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newModel.(Model)

	// Open chat sidebar first (chat focused).
	newModel, _ = m.Update(input.ToggleChatMsg{})
	m = newModel.(Model)
	if m.terminalFocused {
		t.Fatal("Expected chat focus after opening sidebar")
	}

	// Move focus back to terminal.
	m.setTerminalFocused(true)
	if !m.terminalFocused {
		t.Fatal("Expected terminal focus after setTerminalFocused(true)")
	}
	if m.sidebar.IsFocusedOnInput() {
		t.Fatal("Expected sidebar input to be blurred when terminal is focused")
	}

	newModel, cmd := m.Update(testutils.NewTextKeyPressMsg("x"))
	m = newModel.(Model)
	if cmd != nil {
		t.Fatal("Expected no command for regular printable PTY input")
	}
	if m.sidebar.IsFocusedOnInput() {
		t.Fatal("Expected key input to bypass sidebar while terminal is focused")
	}

	if _, err := ptyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("Failed to seek PTY file: %v", err)
	}
	data, err := io.ReadAll(ptyFile)
	if err != nil {
		t.Fatalf("Failed to read PTY file: %v", err)
	}
	if !strings.Contains(string(data), "x") {
		t.Fatalf("Expected PTY to receive typed key, got %q", string(data))
	}
}

func TestModel_Update_OpenModelPickerMsg_GoogleStartsFetchWhenAPIKeyPresent(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	newModel, cmd := m.Update(picker.OpenModelPickerMsg{
		Options:  ai.GetProviderModels("google"),
		Current:  "gemini-2.5-flash",
		FieldKey: "google_model",
		APIKey:   "test-google-key",
	})
	m = newModel.(Model)

	if m.modelPicker == nil || !m.modelPicker.IsVisible() {
		t.Fatal("Expected model picker to be visible")
	}
	if cmd == nil {
		t.Fatal("Expected command to fetch Google models when API key is present")
	}
}

func TestModel_Update_OpenModelPickerMsg_GoogleSkipsFetchWithoutAPIKey(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	newModel, cmd := m.Update(picker.OpenModelPickerMsg{
		Options:  ai.GetProviderModels("google"),
		Current:  "gemini-2.5-flash",
		FieldKey: "google_model",
	})
	m = newModel.(Model)

	if m.modelPicker == nil || !m.modelPicker.IsVisible() {
		t.Fatal("Expected model picker to be visible")
	}
	if cmd != nil {
		t.Fatal("Expected no fetch command when Google API key is missing")
	}
}

func TestModel_Update_ModelPickerSelectMsg_GoogleUpdatesSettings(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)

	cfg := config.Default()
	cfg.LLMProvider = "google"
	cfg.Providers.Google.Model = "gemini-2.5-flash"
	m.settingsPanel.Show(cfg, "/tmp/test_config.json")
	m.modelPicker.Show(ai.GetProviderModels("google"), cfg.Providers.Google.Model, "google_model")

	newModel, cmd := m.Update(picker.ModelPickerSelectMsg{
		ModelID:  "gemini-2.5-pro",
		FieldKey: "google_model",
	})
	m = newModel.(Model)

	if cmd != nil {
		t.Fatal("Expected no command after Gemini model select")
	}
	if got := m.settingsPanel.GetConfig().Providers.Google.Model; got != "gemini-2.5-pro" {
		t.Fatalf("Expected Google model to update, got %q", got)
	}
	if !m.settingsPanel.HasChanges() {
		t.Fatal("Expected settings panel to be marked as changed")
	}
	if m.modelPicker.IsVisible() {
		t.Fatal("Expected model picker to be hidden after selection")
	}
}

func TestGetModelForProvider_Google(t *testing.T) {
	cfg := config.Default()
	cfg.LLMProvider = "google"
	cfg.Providers.Google.Model = "gemini-2.5-pro"

	if got := getModelForProvider(cfg); got != "gemini-2.5-pro" {
		t.Fatalf("Expected configured Google model, got %q", got)
	}

	cfg.Providers.Google.Model = ""
	if got := getModelForProvider(cfg); got != "gemini-3-flash-preview" {
		t.Fatalf("Expected Google fallback model, got %q", got)
	}
}

func TestModel_Update_UpdateCheckMsg_ShowsWelcomeUpdate(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)

	res := updatecheck.Result{
		CurrentVersion:  "v0.1.0",
		LatestVersion:   "v0.2.0",
		ReleaseURL:      "https://github.com/pakru/wtf_cli/releases",
		UpgradeCommand:  "curl -fsSL https://raw.githubusercontent.com/pakru/wtf_cli/main/install.sh | bash",
		UpdateAvailable: true,
	}

	newModel, _ := m.Update(updateCheckMsg{Result: res})
	updated := newModel.(Model)

	if !updated.startupUpdateShown {
		t.Fatal("expected startup update notice to be shown")
	}
	content := updated.viewport.GetContent()
	if !strings.Contains(content, "Update available:") {
		t.Fatalf("expected viewport content to include update section, got %q", content)
	}
}

func TestModel_Update_UpdateCheckMsg_ErrorNoUserNotice(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)
	original := m.viewport.GetContent()

	newModel, _ := m.Update(updateCheckMsg{Err: fmt.Errorf("network down")})
	updated := newModel.(Model)

	if updated.startupUpdateShown {
		t.Fatal("expected no startup update shown on error")
	}
	if updated.viewport.GetContent() != original {
		t.Fatal("expected viewport to remain unchanged on update-check error")
	}
}

// --- Scroll mode tests ---

// fillViewport adds enough lines to create real scrollback.
func fillViewport(m *Model, lines int) {
	for i := 0; i < lines; i++ {
		m.viewport.AppendOutput([]byte("scrollback line\n"))
	}
}

func TestModel_ScrollUp_EntersScrollMode(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	newModel, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	updated := newModel.(Model)

	if !updated.scrollMode {
		t.Error("expected scrollMode=true after shift+up with scrollback")
	}
}

func TestModel_ScrollUp_ShortOutput_NoScrollMode(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 24)
	// Only one line — viewport fits it; scrolling up cannot leave the bottom
	m.viewport.AppendOutput([]byte("short\n"))

	newModel, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	updated := newModel.(Model)

	if updated.scrollMode {
		t.Error("expected scrollMode=false when output fits in viewport")
	}
}

func TestModel_ScrollDown_ExitsScrollModeAtBottom(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	// Enter scroll mode via shift+up
	m2, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	m = m2.(Model)
	if !m.scrollMode {
		t.Fatal("precondition: expected scroll mode after shift+up")
	}

	// Scroll all the way down — use pgdown to reach bottom quickly
	for i := 0; i < 10; i++ {
		m2, _ = m.Update(testutils.TestKeyPgDown)
		m = m2.(Model)
		if !m.scrollMode {
			break
		}
	}

	if m.scrollMode {
		t.Error("expected scrollMode=false after scrolling back to bottom")
	}
}

func TestModel_Esc_ExitsScrollMode(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	m2, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	m = m2.(Model)
	if !m.scrollMode {
		t.Fatal("precondition: expected scroll mode")
	}

	m2, _ = m.Update(testutils.TestKeyEsc)
	m = m2.(Model)

	if m.scrollMode {
		t.Error("expected scrollMode=false after Esc")
	}
	if !m.viewport.IsAtBottom() {
		t.Error("expected viewport snapped to bottom after Esc exits scroll mode")
	}
}

func TestModel_ScrollKeys_NoEffect_WhenSidebarFocused(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	// Give sidebar focus
	m.terminalFocused = false

	m2, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	updated := m2.(Model)

	if updated.scrollMode {
		t.Error("scroll keys should be ignored when sidebar has focus")
	}
}

func TestModel_ScrollKeys_NoEffect_InFullScreenMode(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	m.fullScreenMode = true

	m2, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	updated := m2.(Model)

	if updated.scrollMode {
		t.Error("scroll keys should be ignored in full-screen mode")
	}
}

func TestModel_AutoScroll_PausedDuringScrollMode(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	// Enter scroll mode
	m2, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	m = m2.(Model)
	if !m.scrollMode {
		t.Fatal("precondition: expected scroll mode")
	}

	// Simulate new PTY output arriving while in scroll mode
	m.viewport.AppendOutput([]byte("new output while scrolled\n"))

	// Viewport must NOT snap to bottom — user is still scrolled back
	if m.viewport.IsAtBottom() {
		t.Error("auto-scroll should be paused: viewport must not snap to bottom on new output")
	}
}

func TestModel_Typing_ExitsScrollMode(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), nil)
	m.ready = true
	m.viewport.SetSize(80, 5)
	fillViewport(&m, 30)

	// Enter scroll mode
	m2, _ := m.Update(testutils.NewAltUpKeyPressMsg())
	m = m2.(Model)
	if !m.scrollMode {
		t.Fatal("precondition: expected scroll mode after alt+up")
	}

	// Type a regular character — should exit scroll mode
	m2, _ = m.Update(testutils.NewTextKeyPressMsg("a"))
	m = m2.(Model)

	if m.scrollMode {
		t.Error("expected scrollMode=false after typing a character")
	}
}
