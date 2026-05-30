package sidebar

import (
	"fmt"
	"strings"
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ui/components/testutils"

	tea "charm.land/bubbletea/v2"
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
	// Error messages are added as assistant messages with an "Error: " prefix.
	if messages[0].Role != "assistant" {
		t.Errorf("Expected role 'assistant', got %q", messages[0].Role)
	}
	expected := "Error: Connection failed"
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

func TestSidebar_ScrollKeysScrollViewport(t *testing.T) {
	s := NewSidebar()
	s.SetSize(40, 15)

	lines := make([]string, 0, 20)
	for i := 0; i < 20; i++ {
		lines = append(lines, fmt.Sprintf("Line %d", i))
	}
	s.SetContent(strings.Join(lines, "\n"))
	s.Show()
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
	s.Show()

	view := stripANSICodes(s.View())
	if strings.Contains(view, "<cmd>") || strings.Contains(view, "</cmd>") {
		t.Fatalf("Expected command markers to be stripped in view, got:\n%s", view)
	}
	if !strings.Contains(view, "LLM: unknown-unknown | Apply") {
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
	s.Show()
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
	s.Show()
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
	s.Show()
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
	s.Show()
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
	s.Show()
	s.FocusInput()

	view := stripANSICodes(s.View())
	if !strings.Contains(view, "LLM: unknown-unknown | Apply") {
		t.Fatalf("Expected apply footer hint when input is empty, got:\n%s", view)
	}

	s.textarea.SetValue("hello")
	view = stripANSICodes(s.View())
	// With text typed the command is no longer selectable, so footer shows only the LLM label.
	if !strings.Contains(view, "LLM: unknown-unknown") {
		t.Fatalf("Expected LLM label in footer when input has text, got:\n%s", view)
	}
	if strings.Contains(view, "| Apply") {
		t.Fatalf("Did not expect apply hint when input has text, got:\n%s", view)
	}
	if strings.Contains(view, "| Send") {
		t.Fatalf("Did not expect send hint in footer, got:\n%s", view)
	}
}

func TestSidebar_FooterHintRendersBelowInput(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>git status</cmd>.")
	s.Show()
	s.FocusInput()

	view := stripANSICodes(s.View())

	inputIdx := strings.Index(view, "Type your message...")
	if inputIdx < 0 {
		t.Fatalf("Expected input placeholder in view, got:\n%s", view)
	}
	if strings.Contains(view, "1 Type your message...") {
		t.Fatalf("Did not expect textarea line-number gutter in view, got:\n%s", view)
	}
	hintIdx := strings.Index(view, "LLM: unknown-unknown")
	if hintIdx < 0 {
		t.Fatalf("Expected footer hint in view, got:\n%s", view)
	}
	if hintIdx < inputIdx {
		t.Fatalf("Expected footer hint below input (hintIdx=%d inputIdx=%d), got:\n%s", hintIdx, inputIdx, view)
	}

	var hintLine string
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "LLM: unknown-unknown") {
			hintLine = line
			break
		}
	}
	if hintLine == "" {
		t.Fatalf("Expected footer hint line in view, got:\n%s", view)
	}
	// Footer is left-aligned; the LLM label should appear near the start.
	if !strings.Contains(hintLine, "LLM: unknown-unknown") {
		t.Fatalf("Expected left-aligned footer hint with LLM label, got line: %q", hintLine)
	}
}

func TestSidebar_FooterAlwaysShownWithoutCommands(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.Show()

	view := stripANSICodes(s.View())
	if !strings.Contains(view, "LLM: unknown-unknown") {
		t.Fatalf("Expected footer with LLM label even without command entries, got:\n%s", view)
	}
}

func TestSidebar_SetActiveLLM_UpdatesFooterLabel(t *testing.T) {
	s := NewSidebar()
	s.SetSize(90, 14)
	s.SetActiveLLM("openai", "gpt-4o")
	s.Show()

	view := stripANSICodes(s.View())
	if !strings.Contains(view, "LLM: openai-gpt-4o") {
		t.Fatalf("Expected provider-model label in footer, got:\n%s", view)
	}
}

