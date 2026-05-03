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
	// DecisionAllowSession permits this and any future call to the same tool
	// for the rest of the wtf_cli session.
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
	cursor     int // 0=allow once, 1=allow session, 2=deny
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
// when the user picks an option. The popup eats arrow keys and 1/2/3/y/a/n
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
	case "2", "a":
		return p.decide(DecisionAllowSession)
	case "3", "n", "esc":
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

	panelWidth := p.width - 4
	if panelWidth > 70 {
		panelWidth = 70
	}
	if panelWidth < 40 {
		panelWidth = 40
	}

	contentWidth := panelWidth - 6
	if contentWidth < 10 {
		contentWidth = 10
	}

	question := "Allow tool call: " + p.request.Name + "?"
	questionLine := styles.DialogQuestionStyle.Width(contentWidth).Render(
		utils.TruncateToWidth(question, contentWidth),
	)

	var argsBlock string
	if p.prettyArgs != "" {
		var argLines []string
		for _, line := range strings.Split(p.prettyArgs, "\n") {
			argLines = append(argLines, utils.TruncateToWidth(line, contentWidth))
		}
		argsBlock = styles.TextMutedStyle.Width(contentWidth).Render(strings.Join(argLines, "\n"))
	}

	labels := []string{"Allow once", "Allow always", "Deny"}
	buttons := make([]string, len(labels))
	for i, label := range labels {
		style := styles.DialogButtonStyle
		if i == p.cursor {
			style = styles.DialogActiveButtonStyle
		}
		if i < len(labels)-1 {
			style = style.MarginRight(2)
		}
		buttons[i] = style.Render(label)
	}
	buttonRow := lipgloss.JoinHorizontal(lipgloss.Top, buttons...)
	buttonBlock := lipgloss.PlaceHorizontal(contentWidth, lipgloss.Center, buttonRow)

	parts := []string{questionLine}
	if argsBlock != "" {
		parts = append(parts, argsBlock)
	}
	parts = append(parts, buttonBlock)
	body := lipgloss.JoinVertical(lipgloss.Center, parts...)

	return styles.BoxStyle.Width(panelWidth).Render(body)
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
