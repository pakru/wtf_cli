package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
)

// ExplainHandler handles the /explain command.
type ExplainHandler struct{}

func (h *ExplainHandler) Name() string        { return "/explain" }
func (h *ExplainHandler) Description() string { return "Analyze last output and suggest fixes" }

func (h *ExplainHandler) Execute(ctx *Context) *Result {
	// Get last 100 lines of output for analysis
	lines := ctx.GetLastNLines(ai.DefaultContextLines)
	if len(lines) == 0 {
		return &Result{
			Title:   "WTF Analysis",
			Content: "No terminal output to analyze yet.",
		}
	}

	return &Result{
		Title:   "WTF Analysis",
		Content: "Loading...",
	}
}

// WtfStreamEvent represents a streaming event from the LLM.
type WtfStreamEvent struct {
	Delta string
	Done  bool
	Err   error
}

// StreamingHandler exposes a streaming command interface.
type StreamingHandler interface {
	Handler
	StartStream(ctx *Context) (<-chan WtfStreamEvent, error)
}

// StartStream streams the /explain response using the OpenRouter provider.
func (h *ExplainHandler) StartStream(ctx *Context) (<-chan WtfStreamEvent, error) {
	lines := ctx.GetLastNLines(ai.DefaultContextLines)
	if len(lines) == 0 {
		slog.Info("wtf_stream_skip", "reason", "no_output")
		return nil, nil
	}

	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		slog.Error("wtf_stream_config_error", "error", err)
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		slog.Error("wtf_stream_config_invalid", "error", err)
		return nil, err
	}

	provider, err := ai.NewOpenRouterProvider(cfg.OpenRouter)
	if err != nil {
		slog.Error("wtf_stream_provider_error", "error", err)
		return nil, err
	}

	meta := buildTerminalMetadata(ctx)
	messages, termCtx := ai.BuildWtfMessages(lines, meta)

	temperature := cfg.OpenRouter.Temperature
	maxTokens := cfg.OpenRouter.MaxTokens
	req := ai.ChatRequest{
		Model:       cfg.OpenRouter.Model,
		Messages:    messages,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	}

	slog.Info("wtf_stream_start",
		"model", cfg.OpenRouter.Model,
		"lines", len(lines),
		"cwd", ctx.CurrentDir,
		"temperature", temperature,
		"max_tokens", maxTokens,
	)
	slog.Debug("wtf_stream_request",
		"model", cfg.OpenRouter.Model,
		"message_count", len(messages),
		"messages_preview", buildMessagePreview(messages, llmLogMaxMessages, llmLogMessagePreviewChars),
		"messages_omitted", omittedCount(len(messages), llmLogMaxMessages),
		"context_lines", termCtx.LineCount,
		"context_truncated", termCtx.Truncated,
		"temperature", temperature,
		"max_tokens", maxTokens,
	)

	reqCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.OpenRouter.APITimeoutSeconds)*time.Second)
	stream, err := provider.CreateChatCompletionStream(reqCtx, req)
	if err != nil {
		cancel()
		slog.Error("wtf_stream_request_error", "error", err)
		return nil, err
	}

	slog.Info("wtf_stream_ready", "model", cfg.OpenRouter.Model)

	ch := make(chan WtfStreamEvent, 8)
	go func() {
		defer close(ch)
		defer cancel()
		defer stream.Close()

		responsePreview := newLogPreview(llmLogResponsePreviewChars)
		totalRunes := 0

		for stream.Next() {
			delta := stream.Content()
			if delta != "" {
				responsePreview.Append(delta)
				totalRunes += utf8.RuneCountInString(delta)
				ch <- WtfStreamEvent{Delta: delta}
			}
		}

		if err := stream.Err(); err != nil {
			slog.Debug("wtf_stream_response",
				"model", cfg.OpenRouter.Model,
				"response_chars", totalRunes,
				"response_preview", sanitizeForLog(responsePreview.String()),
				"response_truncated", responsePreview.Truncated(),
				"error", err,
			)
			ch <- WtfStreamEvent{Err: err, Done: true}
			return
		}
		slog.Debug("wtf_stream_response",
			"model", cfg.OpenRouter.Model,
			"response_chars", totalRunes,
			"response_preview", sanitizeForLog(responsePreview.String()),
			"response_truncated", responsePreview.Truncated(),
		)
		ch <- WtfStreamEvent{Done: true}
	}()

	return ch, nil
}

