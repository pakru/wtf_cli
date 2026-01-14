package commands

import (
	"fmt"
	"strings"
)

// WtfHandler handles the /wtf command
type WtfHandler struct{}

func (h *WtfHandler) Name() string        { return "/wtf" }
func (h *WtfHandler) Description() string { return "Analyze last output and suggest fixes" }

func (h *WtfHandler) Execute(ctx *Context) *Result {
	// Get last 100 lines of output for analysis
	lines := ctx.GetLastNLines(100)
	if len(lines) == 0 {
		return &Result{
			Title:   "WTF Analysis",
			Content: "No terminal output to analyze yet.",
		}
	}

	// For now, return a placeholder result
	// In Phase 6, this will call the AI API
	return &Result{
		Title: "WTF Analysis",
		Content: fmt.Sprintf(
			"📊 Analyzing last %d lines...\n\n"+
				"⚠️ AI integration coming in Phase 6!\n\n"+
				"This command will:\n"+
				"• Analyze your terminal output\n"+
				"• Detect errors and issues\n"+
				"• Suggest fixes and solutions\n\n"+
				"Current directory: %s",
			len(lines), ctx.CurrentDir),
	}
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
