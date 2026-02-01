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
	"wtf_cli/pkg/logging"
	"wtf_cli/pkg/version"
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

	provider, err := ai.GetProviderFromConfig(cfg)
	if err != nil {
		slog.Error("wtf_stream_provider_error", "error", err)
		return nil, err
	}

	meta := buildTerminalMetadata(ctx)
	messages, termCtx := ai.BuildWtfMessages(lines, meta)

	model, temperature, maxTokens, timeout := getProviderSettings(cfg)

	logger := slog.Default()
	if logger.Enabled(context.Background(), logging.LevelTrace) {
		logger.Log(
			context.Background(),
			logging.LevelTrace,
			"wtf_stream_prompt",
			"model", model,
			"message_count", len(messages),
			"messages_full", buildMessageDump(messages),
		)
	}

	req := ai.ChatRequest{
		Model:       model,
		Messages:    messages,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	}

	slog.Info("wtf_stream_start",
		"model", model,
		"lines", len(lines),
		"cwd", ctx.CurrentDir,
		"temperature", temperature,
		"max_tokens", maxTokens,
	)
	slog.Debug("wtf_stream_request",
		"model", model,
		"message_count", len(messages),
		"messages_preview", buildMessagePreview(messages, llmLogMaxMessages, llmLogMessagePreviewChars),
		"messages_omitted", omittedCount(len(messages), llmLogMaxMessages),
		"context_lines", termCtx.LineCount,
		"context_truncated", termCtx.Truncated,
		"temperature", temperature,
		"max_tokens", maxTokens,
	)

	reqCtx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	stream, err := provider.CreateChatCompletionStream(reqCtx, req)
	if err != nil {
		cancel()
		slog.Error("wtf_stream_request_error", "error", err)
		return nil, err
	}

	slog.Info("wtf_stream_ready", "model", model)

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
				"model", model,
				"response_chars", totalRunes,
				"response_preview", sanitizeForLog(responsePreview.String()),
				"response_truncated", responsePreview.Truncated(),
				"error", err,
			)
			ch <- WtfStreamEvent{Err: err, Done: true}
			return
		}
		slog.Debug("wtf_stream_response",
			"model", model,
			"response_chars", totalRunes,
			"response_preview", sanitizeForLog(responsePreview.String()),
			"response_truncated", responsePreview.Truncated(),
		)
		ch <- WtfStreamEvent{Done: true}
	}()

	return ch, nil
}

func getProviderSettings(cfg config.Config) (model string, temperature float64, maxTokens int, timeout int) {
	switch cfg.LLMProvider {
	case "openai":
		model = cfg.Providers.OpenAI.Model
		if model == "" {
			model = "gpt-4o"
		}
		temperature = cfg.Providers.OpenAI.Temperature
		maxTokens = cfg.Providers.OpenAI.MaxTokens
		timeout = cfg.Providers.OpenAI.APITimeoutSeconds
		if timeout <= 0 {
			timeout = 30
		}
	case "copilot":
		model = cfg.Providers.Copilot.Model
		if model == "" {
			model = "gpt-4o"
		}
		temperature = cfg.Providers.Copilot.Temperature
		maxTokens = cfg.Providers.Copilot.MaxTokens
		timeout = cfg.Providers.Copilot.APITimeoutSeconds
		if timeout <= 0 {
			timeout = 30
		}
	case "anthropic":
		model = cfg.Providers.Anthropic.Model
		if model == "" {
			model = "claude-3-5-sonnet-20241022"
		}
		temperature = cfg.Providers.Anthropic.Temperature
		maxTokens = cfg.Providers.Anthropic.MaxTokens
		timeout = cfg.Providers.Anthropic.APITimeoutSeconds
		if timeout <= 0 {
			timeout = 60
		}
	default:
		model = cfg.OpenRouter.Model
		temperature = cfg.OpenRouter.Temperature
		maxTokens = cfg.OpenRouter.MaxTokens
		timeout = cfg.OpenRouter.APITimeoutSeconds
	}
	return
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

func buildMessageDump(messages []ai.Message) string {
	if len(messages) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, msg := range messages {
		role := strings.TrimSpace(msg.Role)
		if role == "" {
			role = "unknown"
		}
		sb.WriteString(role)
		sb.WriteString(":\n")
		sb.WriteString(msg.Content)
		if i < len(messages)-1 {
			sb.WriteString("\n\n---\n\n")
		}
	}
	return sb.String()
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
	return &Result{
		Title:  "History",
		Action: ResultActionOpenHistoryPicker,
	}
}

// SettingsHandler handles the /settings command
type SettingsHandler struct{}

func (h *SettingsHandler) Name() string        { return "/settings" }
func (h *SettingsHandler) Description() string { return "Open settings panel" }

func (h *SettingsHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title:  "Settings",
		Action: ResultActionOpenSettings,
	}
}

// HelpHandler handles the /help command
type HelpHandler struct{}

func (h *HelpHandler) Name() string        { return "/help" }
func (h *HelpHandler) Description() string { return "Show help" }

func (h *HelpHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title: "Help",
		Content: fmt.Sprintf(`📚 WTF CLI Help

Version: %s

Available Commands:
  /chat     - Toggle chat sidebar
  /explain  - Analyze last output and suggest fixes
  /history  - Show command history
  /help     - Show this help

Shortcuts:
  Ctrl+T    - Toggle chat sidebar
  Ctrl+R    - Search command history
  Ctrl+C    - Cancel current command
  Ctrl+D    - Exit terminal (press twice)
  /         - Open command palette (at empty prompt)
  Esc       - Close command palette or result

Press Esc to close this panel.`,
			version.Summary()),
	}
}
