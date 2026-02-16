package sidebar

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ui/components/testutils"
)

const (
	applyFooterHint = "Enter Apply | Up/Down Navigate | Shift+Tab TTY | Ctrl+T Hide"
	sendFooterHint  = "Enter Send | Up/Down Scroll | Shift+Tab TTY | Ctrl+T Hide"
)

func TestSidebar_AppendUserMessage(t *testing.T) {
	s := NewSidebar()

	s.AppendUserMessage("Hello AI")

	messages := s.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Errorf("Expected role 'user', got %q", messages[0].Role)
	}
	if messages[0].Content != "Hello AI" {
		t.Errorf("Expected content 'Hello AI', got %q", messages[0].Content)
	}
}

func TestSidebar_StartAssistantMessage(t *testing.T) {
	s := NewSidebar()

	s.StartAssistantMessage()

	messages := s.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %q", messages[0].Role)
	}
	if messages[0].Content != "" {
		t.Errorf("Expected empty content initially, got %q", messages[0].Content)
	}
}

func TestSidebar_UpdateLastMessage(t *testing.T) {
	s := NewSidebar()

	s.StartAssistantMessage()
	s.UpdateLastMessage("Hello")
	s.UpdateLastMessage(" world")

	messages := s.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Hello world" {
		t.Errorf("Expected 'Hello world', got %q", messages[0].Content)
	}
}

func TestSidebar_SetLastMessageContent(t *testing.T) {
	s := NewSidebar()

	s.StartAssistantMessage()
	s.UpdateLastMessage("Hello")
	s.SetLastMessageContent("Replaced")

	messages := s.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Content != "Replaced" {
		t.Errorf("Expected 'Replaced', got %q", messages[0].Content)
	}
}

func TestSidebar_RemoveLastMessage(t *testing.T) {
	s := NewSidebar()

	s.AppendUserMessage("First")
	s.StartAssistantMessage()
	s.RemoveLastMessage()

	messages := s.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	if messages[0].Role != "user" {
		t.Errorf("Expected role 'user', got %q", messages[0].Role)
	}
}

func TestSidebar_AppendErrorMessage(t *testing.T) {
	s := NewSidebar()

	s.AppendErrorMessage("Connection failed")

	messages := s.GetMessages()
	if len(messages) != 1 {
		t.Fatalf("Expected 1 message, got %d", len(messages))
	}
	// Error messages are added as assistant messages with emoji prefix
	if messages[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %q", messages[0].Role)
	}
	expected := "❌ Error: Connection failed"
	if runtime.GOOS == "darwin" {
		expected = "Error: Connection failed"
	}
	if messages[0].Content != expected {
		t.Errorf("Expected %q, got %q", expected, messages[0].Content)
	}
}

func TestSidebar_StreamingState(t *testing.T) {
	s := NewSidebar()

	// Initially not streaming
	if s.IsStreaming() {
		t.Error("Expected streaming to be false initially")
	}

	// Set streaming true
	s.SetStreaming(true)
	if !s.IsStreaming() {
		t.Error("Expected streaming to be true after SetStreaming(true)")
	}

	// Set streaming false
	s.SetStreaming(false)
	if s.IsStreaming() {
		t.Error("Expected streaming to be false after SetStreaming(false)")
	}
}

func TestSidebar_FocusToggle(t *testing.T) {
	s := NewSidebar()

	// NewSidebar sets initial focus to input.
	if !s.IsFocusedOnInput() {
		t.Error("Expected focus on input initially")
	}

	// Toggle to viewport
	s.ToggleFocus()
	if s.IsFocusedOnInput() {
		t.Error("Expected focus on viewport after ToggleFocus()")
	}

	// Toggle back to input
	s.ToggleFocus()
	if !s.IsFocusedOnInput() {
		t.Error("Expected focus on input after second ToggleFocus()")
	}
}

func TestSidebar_FocusInput(t *testing.T) {
	s := NewSidebar()

	s.FocusInput()
	if !s.IsFocusedOnInput() {
		t.Error("Expected focus on input after FocusInput()")
	}
}

func TestSidebar_BlurInput(t *testing.T) {
	s := NewSidebar()

	s.BlurInput()
	if s.IsFocusedOnInput() {
		t.Error("Expected focus on viewport after BlurInput()")
	}
}

