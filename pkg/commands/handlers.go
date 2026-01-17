package commands

import (
	"context"
	"fmt"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"
)

// WtfHandler handles the /wtf command
type WtfHandler struct{}

func (h *WtfHandler) Name() string        { return "/wtf" }
func (h *WtfHandler) Description() string { return "Analyze last output and suggest fixes" }

func (h *WtfHandler) Execute(ctx *Context) *Result {
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

// StartStream streams the /wtf response using the OpenRouter provider.
func (h *WtfHandler) StartStream(ctx *Context) (<-chan WtfStreamEvent, error) {
	lines := ctx.GetLastNLines(ai.DefaultContextLines)
	if len(lines) == 0 {
		return nil, nil
	}

	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	provider, err := ai.NewOpenRouterProvider(cfg.OpenRouter)
	if err != nil {
		return nil, err
	}

	meta := buildTerminalMetadata(ctx)
	messages, _ := ai.BuildWtfMessages(lines, meta)

	temperature := cfg.OpenRouter.Temperature
	maxTokens := cfg.OpenRouter.MaxTokens
	req := ai.ChatRequest{
		Model:       cfg.OpenRouter.Model,
		Messages:    messages,
		Temperature: &temperature,
		MaxTokens:   &maxTokens,
	}

	reqCtx, cancel := context.WithTimeout(context.Background(), time.Duration(cfg.OpenRouter.APITimeoutSeconds)*time.Second)
	stream, err := provider.CreateChatCompletionStream(reqCtx, req)
	if err != nil {
		cancel()
		return nil, err
	}

	ch := make(chan WtfStreamEvent, 8)
	go func() {
		defer close(ch)
		defer cancel()
		defer stream.Close()

		for stream.Next() {
			delta := stream.Content()
			if delta != "" {
				ch <- WtfStreamEvent{Delta: delta}
			}
		}

		if err := stream.Err(); err != nil {
			ch <- WtfStreamEvent{Err: err, Done: true}
			return
		}
		ch <- WtfStreamEvent{Done: true}
	}()

	return ch, nil
}

func buildTerminalMetadata(ctx *Context) ai.TerminalMetadata {
	meta := ai.TerminalMetadata{
		WorkingDir: ctx.CurrentDir,
		ExitCode:   ctx.LastExitCode,
	}
	if ctx.Session != nil {
		last := ctx.Session.GetLastN(1)
		if len(last) > 0 {
			meta.LastCommand = last[0].Command
			meta.ExitCode = last[0].ExitCode
		}
	}
	return meta
}

// ExplainHandler handles the /explain command
type ExplainHandler struct{}

func (h *ExplainHandler) Name() string        { return "/explain" }
func (h *ExplainHandler) Description() string { return "Explain what the last command did" }

func (h *ExplainHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title: "Explain",
		Content: "🔍 Explain command\n\n" +
			"⚠️ AI integration coming in Phase 6!\n\n" +
			"This command will explain what your last command did\n" +
			"and break down the output.",
	}
}

// FixHandler handles the /fix command
type FixHandler struct{}

func (h *FixHandler) Name() string        { return "/fix" }
func (h *FixHandler) Description() string { return "Suggest fix for last error" }

func (h *FixHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title: "Fix Suggestion",
		Content: "🔧 Fix command\n\n" +
			"⚠️ AI integration coming in Phase 6!\n\n" +
			"This command will analyze errors and suggest fixes.",
	}
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

// CloseSidebarHandler handles the /close_sidebar command.
type CloseSidebarHandler struct{}

func (h *CloseSidebarHandler) Name() string        { return "/close_sidebar" }
func (h *CloseSidebarHandler) Description() string { return "Close AI sidebar" }

func (h *CloseSidebarHandler) Execute(ctx *Context) *Result {
	return &Result{
		Title:   "__CLOSE_SIDEBAR__",
		Content: "",
	}
}

// ModelsHandler handles the /models command
type ModelsHandler struct{}

func (h *ModelsHandler) Name() string        { return "/models" }
func (h *ModelsHandler) Description() string { return "Show available models" }

func (h *ModelsHandler) Execute(ctx *Context) *Result {
	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		return &Result{
			Title:   "Models",
			Content: fmt.Sprintf("Failed to load config: %v", err),
		}
	}

	cachePath := ai.DefaultModelCachePath()
	reqCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	cache, err := ai.RefreshOpenRouterModelCache(reqCtx, cfg.OpenRouter.APIURL, cachePath)
	note := ""
	if err != nil {
		cached, cacheErr := ai.LoadModelCache(cachePath)
		if cacheErr != nil {
			return &Result{
				Title: "Models",
				Content: fmt.Sprintf(
					"Failed to refresh models: %v\nAlso failed to read cache: %v",
					err, cacheErr,
				),
			}
		}
		cache = cached
		if !cache.UpdatedAt.IsZero() {
			note = fmt.Sprintf("Warning: refresh failed (%v). Showing cached list from %s.", err, formatTimestamp(cache.UpdatedAt))
		} else {
			note = fmt.Sprintf("Warning: refresh failed (%v). Showing cached list.", err)
		}
	}

	return &Result{
		Title:   "Models",
		Content: formatModelList(cache, note),
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
  /wtf      - Analyze last output and suggest fixes
  /explain  - Explain what the last command did
  /fix      - Suggest fix for last error
  /history  - Show command history
  /models   - Show available models
  /close_sidebar - Close AI sidebar
  /help     - Show this help

Shortcuts:
  Ctrl+D    - Exit terminal
  Ctrl+C    - Cancel current command
  Ctrl+Z    - Suspend process
  /         - Open command palette (at empty prompt)
  Esc       - Close command palette or result

Press Esc to close this panel.`,
	}
}

func formatModelList(cache ai.ModelCache, note string) string {
	var sb strings.Builder

	if note != "" {
		sb.WriteString(note)
		sb.WriteString("\n\n")
	}

	sb.WriteString(fmt.Sprintf("Models (%d)\n", len(cache.Models)))
	if !cache.UpdatedAt.IsZero() {
		sb.WriteString("Updated: " + formatTimestamp(cache.UpdatedAt) + "\n")
	}
	sb.WriteString("\n")

	for _, model := range cache.Models {
		sb.WriteString(formatModelLine(model))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatModelLine(model ai.ModelInfo) string {
	name := strings.TrimSpace(model.Name)
	label := model.ID
	if name != "" && name != model.ID {
		label = fmt.Sprintf("%s - %s", model.ID, name)
	}

	contextInfo := ""
	if model.ContextLength > 0 {
		contextInfo = fmt.Sprintf("ctx:%d", model.ContextLength)
	}

	pricingInfo := formatPricing(model.Pricing)

	parts := []string{label}
	if contextInfo != "" {
		parts = append(parts, contextInfo)
	}
	if pricingInfo != "" {
		parts = append(parts, pricingInfo)
	}

	return strings.Join(parts, " | ")
}

func formatPricing(pricing map[string]string) string {
	if len(pricing) == 0 {
		return ""
	}

	parts := make([]string, 0, 2)
	if value := strings.TrimSpace(pricing["prompt"]); value != "" {
		parts = append(parts, "prompt="+value)
	}
	if value := strings.TrimSpace(pricing["completion"]); value != "" {
		parts = append(parts, "completion="+value)
	}

	if len(parts) == 0 {
		return ""
	}

	return strings.Join(parts, ", ")
}

func formatTimestamp(ts time.Time) string {
	return ts.Local().Format("2006-01-02 15:04")
}
