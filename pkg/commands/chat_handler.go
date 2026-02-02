package commands

import (
	"context"
	"log/slog"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/logging"
)

const MaxChatHistoryMessages = 10

// ChatHandler handles the /chat command and interactive chat conversations.
type ChatHandler struct{}

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

// StartChatStream builds context from messages + buffer and streams response.
func (h *ChatHandler) StartChatStream(
	ctx *Context,
	messages []ai.ChatMessage,
) (<-chan WtfStreamEvent, error) {
	// Cap history to last N messages
	capped := messages
	if len(messages) > MaxChatHistoryMessages {
		capped = messages[len(messages)-MaxChatHistoryMessages:]
	}

	// Build AI messages from history + fresh TTY context
	aiMessages := buildChatMessages(capped, ctx)

	// Load config
	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		slog.Error("chat_stream_config_error", "error", err)
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("chat_stream_config_invalid", "error", err)
		return nil, err
	}

	slog.Debug("chat_stream_provider_config", "llm_provider", cfg.LLMProvider)
	// Create provider
	provider, err := ai.GetProviderFromConfig(cfg)
	if err != nil {
		slog.Error("chat_stream_provider_error", "error", err)
		return nil, err
	}

	model, temperature, maxTokens, timeout := getProviderSettings(cfg)

	// Log request (trace level)
	logger := slog.Default()
	if logger.Enabled(context.Background(), logging.LevelTrace) {
		logger.Log(
			context.Background(),
			logging.LevelTrace,
			"chat_stream_prompt",
			"model", model,
			"message_count", len(aiMessages),
			"messages_full", buildMessageDump(aiMessages),
		)
	}

	// Build request (same as ExplainHandler pattern)
	req := ai.ChatRequest{
		Model:       model,
		Messages:    aiMessages,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	}

	slog.Info("chat_stream_start",
		"model", model,
		"message_count", len(aiMessages),
		"history_messages", len(messages),
		"capped_history", len(capped),
	)

	// Start streaming with timeout to prevent hanging
	streamCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	stream, err := provider.CreateChatCompletionStream(streamCtx, req)
	if err != nil {
		cancel()
		slog.Error("chat_stream_create_error", "error", err)
		return nil, err
	}

	ch := make(chan WtfStreamEvent, 8)

	go func() {
		defer close(ch)
		defer stream.Close()
		defer cancel() // Ensure the context is cancelled when the goroutine exits

		for stream.Next() {
			delta := stream.Content()
			if delta != "" {
				ch <- WtfStreamEvent{Delta: delta}
			}
		}

		if err := stream.Err(); err != nil {
			slog.Error("chat_stream_error", "error", err)
			ch <- WtfStreamEvent{Err: err, Done: true}
			return
		}

		slog.Info("chat_stream_done")
		ch <- WtfStreamEvent{Done: true}
	}()

	return ch, nil
}

// buildChatMessages constructs AI messages from chat history + terminal context.
func buildChatMessages(
	history []ai.ChatMessage,
	ctx *Context,
) []ai.Message {
	lines := ctx.GetLastNLines(ai.DefaultContextLines)

	// Use existing helper (pulls last command/exit code from session)
	meta := buildTerminalMetadata(ctx)

	// Use existing BuildTerminalContext which returns SystemPrompt + UserPrompt
	termCtx := ai.BuildTerminalContext(lines, meta)

	// Build messages: system + TTY context as developer message + history
	msgs := []ai.Message{
		{Role: "system", Content: termCtx.SystemPrompt},
		{Role: "developer", Content: termCtx.UserPrompt}, // TTY context
	}

	// Append conversation history
	for _, msg := range history {
		msgs = append(msgs, ai.Message{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	return msgs
}
