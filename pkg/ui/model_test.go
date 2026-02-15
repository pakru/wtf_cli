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
	"wtf_cli/pkg/ui/components/settings"
	"wtf_cli/pkg/ui/components/testutils"
	"wtf_cli/pkg/ui/input"

	tea "charm.land/bubbletea/v2"
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
	if view.Content == nil {
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
	if view.Content == nil {
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

func TestModel_Update_DirectoryUpdate_ChangeGated(t *testing.T) {
	nextDir := "/tmp/a"
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), func() (string, error) {
		return nextDir, nil
	})

	// Start with initial directory already marked as resolved.
	if m.currentDir != "/tmp/a" || m.lastResolvedDir != "/tmp/a" {
		t.Fatalf("unexpected initial dirs: current=%q resolved=%q", m.currentDir, m.lastResolvedDir)
	}

	// No directory change: gate should keep lastResolvedDir unchanged.
	newModel, _ := m.Update(directoryUpdateMsg{})
	m = newModel.(Model)
	if m.lastResolvedDir != "/tmp/a" {
		t.Fatalf("expected lastResolvedDir to remain %q, got %q", "/tmp/a", m.lastResolvedDir)
	}

	// Directory change: gate should advance to new directory.
	nextDir = "/tmp/b"
	newModel, _ = m.Update(directoryUpdateMsg{})
	m = newModel.(Model)
	if m.currentDir != "/tmp/b" {
		t.Fatalf("expected currentDir %q, got %q", "/tmp/b", m.currentDir)
	}
	if m.lastResolvedDir != "/tmp/b" {
		t.Fatalf("expected lastResolvedDir %q, got %q", "/tmp/b", m.lastResolvedDir)
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

func TestModel_Update_CommandSubmitted_ResetsLastResolvedDir(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext(), func() (string, error) {
		return "/tmp/repo", nil
	})
	m.lastResolvedDir = "/tmp/repo"

	newModel, _ := m.Update(input.CommandSubmittedMsg{Command: "git checkout -b feature/test"})
	m = newModel.(Model)
	if m.lastResolvedDir != "" {
		t.Fatalf("expected lastResolvedDir to be reset after submitted command, got %q", m.lastResolvedDir)
	}

	m.lastResolvedDir = "/tmp/repo"
	newModel, _ = m.Update(input.CommandSubmittedMsg{Command: "   "})
	m = newModel.(Model)
	if m.lastResolvedDir != "/tmp/repo" {
		t.Fatalf("expected empty command not to reset lastResolvedDir, got %q", m.lastResolvedDir)
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