func TestSidebar_CommandStyleDoesNotCorruptANSI(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 20)
	s.StartAssistantMessageWithContent("Use <cmd>ls -la</cmd> now")
	s.Show()

	view := s.View()
	if strings.Contains(view, "[4;97;4m[") {
		t.Fatalf("Detected corrupted command ANSI rendering in view: %q", view)
	}
}

func TestSidebar_ArrowKeysScrollWhenCommandSelectable(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Use <cmd>git status</cmd>",
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
	}, "\n"))
	s.Show()
	s.FocusInput()

	if !s.commandSelectionEnabled() {
		t.Fatal("Expected command selection to be enabled")
	}

	s.scrollY = 0
	s.follow = false
	s.updateActiveCommand() // sync selection to the visible top command, as a real scroll would
	s.Update(testutils.TestKeyDown)
	if s.scrollY != 1 {
		t.Fatalf("Expected down key to scroll one line, got scrollY=%d", s.scrollY)
	}

	s.Update(testutils.TestKeyUp)
	if s.scrollY != 0 {
		t.Fatalf("Expected up key to scroll back to top, got scrollY=%d", s.scrollY)
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
		"line 11",
		"line 12",
		"<cmd>docker network prune</cmd>",
		"<cmd>docker network ls</cmd>",
	}, "\n"))
	s.Show()
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

func TestSidebar_MouseWheelScrollsWhenCommandSelectableAfterStreaming(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Use <cmd>git status</cmd>",
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
	}, "\n"))
	s.Show()
	s.FocusInput()
	s.SetStreaming(false)

	if s.maxScroll() < 1 {
		t.Fatalf("Expected scrollable content, maxScroll=%d", s.maxScroll())
	}
	if !s.commandSelectionEnabled() {
		t.Fatal("Expected command selection to be enabled after streaming")
	}

	s.scrollY = 0
	s.follow = false
	s.updateActiveCommand()
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("Expected visible command to be selected, got %d", s.cmdSelectedIdx)
	}

	s.HandleWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))

	if s.scrollY != 1 {
		t.Fatalf("Expected wheel down to scroll exactly one line, got scrollY=%d", s.scrollY)
	}
}

func TestSidebar_MouseWheelScrollsWhileStreamingWithCommands(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Use <cmd>git status</cmd>",
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
	}, "\n"))
	s.Show()
	s.FocusInput()
	s.SetStreaming(true)

	if s.maxScroll() < 1 {
		t.Fatalf("Expected scrollable content, maxScroll=%d", s.maxScroll())
	}
	if s.commandSelectionEnabled() {
		t.Fatal("Expected command selection to be disabled while streaming")
	}

	s.scrollY = 0
	s.follow = false
	s.HandleWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelDown}))

	if s.scrollY != 1 {
		t.Fatalf("Expected wheel down to scroll while streaming, got scrollY=%d", s.scrollY)
	}
}

func TestSidebar_WheelUpBreaksFollowMode(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Use <cmd>git status</cmd>",
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
	}, "\n"))
	s.Show()
	s.FocusInput()

	if s.maxScroll() < 1 {
		t.Fatalf("Expected scrollable content, maxScroll=%d", s.maxScroll())
	}
	if !s.follow {
		t.Fatal("Expected sidebar to follow after Show")
	}

	before := s.scrollY
	s.HandleWheel(tea.MouseWheelMsg(tea.Mouse{Button: tea.MouseWheelUp}))

	if s.scrollY != before-1 {
		t.Fatalf("Expected wheel up to move from %d to %d, got %d", before, before-1, s.scrollY)
	}
	if s.follow {
		t.Fatal("Expected wheel up to disable follow mode")
	}
}

func TestSidebar_PageKeysStillScrollWhenCommandSelectable(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Use <cmd>git status</cmd>",
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
		"line 11",
		"line 12",
	}, "\n"))
	s.Show()
	s.FocusInput()

	if !s.commandSelectionEnabled() {
		t.Fatal("Expected command selection to be enabled")
	}

	s.scrollY = 0
	s.follow = false
	s.updateActiveCommand()
	s.Update(testutils.TestKeyPgDown)

	if s.scrollY <= 0 {
		t.Fatalf("Expected PgDown to scroll content, got scrollY=%d", s.scrollY)
	}

	afterPgDown := s.scrollY
	s.Update(testutils.TestKeyPgUp)

	if s.scrollY >= afterPgDown {
		t.Fatalf("Expected PgUp to scroll upward from %d, got %d", afterPgDown, s.scrollY)
	}
}

