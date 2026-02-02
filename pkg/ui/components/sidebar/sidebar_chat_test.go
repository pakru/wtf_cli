package sidebar

import (
	"fmt"
	"runtime"
	"strings"
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ui/components/testutils"
)

func TestSidebar_ChatMode(t *testing.T) {
	s := NewSidebar()

	// Initially not in chat mode
	if s.IsChatMode() {
		t.Error("Expected chat mode to be false initially")
	}

	// Enable chat mode
	s.EnableChatMode()
	if !s.IsChatMode() {
		t.Error("Expected chat mode to be true after EnableChatMode()")
	}
}

func TestSidebar_AppendUserMessage(t *testing.T) {
	s := NewSidebar()
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

	// EnableChatMode sets initial focus to input
	if !s.IsFocusedOnInput() {
		t.Error("Expected focus on input initially after EnableChatMode()")
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
	s.EnableChatMode()

	s.FocusInput()
	if !s.IsFocusedOnInput() {
		t.Error("Expected focus on input after FocusInput()")
	}
}

func TestSidebar_SubmitMessage(t *testing.T) {
	s := NewSidebar()
	s.EnableChatMode()
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
	s.EnableChatMode()
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
	s.EnableChatMode()

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
	s.EnableChatMode()

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
	s.EnableChatMode()

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

func TestSidebar_ChatMode_ScrollKeysScrollViewport(t *testing.T) {
	s := NewSidebar()
	s.EnableChatMode()
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
