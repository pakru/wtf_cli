// Package toolapproval renders the modal popup that asks the user whether a
// tool call should run. Three options: allow once, allow always this session,
// deny.
//
// The component is presentation-only: it receives a ToolApprovalRequest from
// the agent loop, displays it, and emits a DecisionMsg when the user picks an
// option. The Model owns state continuity (session policy, dispatching the
// reply back to the loop) — this file does not.
package toolapproval

import (
	"encoding/json"
	"fmt"
	"strings"

	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DecisionKind is the user's choice on a Panel popup.
type DecisionKind int

const (
	// DecisionAllowOnce permits this single tool call.
	DecisionAllowOnce DecisionKind = iota
	// DecisionAllowSession permits remembering this decision for the rest of
	// the wtf_cli session. Its scope is contextual, mirroring
	// commands.ApprovalDecision.Persistent: for an ordinary request
	// (Request.Escape == nil) it means "always allow this tool by name"; for
	// an escape request (Request.Escape != nil) it means "always allow this
	// tool to access this directory" — the Model interprets which one
	// applies and dispatches to the corresponding store.
	DecisionAllowSession
	// DecisionDeny refuses the tool call.
	DecisionDeny
)

// DecisionMsg is emitted when the user selects an option. The Model receives
// it, dispatches the corresponding reply on the agent loop's Reply channel,
// and hides the panel.
type DecisionMsg struct {
	// Request is the original ApprovalRequest the popup was shown for.
	Request *commands.ApprovalRequest
	Kind    DecisionKind
}

// Panel is the approval-popup component. Use NewPanel + Show to display, then
// drive its lifecycle through Update / View like other overlay components.
type Panel struct {
	visible    bool
	width      int
	height     int
	request    *commands.ApprovalRequest
	cursor     int // 0=allow, 1=allow session, 2=deny
	prettyArgs string
}

// NewPanel returns an empty, invisible panel.
func NewPanel() *Panel {
	return &Panel{}
}

// Show makes the panel visible for the given approval request and resets the
// cursor to "allow once". Calling Show on an already-visible panel replaces
// the request — the caller is responsible for handling the abandoned previous
// request (the agent loop emits at most one ApprovalRequest at a time, so
// this should not happen in practice).
func (p *Panel) Show(req *commands.ApprovalRequest) {
	p.visible = true
	p.request = req
	p.cursor = 0
	p.prettyArgs = formatArgs(req.Args)
}

// Hide makes the panel invisible and forgets the request.
func (p *Panel) Hide() {
	p.visible = false
	p.request = nil
}

// IsVisible reports whether the panel should be rendered.
func (p *Panel) IsVisible() bool { return p.visible }

// Request returns the currently displayed approval request, or nil when the
// panel is hidden. The Model uses this to dispatch the reply.
func (p *Panel) Request() *commands.ApprovalRequest { return p.request }

// SetSize records the terminal dimensions for centered rendering.
func (p *Panel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Update handles a key press and returns a tea.Cmd that emits a DecisionMsg
// when the user picks an option. The popup eats arrow keys and 1/2/3/y/a/s/d/n
// shortcuts. Esc denies (treated as "no" for safety).
func (p *Panel) Update(msg tea.KeyPressMsg) tea.Cmd {
	if !p.visible || p.request == nil {
		return nil
	}
	switch msg.String() {
	case "up", "k", "left", "h":
		if p.cursor > 0 {
			p.cursor--
		}
		return nil
	case "down", "j", "right", "l":
		if p.cursor < 2 {
			p.cursor++
		}
		return nil
	case "tab":
		p.cursor = (p.cursor + 1) % 3
		return nil
	case "shift+tab":
		p.cursor = (p.cursor + 2) % 3
		return nil
	case "1", "y":
		return p.decide(DecisionAllowOnce)
	case "2", "a", "s":
		return p.decide(DecisionAllowSession)
	case "3", "d", "n", "esc":
		return p.decide(DecisionDeny)
	case "enter":
		switch p.cursor {
		case 0:
			return p.decide(DecisionAllowOnce)
		case 1:
			return p.decide(DecisionAllowSession)
		default:
			return p.decide(DecisionDeny)
		}
	}
	return nil
}

func (p *Panel) decide(k DecisionKind) tea.Cmd {
	req := p.request
	return func() tea.Msg {
		return DecisionMsg{Request: req, Kind: k}
	}
}

// View renders the modal. Caller composes this on top of the rest of the UI.
func (p *Panel) View() string {
	if !p.visible || p.request == nil {
		return ""
	}

	panelWidth := approvalPanelWidth(p.width)
	boxStyle := styles.BoxStyleCompact
	contentWidth := panelWidth - boxStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}

	header := renderApprovalHeader(contentWidth)
	metadata := p.renderMetadata(contentWidth)
	content := p.renderContentPanel(contentWidth)
	buttons := p.renderButtons(contentWidth)
	help := renderApprovalHelp(contentWidth)

	parts := []string{header}
	if p.request.Escape != nil {
		parts = append(parts, "", renderEscapeWarning(contentWidth))
	}
	parts = append(parts, "", metadata)
	if content != "" {
		parts = append(parts, "", content)
	}
	if p.request.Escape != nil {
		parts = append(parts, "", renderEscapeScopeNote(p.request, contentWidth))
	}
	parts = append(parts, "", buttons, "", help)
	body := lipgloss.JoinVertical(lipgloss.Left, parts...)

	return boxStyle.Width(panelWidth).Render(body)
}

// escapeWarningStyle highlights the out-of-workdir banner shown on an escape
// request's popup.
var escapeWarningStyle = lipgloss.NewStyle().Foreground(styles.ColorWarning).Bold(true)

func renderEscapeWarning(width int) string {
	text := "⚠ Path is OUTSIDE your working directory"
	return escapeWarningStyle.Render(utils.TruncateToWidth(text, width))
}

// renderEscapeScopeNote clarifies what "Allow for session" grants on an
// escape popup: this tool, this directory — never the tool everywhere, and
// never other tools under the same directory.
//
// The directory is rendered on its own line so it gets the *entire* content
// width as its truncation budget, not whatever is left over after a fixed
// sentence. An earlier version embedded the directory mid-sentence and
// shared one budget between fixed text and the directory: at ordinary popup
// widths the fixed text alone consumed nearly all of it, leaving only a
// couple of cells for the directory — technically tail-preserving but
// practically useless. The label line (short, fixed, never model-controlled)
// can safely head-truncate in the rare case it doesn't fit.
func renderEscapeScopeNote(req *commands.ApprovalRequest, width int) string {
	label := fmt.Sprintf("%q scopes to %s, directory:", "Allow for session", req.Name)
	dir := utils.TailPreservingTruncate(utils.EscapeControl(req.Escape.GrantDir), width)
	return lipgloss.JoinVertical(
		lipgloss.Left,
		styles.DialogHelpTextStyle.Render(utils.TruncateToWidth(label, width)),
		styles.DialogMetaValueStyle.Render(dir),
	)
}

func approvalPanelWidth(screenWidth int) int {
	const (
		defaultWidth = 60
		minWidth     = 30
		maxWidth     = 80
		margin       = 4
	)

	if screenWidth <= 0 {
		return defaultWidth
	}
	width := screenWidth - margin
	if width > maxWidth {
		width = maxWidth
	}
	if width < minWidth {
		width = screenWidth
	}
	if width < 1 {
		width = 1
	}
	return width
}

func renderApprovalHeader(width int) string {
	title := "Permission Required"
	if lipgloss.Width(title) >= width {
		return styles.DialogTitleStyle.Render(utils.TruncateToWidth(title, width))
	}
	fillWidth := width - lipgloss.Width(title) - 1
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		styles.DialogTitleStyle.Render(title),
		" ",
		styles.DialogTitleFillStyle.Render(strings.Repeat("=", fillWidth)),
	)
}

