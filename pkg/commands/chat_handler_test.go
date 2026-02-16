package commands

import (
	"testing"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"
)

func TestChatHandler_buildChatMessages_Empty(t *testing.T) {
	ctx := NewContext(buffer.New(100), nil, "/tmp")
	messages := buildChatMessages([]ai.ChatMessage{}, ctx)

	// Should have system + developer messages (TTY context)
	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages (system + developer), got %d", len(messages))
	}

	if messages[0].Role != "system" {
		t.Errorf("Expected first message to be 'system', got %q", messages[0].Role)
	}

	if messages[1].Role != "developer" {
		t.Errorf("Expected second message to be 'developer', got %q", messages[1].Role)
	}
}

func TestChatHandler_buildChatMessages_WithHistory(t *testing.T) {
	ctx := NewContext(buffer.New(100), nil, "/tmp")
	history := []ai.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	messages := buildChatMessages(history, ctx)

	// Should have system + developer + 2 history messages
	if len(messages) != 4 {
		t.Fatalf("Expected 4 messages, got %d", len(messages))
	}

	// System and developer first
	if messages[0].Role != "system" {
		t.Errorf("Expected first message to be 'system', got %q", messages[0].Role)
	}
	if messages[1].Role != "developer" {
		t.Errorf("Expected second message to be 'developer', got %q", messages[1].Role)
	}

	// Then history
	if messages[2].Role != "user" || messages[2].Content != "Hello" {
		t.Error("User message not preserved in history")
	}
	if messages[3].Role != "assistant" || messages[3].Content != "Hi there" {
		t.Error("Assistant message not preserved in history")
	}
}

func TestChatHandler_buildChatMessages_FiltersThinkingPlaceholder(t *testing.T) {
	ctx := NewContext(buffer.New(100), nil, "/tmp")
	history := []ai.ChatMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Thinking..."},
		{Role: "assistant", Content: "Real answer"},
	}

	messages := buildChatMessages(history, ctx)

	// system + developer + user + real assistant
	if len(messages) != 4 {
		t.Fatalf("Expected 4 messages, got %d", len(messages))
	}
	if messages[2].Role != "user" || messages[2].Content != "Hello" {
		t.Fatalf("Expected preserved user message, got role=%q content=%q", messages[2].Role, messages[2].Content)
	}
	if messages[3].Role != "assistant" || messages[3].Content != "Real answer" {
		t.Fatalf("Expected preserved assistant message, got role=%q content=%q", messages[3].Role, messages[3].Content)
	}
}

func TestChatHandler_MessageCapping(t *testing.T) {
	// Create > MaxChatHistoryMessages (15 total, cap is 10)
	history := make([]ai.ChatMessage, MaxChatHistoryMessages+5)
	for i := range history {
		history[i] = ai.ChatMessage{
			Role:    "user",
			Content: "Message",
		}
	}

	ctx := NewContext(buffer.New(100), nil, "/tmp")
	messages := buildChatMessages(history, ctx)

	// StartChatStream caps to last MaxChatHistoryMessages before calling buildChatMessages
	// But test calls buildChatMessages directly with 15 messages
	// buildChatMessages adds system + developer + all input history
	// So: 2 (system + developer) + 15 (all input history) = 17
	expectedCount := 2 + len(history) // 2 + 15 = 17
	if len(messages) != expectedCount {
		t.Errorf("Expected %d messages, got %d", expectedCount, len(messages))
	}
}

func TestChatHandler_MessageCapping_ExactLimit(t *testing.T) {
	// Create exactly MaxChatHistoryMessages
	history := make([]ai.ChatMessage, MaxChatHistoryMessages)
	for i := range history {
		history[i] = ai.ChatMessage{
			Role:    "user",
			Content: "Message",
		}
	}

	ctx := NewContext(buffer.New(100), nil, "/tmp")
	messages := buildChatMessages(history, ctx)

	// Should have system + developer + MaxChatHistoryMessages
	expectedCount := 2 + MaxChatHistoryMessages
	if len(messages) != expectedCount {
		t.Errorf("Expected %d messages, got %d", expectedCount, len(messages))
	}
}

