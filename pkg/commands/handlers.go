package commands

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/tools"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/logging"
	"wtf_cli/pkg/version"
)

// ApproverFactory builds an Approver bound to the per-invocation event
// channel. Used by handlers so the UI can inject a popup-driven approver
// (which needs to send ToolApproval events on the same channel the loop reads
// from) without changing the StartStream signature.
//
// When nil, handlers fall back to AutoAllowApprover (suitable for tests and
// headless flows).
type ApproverFactory func(out chan<- WtfStreamEvent) Approver

// ExplainHandler handles the /explain command.
type ExplainHandler struct {
	// ApproverFactory builds the approver for each /explain invocation. Wired
	// up by the UI layer to surface a popup. Nil ⇒ AutoAllowApprover.
	ApproverFactory ApproverFactory
}

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

// WtfStreamEvent represents a streaming event from the agent loop.
//
// Most events carry exactly one populated field. Receivers should check fields
// in this order: Err, ToolApproval, ToolCallStart, ToolCallFinished, Delta,
// Done. Unknown future variants must be ignored gracefully (no field set ⇒
// keep listening).
type WtfStreamEvent struct {
	Delta string
	Done  bool
	Err   error

	// Tool-call lifecycle events. nil unless the agent loop is reporting on a
	// tool call this iteration.
	ToolCallStart    *ToolCallInfo
	ToolApproval     *ApprovalRequest
	ToolCallFinished *ToolCallInfo
}

// ToolCallInfo carries metadata about a single tool invocation for the UI.
//
// On a ToolCallStart event Result is empty and Duration is zero. On a
// ToolCallFinished event they are populated. Denied is set when the user
// denied the call; ErrorMessage is set when the tool itself returned a soft
// error or could not be found.
type ToolCallInfo struct {
	ID           string
	Name         string
	ArgsJSON     string
	Result       string
	Duration     time.Duration
	Denied       bool
	ErrorMessage string
}

// StreamingHandler exposes a streaming command interface.
type StreamingHandler interface {
	Handler
	StartStream(ctx *Context) (<-chan WtfStreamEvent, error)
}

// StartStream streams the /explain response. The agent loop may invoke tools
// (e.g. read_file) between turns; tool-call lifecycle events are emitted on
// the returned channel for the UI to surface.
func (h *ExplainHandler) StartStream(ctx *Context) (<-chan WtfStreamEvent, error) {
	lines := ctx.GetLastNLines(ai.DefaultContextLines)
	if len(lines) == 0 {
		slog.Info("wtf_stream_skip", "reason", "no_output")
		return nil, nil
	}

	prep, err := prepareAgentRun(ctx, "explain")
	if err != nil {
		return nil, err
	}

	meta := buildTerminalMetadata(ctx)
	messages, termCtx := ai.BuildWtfMessages(lines, meta)

	toolDefs := prep.registry.Definitions()
	if len(toolDefs) > 0 && len(messages) > 0 && messages[0].Role == "system" {
		messages[0].Content = ai.AppendToolInstructions(messages[0].Content, toolDefs)
	}

	logger := slog.Default()
	if logger.Enabled(context.Background(), logging.LevelTrace) {
		logger.Log(
			context.Background(),
			logging.LevelTrace,
			"wtf_stream_prompt",
			"model", prep.model,
			"message_count", len(messages),
			"messages_full", buildMessageDump(messages),
		)
	}

	req := ai.ChatRequest{
		Model:       prep.model,
		Messages:    messages,
		Temperature: &prep.temperature,
		MaxTokens:   &prep.maxTokens,
		Tools:       toolDefs,
	}

	slog.Info("wtf_stream_start",
		"model", prep.model,
		"lines", len(lines),
		"cwd", ctx.CurrentDir,
		"temperature", prep.temperature,
		"max_tokens", prep.maxTokens,
		"tools", len(toolDefs),
	)
	slog.Debug("wtf_stream_request",
		"model", prep.model,
		"message_count", len(messages),
		"messages_preview", buildMessagePreview(messages, llmLogMaxMessages, llmLogMessagePreviewChars),
		"messages_omitted", omittedCount(len(messages), llmLogMaxMessages),
		"context_lines", termCtx.LineCount,
		"context_truncated", termCtx.Truncated,
		"temperature", prep.temperature,
		"max_tokens", prep.maxTokens,
		"tools", len(toolDefs),
	)

	ch := make(chan WtfStreamEvent, 16)
	approver := h.resolveApprover(ch)
	loopCtx, cancel := context.WithCancel(context.Background())
	go func() {
		defer cancel()
		RunAgentLoop(loopCtx, prep.provider, req, AgentLoopConfig{
			Registry:       prep.registry,
			Approver:       approver,
			MaxIterations:  prep.maxIterations,
			PerCallTimeout: time.Duration(prep.timeout) * time.Second,
			Tag:            "explain",
		}, ch)
	}()

	return ch, nil
}