func TestSidebar_SubmitMessage(t *testing.T) {
	s := NewSidebar()
	s.FocusInput()

	// Simulate textarea having content
	s.textarea.SetValue("Test message")

	content, ok := s.SubmitMessage()
	if !ok {
		t.Error("Expected SubmitMessage to return ok=true")
	}
	if content != "Test message" {
		t.Errorf("Expected 'Test message', got %q", content)
	}

	// Textarea should be cleared
	if s.textarea.Value() != "" {
		t.Errorf("Expected textarea to be cleared, got %q", s.textarea.Value())
	}
}

func TestSidebar_SubmitMessage_Empty(t *testing.T) {
	s := NewSidebar()
	s.FocusInput()

	// Empty textarea
	s.textarea.SetValue("")

	content, ok := s.SubmitMessage()
	if ok {
		t.Error("Expected SubmitMessage to return ok=false for empty content")
	}
	if content != "" {
		t.Errorf("Expected empty content, got %q", content)
	}
}

func TestSidebar_MessageHistory(t *testing.T) {
	s := NewSidebar()

	// Add multiple messages
	s.AppendUserMessage("First question")
	s.StartAssistantMessage()
	s.UpdateLastMessage("First answer")
	s.AppendUserMessage("Second question")

	messages := s.GetMessages()
	if len(messages) != 3 {
		t.Fatalf("Expected 3 messages, got %d", len(messages))
	}

	// Verify order
	if messages[0].Role != "user" || messages[0].Content != "First question" {
		t.Error("First message incorrect")
	}
	if messages[1].Role != "assistant" || messages[1].Content != "First answer" {
		t.Error("Second message incorrect")
	}
	if messages[2].Role != "user" || messages[2].Content != "Second question" {
		t.Error("Third message incorrect")
	}
}

func TestSidebar_GetMessages_ReturnsSlice(t *testing.T) {
	s := NewSidebar()

	s.AppendUserMessage("Test")

	// Get messages twice to ensure it returns the underlying slice
	messages1 := s.GetMessages()
	messages2 := s.GetMessages()

	if len(messages1) != len(messages2) {
		t.Error("GetMessages should return consistent results")
	}
}

func TestSidebar_ConversionToChatMessage(t *testing.T) {
	s := NewSidebar()

	s.AppendUserMessage("Hello")
	messages := s.GetMessages()

	// Ensure it's a valid ai.ChatMessage
	var _ []ai.ChatMessage = messages
}

func TestSidebar_GetTitle(t *testing.T) {
	s := NewSidebar()

	// Initially empty title
	if s.GetTitle() != "" {
		t.Errorf("Expected empty title initially, got %q", s.GetTitle())
	}

	// After Show, title should be set
	s.Show("WTF Analysis", "Some content")
	if s.GetTitle() != "WTF Analysis" {
		t.Errorf("Expected 'WTF Analysis', got %q", s.GetTitle())
	}

	// Hide and re-show should preserve title
	s.Hide()
	if s.GetTitle() != "WTF Analysis" {
		t.Errorf("Expected title to persist after Hide(), got %q", s.GetTitle())
	}
}

func TestSidebar_ScrollKeysScrollViewport(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 15)

	lines := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("Line %d", i))
	}
	s.Show("WTF Analysis", strings.Join(lines, "\n"))
	s.FocusInput()

	s.scrollY = 5
	s.follow = true

	s.Update(testutils.TestKeyUp)
	if s.scrollY != 4 {
		t.Errorf("Expected scrollY to decrement to 4, got %d", s.scrollY)
	}
	if s.follow {
		t.Error("Expected follow to be false after scrolling up")
	}

	s.Update(testutils.TestKeyDown)
	if s.scrollY != 5 {
		t.Errorf("Expected scrollY to increment to 5, got %d", s.scrollY)
	}
	if s.follow {
		t.Error("Expected follow to remain false when not at bottom")
	}

	s.scrollY = s.maxScroll() - 1
	s.follow = false
	s.Update(testutils.TestKeyDown)
	if s.scrollY != s.maxScroll() {
		t.Errorf("Expected scrollY to reach maxScroll, got %d", s.scrollY)
	}
	if !s.follow {
		t.Error("Expected follow to be true at bottom after scrolling down")
	}
}