func TestChatHandler_MessageCapping_BelowLimit(t *testing.T) {
	// Create fewer than MaxChatHistoryMessages
	history := make([]ai.ChatMessage, 3)
	for i := range history {
		history[i] = ai.ChatMessage{
			Role:    "user",
			Content: "Message",
		}
	}

	ctx := NewContext(buffer.New(100), nil, "/tmp")
	messages := buildChatMessages(history, ctx)

	// Should have system + developer + 3
	if len(messages) != 5 {
		t.Errorf("Expected 5 messages, got %d", len(messages))
	}
}

func TestChatHandler_ContextBuilding(t *testing.T) {
	buf := buffer.New(100)
	buf.Write([]byte("test output line 1"))
	buf.Write([]byte("test output line 2"))

	sess := capture.NewSessionContext()
	sess.AddCommand(capture.CommandRecord{
		Command:    "ls -la",
		WorkingDir: "/home/user",
		ExitCode:   0,
	})

	ctx := NewContext(buf, sess, "/home/user")
	messages := buildChatMessages([]ai.ChatMessage{}, ctx)

	// Should have system + developer messages with TTY context
	if len(messages) < 2 {
		t.Fatalf("Expected at least 2 messages, got %d", len(messages))
	}

	// Developer message should contain TTY context
	developerMsg := messages[1]
	if developerMsg.Role != "developer" {
		t.Errorf("Expected developer role, got %q", developerMsg.Role)
	}

	// Should have some content (terminal output context)
	if len(developerMsg.Content) == 0 {
		t.Error("Expected developer message to have TTY context content")
	}
}

func TestBuildTerminalMetadata_WithSession(t *testing.T) {
	sess := capture.NewSessionContext()
	sess.AddCommand(capture.CommandRecord{
		Command:    "git status",
		WorkingDir: "/home/project",
		ExitCode:   0,
	})

	// Pass empty currentDir so it uses command record's working dir
	ctx := NewContext(buffer.New(100), sess, "")
	meta := buildTerminalMetadata(ctx)

	if meta.LastCommand != "git status" {
		t.Errorf("Expected 'git status', got %q", meta.LastCommand)
	}
	if meta.ExitCode != 0 {
		t.Errorf("Expected exit code 0, got %d", meta.ExitCode)
	}
	// buildTerminalMetadata uses command record's working dir when ctx.CurrentDir is empty
	if meta.WorkingDir != "/home/project" {
		t.Errorf("Expected '/home/project' from command record, got %q", meta.WorkingDir)
	}
}

func TestBuildTerminalMetadata_NoSession(t *testing.T) {
	ctx := NewContext(buffer.New(100), nil, "/tmp")
	meta := buildTerminalMetadata(ctx)

	if meta.WorkingDir != "/tmp" {
		t.Errorf("Expected '/tmp', got %q", meta.WorkingDir)
	}
	if meta.ExitCode != -1 {
		t.Errorf("Expected exit code -1, got %d", meta.ExitCode)
	}
}

func TestChatHandler_Name(t *testing.T) {
	h := &ChatHandler{}
	if h.Name() != "/chat" {
		t.Errorf("Expected '/chat', got %q", h.Name())
	}
}

func TestChatHandler_Description(t *testing.T) {
	h := &ChatHandler{}
	expected := "Toggle chat sidebar"
	if h.Description() != expected {
		t.Errorf("Expected %q, got %q", expected, h.Description())
	}
}

func TestChatHandler_Execute(t *testing.T) {
	h := &ChatHandler{}
	ctx := NewContext(buffer.New(100), nil, "/tmp")

	result := h.Execute(ctx)

	if result == nil {
		t.Fatal("Expected result, got nil")
	}
	if result.Title != "Chat" {
		t.Errorf("Expected title 'Chat', got %q", result.Title)
	}
	if result.Action != ResultActionToggleChat {
		t.Errorf("Expected action ResultActionToggleChat, got %q", result.Action)
	}
}