func TestSidebar_RefreshViewFollowStaysAtBottomWithCommandAboveBottom(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"intro",
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"Use <cmd>git status</cmd>",
		"tail 1",
		"tail 2",
		"tail 3",
	}, "\n"))
	s.Show()
	s.FocusInput()

	if s.maxScroll() < 1 {
		t.Fatalf("Expected scrollable content, maxScroll=%d", s.maxScroll())
	}
	s.follow = true
	s.RefreshView()

	if s.scrollY != s.maxScroll() {
		t.Fatalf("Expected follow refresh to stay at bottom %d, got %d", s.maxScroll(), s.scrollY)
	}
	expected := lastVisibleCommandIndex(s)
	if expected < 0 {
		t.Fatal("Expected fixture to leave a command visible in the bottom viewport")
	}
	if s.cmdSelectedIdx != expected {
		t.Fatalf("Expected selected command index %d, got %d", expected, s.cmdSelectedIdx)
	}
}

func TestSidebar_UpdateActiveCommandSelectsLastVisibleCommand(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.cmdList = []CommandEntry{
		{Command: "first"},
		{Command: "visible first"},
		{Command: "visible last"},
		{Command: "offscreen last"},
	}
	s.cmdRenderedLines = []int{0, 5, 7, 20}
	s.lines = make([]string, 30)
	s.scrollY = 4

	s.updateActiveCommand()

	if s.cmdSelectedIdx != 2 {
		t.Fatalf("Expected last visible command index 2, got %d", s.cmdSelectedIdx)
	}
}

func TestSidebar_UpdateActiveCommandIgnoresOffscreenCommands(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.cmdList = []CommandEntry{
		{Command: "above"},
		{Command: "below"},
	}
	s.cmdRenderedLines = []int{0, 20}
	s.lines = make([]string, 30)
	s.scrollY = 8

	s.updateActiveCommand()

	if s.cmdSelectedIdx != -1 {
		t.Fatalf("Expected no selected command when all commands are offscreen, got %d", s.cmdSelectedIdx)
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
	s.Show()
	s.FocusInput()

	if len(s.cmdList) != 4 {
		t.Fatalf("Expected 4 commands, got %d", len(s.cmdList))
	}
	if s.cmdSelectedIdx != 3 {
		t.Fatalf("Expected last command to be selected initially, got %d", s.cmdSelectedIdx)
	}
}

func TestSidebar_ArrowUpSelectsUpperCommandWhenNoScroll(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 30)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Try one of these:",
		"<cmd>docker network prune</cmd>",
		"<cmd>docker network ls</cmd>",
		"<cmd>ip -brief addr</cmd>",
	}, "\n"))
	s.Show()
	s.FocusInput()

	if len(s.cmdList) != 3 {
		t.Fatalf("expected 3 commands, got %d", len(s.cmdList))
	}
	if s.maxScroll() != 0 {
		t.Fatalf("expected content to fit without scrolling, maxScroll=%d", s.maxScroll())
	}
	if !s.commandSelectionEnabled() {
		t.Fatal("expected command selection to be enabled")
	}
	if s.cmdSelectedIdx != 2 {
		t.Fatalf("expected last command selected initially, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("expected up to select middle command (1), got %d", s.cmdSelectedIdx)
	}
	if s.scrollY != 0 {
		t.Fatalf("expected no scroll while stepping commands, scrollY=%d", s.scrollY)
	}

	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("expected up to select first command (0), got %d", s.cmdSelectedIdx)
	}

	// Regression guard: Up at the top command must not jump back to the last.
	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("expected selection to stay on first command, got %d", s.cmdSelectedIdx)
	}
	if s.scrollY != 0 {
		t.Fatalf("expected scrollY to stay 0, got %d", s.scrollY)
	}
}