func TestSidebar_CommandMarkersAreStrippedAndFooterShown(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Run <cmd>ls -la</cmd> to inspect files.")
	s.Show("WTF Analysis", "")

	view := s.View()
	if strings.Contains(view, "<cmd>") || strings.Contains(view, "</cmd>") {
		t.Fatalf("Expected command markers to be stripped in view, got:\n%s", view)
	}
	if !strings.Contains(view, applyFooterHint) {
		t.Fatalf("Expected command footer hint in view, got:\n%s", view)
	}
	if s.cmdSelectedIdx < 0 {
		t.Fatalf("Expected an active command selection")
	}
}

func TestSidebar_CtrlEnterDoesNotExecuteCommand(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show("WTF Analysis", "")
	s.FocusInput()

	cmd := s.Update(testutils.TestKeyCtrlEnter)
	if cmd != nil {
		t.Fatal("Expected ctrl+enter to be ignored")
	}
}

func TestSidebar_CtrlJDoesNotExecuteCommand(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show("WTF Analysis", "")
	s.FocusInput()

	cmd := s.Update(testutils.NewCtrlKeyPressMsg('j'))
	if cmd != nil {
		t.Fatal("Expected ctrl+j to be ignored")
	}
}

func TestSidebar_EnterOnEmptyInputEmitsCommandExecuteMsg(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show("WTF Analysis", "")
	s.FocusInput()

	cmd := s.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected enter on empty input to emit command execute message")
	}

	msg := cmd()
	execMsg, ok := msg.(CommandExecuteMsg)
	if !ok {
		t.Fatalf("Expected CommandExecuteMsg, got %T", msg)
	}
	if execMsg.Command != "git status" {
		t.Fatalf("Expected command %q, got %q", "git status", execMsg.Command)
	}
}

func TestSidebar_EnterWithTextSubmitsChatMessage(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show("WTF Analysis", "")
	s.FocusInput()
	s.textarea.SetValue("why this command?")

	cmd := s.Update(testutils.TestKeyEnter)
	if cmd == nil {
		t.Fatal("Expected enter with non-empty input to submit chat message")
	}

	msg := cmd()
	chatMsg, ok := msg.(ChatSubmitMsg)
	if !ok {
		t.Fatalf("Expected ChatSubmitMsg, got %T", msg)
	}
	if chatMsg.Content != "why this command?" {
		t.Fatalf("Expected chat content %q, got %q", "why this command?", chatMsg.Content)
	}
}

func TestSidebar_FooterHintChangesWithInputState(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show("WTF Analysis", "")
	s.FocusInput()

	view := s.View()
	if !strings.Contains(view, applyFooterHint) {
		t.Fatalf("Expected apply footer hint when input is empty, got:\n%s", view)
	}
	if strings.Contains(view, sendFooterHint) {
		t.Fatalf("Did not expect send hint when input is empty, got:\n%s", view)
	}

	s.textarea.SetValue("hello")
	view = s.View()
	if !strings.Contains(view, sendFooterHint) {
		t.Fatalf("Expected send footer hint when input has text, got:\n%s", view)
	}
	if strings.Contains(view, applyFooterHint) {
		t.Fatalf("Did not expect apply hint when input has text, got:\n%s", view)
	}
}

func TestSidebar_FooterHintRendersBelowInput(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show("WTF Analysis", "")
	s.FocusInput()

	view := stripANSICodes(s.View())

	inputIdx := strings.Index(view, "Type your message...")
	if inputIdx < 0 {
		t.Fatalf("Expected input placeholder in view, got:\n%s", view)
	}
	if strings.Contains(view, "1 Type your message...") {
		t.Fatalf("Did not expect textarea line-number gutter in view, got:\n%s", view)
	}
	hintIdx := strings.Index(view, applyFooterHint)
	if hintIdx < 0 {
		t.Fatalf("Expected footer hint in view, got:\n%s", view)
	}
	if hintIdx < inputIdx {
		t.Fatalf("Expected footer hint below input (hintIdx=%d inputIdx=%d), got:\n%s", hintIdx, inputIdx, view)
	}

	var hintLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, applyFooterHint) {
			hintLine = line
			break
		}
	}
	if hintLine == "" {
		t.Fatalf("Expected footer hint line in view, got:\n%s", view)
	}
	if idx := strings.Index(hintLine, applyFooterHint); idx <= 3 {
		t.Fatalf("Expected centered footer hint with left padding, got line: %q", hintLine)
	}
}

