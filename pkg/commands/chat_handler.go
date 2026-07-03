package commands

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/logging"
)

const MaxChatHistoryMessages = 10
const chatThinkingPlaceholder = "Thinking..."

// ChatHandler handles the /chat command and interactive chat conversations.
type ChatHandler struct {
	// ApproverFactory builds the approver for each /chat invocation. Wired up
	// by the UI layer to surface a popup. Nil ⇒ AutoAllowApprover.
	ApproverFactory ApproverFactory

	// ContinuerFactory builds the continuer for each /chat invocation. Wired up
	// by the UI layer to surface a popup. Nil ⇒ AutoStopContinuer.
	ContinuerFactory ContinuerFactory
}

// Name returns the command name
func (h *ChatHandler) Name() string { return "/chat" }

// Description returns the command description
func (h *ChatHandler) Description() string { return "Toggle chat sidebar" }

// Execute returns the result indicating to toggle chat
func (h *ChatHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title:  "Chat",
		Action: ResultActionToggleChat,
	}
}

// StartChatStream builds context from messages + buffer and runs the agent
// loop. Tool-call lifecycle events are emitted on the returned channel.
func (h *ChatHandler) StartChatStream(
	ctx *Context,
	messages []ai.ChatMessage,
) (<-chan WtfStreamEvent, error) {
	return h.StartChatStreamWithContext(context.Background(), ctx, messages)
}

// StartChatStreamWithContext is like StartChatStream, but the caller owns the
// parent context so UI actions can cancel the active provider request or agent loop.
func (h *ChatHandler) StartChatStreamWithContext(
	runCtx context.Context,
	ctx *Context,
	messages []ai.ChatMessage,
) (<-chan WtfStreamEvent, error) {
	if runCtx == nil {
		runCtx = context.Background()
	}
	// Cap history to last N messages
	capped := messages
	if len(messages) > MaxChatHistoryMessages {
		capped = messages[len(messages)-MaxChatHistoryMessages:]
	}

	prep, err := prepareAgentRun(ctx, "chat")
	if err != nil {
		return nil, err
	}

	aiMessages := buildChatMessages(capped, ctx)
	toolDefs := prep.registry.Definitions()
	if len(toolDefs) > 0 && len(aiMessages) > 0 && aiMessages[0].Role == "system" {
		aiMessages[0].Content = ai.AppendToolInstructions(aiMessages[0].Content, toolDefs)
	}

	logger := slog.Default()
	if logger.Enabled(context.Background(), logging.LevelTrace) {
		logger.Log(
			context.Background(),
			logging.LevelTrace,
			"chat_stream_prompt",
			"model", prep.model,
			"message_count", len(aiMessages),
			"messages_full", buildMessageDump(aiMessages),
		)
	}

	req := ai.ChatRequest{
		Model:       prep.model,
		Messages:    aiMessages,
		Temperature: &prep.temperature,
		MaxTokens:   &prep.maxTokens,
		Tools:       toolDefs,
	}

	slog.Info("chat_stream_start",
		"model", prep.model,
		"message_count", len(aiMessages),
		"history_messages", len(messages),
		"capped_history", len(capped),
		"tools", len(toolDefs),
	)

	ch := make(chan WtfStreamEvent, 16)
	approver := h.resolveApprover(ch)
	continuer := h.resolveContinuer(ch)
	loopCtx, cancel := context.WithCancel(runCtx)
	go func() {
		defer cancel()
		RunAgentLoop(loopCtx, prep.provider, req, AgentLoopConfig{
			Registry:       prep.registry,
			Approver:       approver,
			Continuer:      continuer,
			MaxIterations:  prep.maxIterations,
			PerCallTimeout: time.Duration(prep.timeout) * time.Second,
			Tag:            "chat",
		}, ch)
	}()

	return ch, nil
}

func (h *ChatHandler) resolveApprover(ch chan<- WtfStreamEvent) Approver {
	if h.ApproverFactory != nil {
		if a := h.ApproverFactory(ch); a != nil {
			return a
		}
	}
	return AutoAllowApprover{}
}

func (h *ChatHandler) resolveContinuer(ch chan<- WtfStreamEvent) Continuer {
	if h.ContinuerFactory != nil {
		if c := h.ContinuerFactory(ch); c != nil {
			return c
		}
	}
	return AutoStopContinuer{}
}

// buildChatMessages constructs AI messages from chat history + terminal context.
func buildChatMessages(
	history []ai.ChatMessage,
	ctx *Context,
) []ai.Message {
	lines := ctx.GetLastNLines(ai.DefaultContextLines)

	// Use existing helper (pulls last command/exit code from session)
	meta := buildTerminalMetadata(ctx)

	// Use chat-specific context builder (background context framing, not diagnostic)
	termCtx := ai.BuildChatContext(lines, meta)

	// Build messages: single system message combining prompt + TTY context, then history
	msgs := []ai.Message{
		{Role: "system", Content: termCtx.SystemPrompt + "\n\n" + termCtx.UserPrompt},
	}

	// Append conversation history
	for _, msg := range history {
		// Skip ephemeral UI placeholder messages from prompt history.
		if msg.Role == "assistant" && strings.TrimSpace(msg.Content) == chatThinkingPlaceholder {
			continue
		}
		msgs = append(msgs, ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return msgs
}