func (p *Panel) renderMetadata(width int) string {
	if p.request.Escape != nil {
		return p.renderEscapeMetadata(width)
	}
	summary := summarizeArgs(p.request.Args)
	lines := []string{renderApprovalKV("Tool", p.request.Name, width)}
	if summary.path != "" {
		lines = append(lines, renderApprovalKVTail("Path", summary.path, width))
	}
	if summary.desc != "" {
		lines = append(lines, renderApprovalKV("Desc", summary.desc, width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderEscapeMetadata shows the requested path and, when symlink resolution
// changed it, the resolved path — e.g. a request for "../logs" that turns
// out to be a symlink to "/var/log" shows both, so the user approves what
// will actually be accessed, not just the innocuous-looking argument.
func (p *Panel) renderEscapeMetadata(width int) string {
	esc := p.request.Escape
	lines := []string{
		renderApprovalKV("Tool", p.request.Name, width),
		renderApprovalKVTail("Path", esc.RequestedPath, width),
	}
	if esc.ResolvedPath != esc.RequestedPath {
		lines = append(lines, renderApprovalKVTail("Resolves to", esc.ResolvedPath, width))
	}
	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderApprovalKV renders a "Key value" metadata row, head-truncating the
// value. Used for fields where the front of the value is what identifies it
// (tool names, free-text descriptions).
func renderApprovalKV(key, value string, width int) string {
	return renderApprovalKVStyled(key, value, width, utils.TruncateToWidth)
}

// renderApprovalKVTail is renderApprovalKV but tail-preserving: for a path,
// the distinguishing part is often the suffix ("…/secret"), so truncating
// from the front instead of the back must never hide it.
func renderApprovalKVTail(key, value string, width int) string {
	return renderApprovalKVStyled(key, value, width, utils.TailPreservingTruncate)
}

func renderApprovalKVStyled(key, value string, width int, truncate func(string, int) string) string {
	keyText := styles.DialogMetaKeyStyle.Render(key)
	valueWidth := width - lipgloss.Width(keyText) - 1
	if valueWidth < 0 {
		valueWidth = 0
	}
	escaped := utils.EscapeControl(value)
	valueText := styles.DialogMetaValueStyle.Render(truncate(escaped, valueWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, keyText, " ", valueText)
}

func (p *Panel) renderContentPanel(width int) string {
	summary := summarizeArgs(p.request.Args)
	content := summary.preview
	if content == "" {
		content = p.prettyArgs
	}
	if strings.TrimSpace(content) == "" {
		return ""
	}

	panelStyle := styles.DialogContentPanelStyle
	innerWidth := width - panelStyle.GetHorizontalFrameSize()
	if innerWidth < 1 {
		innerWidth = 1
	}
	lines := strings.Split(content, "\n")
	const maxPreviewLines = 8
	if len(lines) > maxPreviewLines {
		lines = append(lines[:maxPreviewLines], "...")
	}
	for i, line := range lines {
		lines[i] = utils.TruncateToWidth(utils.EscapeControl(line), innerWidth)
	}
	return panelStyle.Width(width).Render(strings.Join(lines, "\n"))
}

func (p *Panel) renderButtons(width int) string {
	sessionLabel := "2. Allow for Session"
	if p.request.Escape != nil {
		sessionLabel = "2. Allow dir for session"
	}
	labels := []string{"1. Allow", sessionLabel, "3. Deny"}
	buttons := make([]string, len(labels))
	for i, label := range labels {
		style := styles.DialogButtonStyle
		if i == p.cursor {
			style = styles.DialogActiveButtonStyle
		}
		button := style.Render(label)
		if i > 0 {
			button = "  " + button
		}
		buttons[i] = button
	}

	row := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	if lipgloss.Width(row) <= width {
		return lipgloss.PlaceHorizontal(width, lipgloss.Center, row)
	}

	for i, button := range buttons {
		buttons[i] = lipgloss.PlaceHorizontal(width, lipgloss.Right, strings.TrimLeft(button, " "))
	}
	return lipgloss.JoinVertical(lipgloss.Right, buttons...)
}

func renderApprovalHelp(width int) string {
	parts := []string{
		styles.DialogHelpKeyStyle.Render("←/→"),
		" ",
		styles.DialogHelpTextStyle.Render("choose"),
		" ",
		styles.DialogHelpSeparatorStyle.Render("•"),
		" ",
		styles.DialogHelpKeyStyle.Render("enter"),
		" ",
		styles.DialogHelpTextStyle.Render("confirm"),
		" ",
		styles.DialogHelpSeparatorStyle.Render("•"),
		" ",
		styles.DialogHelpKeyStyle.Render("esc"),
		" ",
		styles.DialogHelpTextStyle.Render("exit"),
	}
	help := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return styles.DialogHelpStyle.Width(width).Render(utils.TruncateToWidth(help, width))
}

type argsSummary struct {
	path    string
	desc    string
	preview string
}

func summarizeArgs(raw json.RawMessage) argsSummary {
	if len(raw) == 0 {
		return argsSummary{}
	}

	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return argsSummary{preview: string(raw)}
	}

	summary := argsSummary{
		path:    firstString(obj, "path", "file_path", "file", "directory", "url"),
		desc:    firstString(obj, "description", "desc", "reason"),
		preview: firstString(obj, "command", "cmd", "script"),
	}
	if summary.preview == "" {
		summary.preview = formatArgs(raw)
	}
	return summary
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := obj[key]
		if !ok {
			continue
		}
		switch typed := v.(type) {
		case string:
			if strings.TrimSpace(typed) != "" {
				return typed
			}
		case fmt.Stringer:
			if s := typed.String(); strings.TrimSpace(s) != "" {
				return s
			}
		}
	}
	return ""
}

// formatArgs renders raw JSON arguments as pretty multi-line JSON, falling
// back to the raw string if it's not valid JSON.
func formatArgs(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return string(raw)
	}
	pretty, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(raw)
	}
	return string(pretty)
}