func (h *ExplainHandler) resolveApprover(ch chan<- WtfStreamEvent) Approver {
	if h.ApproverFactory != nil {
		if a := h.ApproverFactory(ch); a != nil {
			return a
		}
	}
	return AutoAllowApprover{}
}

// agentRunPrep bundles the provider, settings, and tool registry needed to
// kick off an agent loop. Built once per /explain or /chat invocation.
type agentRunPrep struct {
	provider      ai.Provider
	registry      *tools.Registry
	model         string
	temperature   float64
	maxTokens     int
	timeout       int
	maxIterations int
}

// prepareAgentRun loads config, builds the provider, resolves provider
// settings, and constructs the per-invocation tool registry. Tag is used in
// slog records (e.g. "explain", "chat").
func prepareAgentRun(ctx *Context, tag string) (*agentRunPrep, error) {
	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		slog.Error(tag+"_stream_config_error", "error", err)
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		slog.Error(tag+"_stream_config_invalid", "error", err)
		return nil, err
	}

	slog.Debug(tag+"_stream_provider_config", "llm_provider", cfg.LLMProvider)
	provider, err := ai.GetProviderFromConfig(cfg)
	if err != nil {
		slog.Error(tag+"_stream_provider_error", "error", err)
		return nil, err
	}

	model, temperature, maxTokens, timeout := getProviderSettings(cfg)
	registry := buildToolRegistry(cfg, ctx.CurrentDir)

	return &agentRunPrep{
		provider:      provider,
		registry:      registry,
		model:         model,
		temperature:   temperature,
		maxTokens:     maxTokens,
		timeout:       timeout,
		maxIterations: cfg.Agent.MaxIterations,
	}, nil
}

// buildToolRegistry constructs the per-invocation tool registry from config.
//
// cwd is snapshotted at agent-loop start so a mid-stream `cd` in the user's
// shell does not change which directory tools see. read_file enforces cwd
// containment against this value.
func buildToolRegistry(cfg config.Config, cwd string) *tools.Registry {
	registry := tools.NewRegistry()
	if cfg.Agent.Tools.ReadFile.Enabled {
		registry.Register(tools.NewReadFile(
			cwd,
			cfg.Agent.Tools.ReadFile.MaxLines,
			cfg.Agent.Tools.ReadFile.MaxBytes,
		))
	}
	return registry
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
	case "google":
		model = cfg.Providers.Google.Model
		if model == "" {
			model = "gemini-3-flash-preview"
		}
		temperature = cfg.Providers.Google.Temperature
		maxTokens = cfg.Providers.Google.MaxTokens
		timeout = cfg.Providers.Google.APITimeoutSeconds
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
  Ctrl+T     - Toggle chat sidebar
  Shift+Tab  - Switch focus to chat panel
  Ctrl+R     - Search command history
  Ctrl+C    - Cancel current command
  Ctrl+D    - Exit terminal (press twice)
  /         - Open command palette (at empty prompt)
  Esc       - Close command palette or result

Press Esc to close this panel.`,
			version.Summary()),
	}
}
