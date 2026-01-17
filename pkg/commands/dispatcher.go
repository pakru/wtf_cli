package commands

// Result represents the result of a command execution
type Result struct {
	Title   string
	Content string
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
	d.Register(&WtfHandler{})
	d.Register(&ExplainHandler{})
	d.Register(&FixHandler{})
	d.Register(&HistoryHandler{})
	d.Register(&ModelsHandler{})
	d.Register(&SettingsHandler{})
	d.Register(&CloseSidebarHandler{})
	d.Register(&HelpHandler{})

	return d
}

// Register adds a handler to the dispatcher
func (d *Dispatcher) Register(h Handler) {
	d.handlers[h.Name()] = h
}

// Dispatch executes a command by name
func (d *Dispatcher) Dispatch(cmdName string, ctx *Context) *Result {
	handler, ok := d.handlers[cmdName]
	if !ok {
		return &Result{
			Title:   "Error",
			Content: "Unknown command: " + cmdName,
		}
	}

	return handler.Execute(ctx)
}

// GetHandler returns a handler by name
func (d *Dispatcher) GetHandler(cmdName string) (Handler, bool) {
	h, ok := d.handlers[cmdName]
	return h, ok
}