func TestSidebar_ArrowDownStopsAtLastCommandWhenNoScroll(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 30)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Try one of these:",
		"<cmd>docker network prune</cmd>",
		"<cmd>docker network ls</cmd>",
		"<cmd>ip -brief addr</cmd>",
	}, "\n"))
	s.Show()
	s.FocusInput()

	if s.maxScroll() != 0 {
		t.Fatalf("expected content to fit without scrolling, maxScroll=%d", s.maxScroll())
	}

	// Move selection to the first command.
	s.Update(testutils.TestKeyUp)
	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("expected first command selected, got %d", s.cmdSelectedIdx)
	}

	s.Update(testutils.TestKeyDown)
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("expected down to select command 1, got %d", s.cmdSelectedIdx)
	}
	s.Update(testutils.TestKeyDown)
	if s.cmdSelectedIdx != 2 {
		t.Fatalf("expected down to select last command 2, got %d", s.cmdSelectedIdx)
	}

	// Extra down at the last command stays put, no scroll.
	s.Update(testutils.TestKeyDown)
	if s.cmdSelectedIdx != 2 {
		t.Fatalf("expected selection to stay on last command, got %d", s.cmdSelectedIdx)
	}
	if s.scrollY != 0 {
		t.Fatalf("expected scrollY to stay 0, got %d", s.scrollY)
	}
}

func TestSidebar_ArrowNavigationStepsThenScrollsPreservingSelection(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"line 1",
		"line 2",
		"line 3",
		"line 4",
		"line 5",
		"line 6",
		"line 7",
		"line 8",
		"Use <cmd>git status</cmd>",
		"and <cmd>git log</cmd>",
	}, "\n"))
	s.Show()
	s.FocusInput()

	if s.maxScroll() < 1 {
		t.Fatalf("expected scrollable content, maxScroll=%d", s.maxScroll())
	}
	if len(s.cmdList) != 2 {
		t.Fatalf("expected 2 commands, got %d", len(s.cmdList))
	}
	if !s.commandSelectionEnabled() {
		t.Fatal("expected command selection to be enabled")
	}
	// Follow mode keeps both commands at the bottom; the last is selected.
	if s.cmdSelectedIdx != 1 {
		t.Fatalf("expected last command selected, got %d", s.cmdSelectedIdx)
	}

	// Step up to the upper visible command without scrolling.
	scrollBefore := s.scrollY
	s.Update(testutils.TestKeyUp)
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("expected up to select upper command 0, got %d", s.cmdSelectedIdx)
	}
	if s.scrollY != scrollBefore {
		t.Fatalf("expected no scroll while stepping, scrollY %d -> %d", scrollBefore, s.scrollY)
	}

	// No visible command above -> scroll text, selection preserved while visible.
	s.Update(testutils.TestKeyUp)
	if s.scrollY != scrollBefore-1 {
		t.Fatalf("expected scroll up by one line, got scrollY=%d (was %d)", s.scrollY, scrollBefore)
	}
	if s.cmdSelectedIdx != 0 {
		t.Fatalf("expected selection preserved at 0 after scroll, got %d", s.cmdSelectedIdx)
	}
}

func TestSidebar_ArrowNavigationFallsBackToScrollWhileStreaming(t *testing.T) {
	s := NewSidebar()
	s.SetSize(80, 12)
	s.StartAssistantMessageWithContent(strings.Join([]string{
		"Use <cmd>git status</cmd>",
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
	}, "\n"))
	s.Show()
	s.FocusInput()
	s.SetStreaming(true)

	if s.commandSelectionEnabled() {
		t.Fatal("expected command selection to be disabled while streaming")
	}

	s.scrollY = 0
	s.follow = false
	s.Update(testutils.TestKeyDown)
	if s.scrollY != 1 {
		t.Fatalf("expected down to scroll one line while streaming, got scrollY=%d", s.scrollY)
	}
}

func lastVisibleCommandIndex(s *Sidebar) int {
	top := s.scrollY
	bottom := top + s.viewportHeight() - 1
	bestIdx := -1
	bestLine := -1
	for i, lineIdx := range s.cmdRenderedLines {
		if i >= len(s.cmdList) || lineIdx < top || lineIdx > bottom {
			continue
		}
		if lineIdx >= bestLine {
			bestLine = lineIdx
			bestIdx = i
		}
	}
	return bestIdx
}
