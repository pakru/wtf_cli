package commands

import (
	"log/slog"
	"time"
)

// ResultAction indicates a UI side-effect to trigger for a command result.
type ResultAction string

const (
	ResultActionOpenHistoryPicker ResultAction = "open_history_picker"
	ResultActionOpenSettings      ResultAction = "open_settings"
)

// Result represents the result of a command execution
type Result struct {
	Title   string
	Content string
	Action  ResultAction
	Error   error
}

// Handler is the interface for command handlers
type Handler interface {
	Execute(ctx *Context) *Result
	Name() string
	Description() string
}

// Dispatcher routes commands to their handlers
type Dispatcher struct {
	handlers map[string]Handler
}

// NewDispatcher creates a new command dispatcher
func NewDispatcher() *Dispatcher {
	d := &Dispatcher{
		handlers: make(map[string]Handler),
	}

	// Register default handlers
	d.Register(&ExplainHandler{})
	d.Register(&HistoryHandler{})
	d.Register(&SettingsHandler{})
	d.Register(&HelpHandler{})

	return d
}

// Register adds a handler to the dispatcher
func (d *Dispatcher) Register(h Handler) {
	d.handlers[h.Name()] = h
}

// Dispatch executes a command by name
func (d *Dispatcher) Dispatch(cmdName string, ctx *Context) *Result {
	start := time.Now()
	attrs := []any{"command", cmdName}
	if ctx != nil {
		attrs = append(attrs, "cwd", ctx.CurrentDir, "exit_code", ctx.LastExitCode)
	}
	slog.Info("command_start", attrs...)

	handler, ok := d.handlers[cmdName]
	if !ok {
		slog.Warn("command_unknown", attrs...)
		return &Result{
			Title:   "Error",
			Content: "Unknown command: " + cmdName,
		}
	}

	result := handler.Execute(ctx)
	durationMs := time.Since(start).Milliseconds()
	doneAttrs := append(attrs, "duration_ms", durationMs)
	if result == nil {
		slog.Warn("command_result_nil", doneAttrs...)
		return result
	}

	doneAttrs = append(doneAttrs, "title", result.Title)
	if result.Error != nil {
		slog.Error("command_error", append(doneAttrs, "error", result.Error)...)
	} else {
		slog.Info("command_done", doneAttrs...)
	}

	return result
}

// GetHandler returns a handler by name
func (d *Dispatcher) GetHandler(cmdName string) (Handler, bool) {
	h, ok := d.handlers[cmdName]
	return h, ok
}