func buildTerminalMetadata(ctx *Context) ai.TerminalMetadata {
	meta := ai.TerminalMetadata{
		WorkingDir: ctx.CurrentDir,
		ExitCode:   -1,
	}
	if ctx.Session != nil {
		if meta.WorkingDir == "" {
			meta.WorkingDir = ctx.Session.GetCurrentDir()
		}
		last := ctx.Session.GetLastN(1)
		if len(last) > 0 {
			meta.LastCommand = last[0].Command
			meta.ExitCode = last[0].ExitCode
			if meta.WorkingDir == "" {
				meta.WorkingDir = last[0].WorkingDir
			}
		}
	}
	return meta
}

const (
	llmLogMaxMessages          = 6
	llmLogMessagePreviewChars  = 400
	llmLogResponsePreviewChars = 2000
)

func buildMessagePreview(messages []ai.Message, maxMessages, maxChars int) []string {
	if maxMessages <= 0 || maxChars <= 0 {
		return nil
	}
	if len(messages) == 0 {
		return nil
	}

	limit := len(messages)
	if limit > maxMessages {
		limit = maxMessages
	}

	preview := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		msg := messages[i]
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "unknown"
		}
		content := strings.TrimSpace(msg.Content)
		content, truncated := tailForLog(content, maxChars)
		content = sanitizeForLog(content)
		if truncated {
			content = "..." + content
		}
		preview = append(preview, fmt.Sprintf("%s: %s", role, content))
	}

	return preview
}

func omittedCount(total, max int) int {
	if max <= 0 || total <= max {
		return 0
	}
	return total - max
}

func tailForLog(text string, maxChars int) (string, bool) {
	if maxChars <= 0 || text == "" {
		return "", text != ""
	}
	runes := []rune(text)
	if len(runes) <= maxChars {
		return text, false
	}
	return string(runes[len(runes)-maxChars:]), true
}

var logSanitizer = strings.NewReplacer("\r", "\\r", "\n", "\\n", "\t", "\\t")

func sanitizeForLog(text string) string {
	if text == "" {
		return ""
	}
	return logSanitizer.Replace(text)
}

type logPreview struct {
	max       int
	runes     []rune
	truncated bool
}

func newLogPreview(max int) *logPreview {
	return &logPreview{max: max}
}

func (p *logPreview) Append(text string) {
	if p == nil || p.truncated || p.max <= 0 || text == "" {
		return
	}
	for _, r := range text {
		if len(p.runes) >= p.max {
			p.truncated = true
			return
		}
		p.runes = append(p.runes, r)
	}
}

func (p *logPreview) String() string {
	if p == nil {
		return ""
	}
	return string(p.runes)
}

func (p *logPreview) Truncated() bool {
	if p == nil {
		return false
	}
	return p.truncated
}

// HistoryHandler handles the /history command
type HistoryHandler struct{}

func (h *HistoryHandler) Name() string        { return "/history" }
func (h *HistoryHandler) Description() string { return "Show command history" }

func (h *HistoryHandler) Execute(ctx *Context) *Result {
	if ctx.Session == nil {
		return &Result{
			Title:   "History",
			Content: "No session history available.",
		}
	}

	history := ctx.Session.GetHistory()
	if len(history) == 0 {
		return &Result{
			Title:   "History",
			Content: "No commands in history yet.",
		}
	}

	var sb strings.Builder
	sb.WriteString("📜 Command History\n\n")
	for i, entry := range history {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, entry.Command))
	}

	return &Result{
		Title:   "History",
		Content: sb.String(),
	}
}

// SettingsHandler handles the /settings command
type SettingsHandler struct{}

func (h *SettingsHandler) Name() string        { return "/settings" }
func (h *SettingsHandler) Description() string { return "Open settings panel" }

func (h *SettingsHandler) Execute(ctx *Context) *Result {
	// Special marker to tell UI to open settings panel
	return &Result{
		Title:   "__OPEN_SETTINGS__",
		Content: "",
	}
}

// HelpHandler handles the /help command
type HelpHandler struct{}

func (h *HelpHandler) Name() string        { return "/help" }
func (h *HelpHandler) Description() string { return "Show help" }

func (h *HelpHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title: "Help",
		Content: `📚 WTF CLI Help

Available Commands:
  /explain  - Analyze last output and suggest fixes
  /history  - Show command history
  /help     - Show this help

Shortcuts:
  Ctrl+D    - Exit terminal (press twice)
  Ctrl+C    - Cancel current command
  Ctrl+Z    - Suspend process
  /         - Open command palette (at empty prompt)
  Esc       - Close command palette or result

Press Esc to close this panel.`,
	}
}