func TestSidebar_CommandStyleDoesNotCorruptANSI(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>ls -la</cmd> now")
	s.Show("WTF Analysis", "")

	view := s.View()
	if strings.Contains(view, "[4;97;4m[") {
		t.Fatalf("Detected corrupted command ANSI rendering in view: %q", view)
	}
}

func TestSidebar_ArrowKeysStepCommandsWhenScrollSelectionStaysStable(t *testing.T) {
	s := NewSidebar()
	s.visible = true
	s.width = 80
	s.height = 20
	s.focused = FocusInput
	s.cmdList = []CommandEntry{
		{Command: "docker network prune"},
		{Command: "nmcli dev wifi"},
		{Command: "ip -c a"},
	}
	// Simulate a layout where viewport-center tracking keeps command 0 selected.
	s.cmdRenderedLines = []int{0, 50, 100}
	s.cmdSelectedIdx = 0
	s.lines = make([]string, s.viewportHeight()) // maxScroll == 0
	s.scrollY = s.maxScroll()
	s.updateActiveCommand()

	if s.cmdSelectedIdx != 0 {
		t.Fatalf("Expected initial selected command index 0, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyDown)
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("Expected down key to move selection to index 1, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyDown)
	if s.cmdSelectedIdx != 2 {
		t.Fatalf("Expected down key to move selection to index 2, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("Expected up key to move selection back to index 1, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("Expected up key to move selection back to index 0, got %d", s.cmdSelectedIdx)
	}
}

func TestSidebar_ArrowKeysScrollWhenInputHasText(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"line 9",
		"line 10",
		"<cmd>docker network prune</cmd>",
		"<cmd>docker network ls</cmd>",
	}, "\n"))
	s.Show("WTF Analysis", "")
	s.FocusInput()

	if s.maxScroll() < 1 {
		t.Fatalf("Expected scrollable content, maxScroll=%d", s.maxScroll())
	}

	s.textarea.SetValue("typed")
	if s.commandSelectionEnabled() {
		t.Fatal("Expected command selection to be disabled when input has text")
	}

	s.scrollY = 0
	s.Update(testutils.TestKeyDown)

	if s.scrollY <= 0 {
		t.Fatalf("Expected down key to scroll content when input has text, got scrollY=%d", s.scrollY)
	}
}

func TestSidebar_RefreshView_SelectsLastCommandWhenFollowing(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.AppendUserMessage("Explain terminal output")
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Try:",
		"<cmd>docker network prune</cmd>",
		"<cmd>docker network ls</cmd>",
		"<cmd>ip -brief addr</cmd>",
		"<cmd>nmcli dev wifi</cmd>",
	}, "\n"))
	s.Show("WTF Analysis", "")
	s.FocusInput()

	if len(s.cmdList) != 4 {
		t.Fatalf("Expected 4 commands, got %d", len(s.cmdList))
	}
	if s.cmdSelectedIdx != 3 {
		t.Fatalf("Expected last command to be selected initially, got %d", s.cmdSelectedIdx)
	}
}

func TestSidebar_NavigatesAcrossAllRenderedCommands(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"### Suggested Actions:",
		"<cmd>docker network prune</cmd>",
		"<cmd>docker network ls</cmd>",
		"<cmd>ip -brief addr</cmd>",
		"<cmd>nmcli dev wifi</cmd>",
	}, "\n"))
	s.Show("WTF Analysis", "")
	s.FocusInput()

	if len(s.cmdList) != 4 {
		t.Fatalf("Expected 4 commands, got %d", len(s.cmdList))
	}
	if s.cmdSelectedIdx != 3 {
		t.Fatalf("Expected initial selected command to be last index 3, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 2 {
		t.Fatalf("Expected selection index 2 after first up, got %d", s.cmdSelectedIdx)
	}
	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("Expected selection index 1 after second up, got %d", s.cmdSelectedIdx)
	}
	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("Expected selection index 0 after third up, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyDown)
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("Expected selection index 1 after down, got %d", s.cmdSelectedIdx)
	}
}
