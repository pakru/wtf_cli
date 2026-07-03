// Package continueprompt renders the modal popup that asks the user whether the
// agent loop should keep going after it has made a streak of tool calls
// (reaching the per-batch iteration limit). Two options: continue or stop.
//
// Like the toolapproval popup, the component is presentation-only: it receives
// a ContinuationRequest from the agent loop, displays it, and emits a
// DecisionMsg when the user picks an option. The Model dispatches the reply
// back to the loop.
package continueprompt

import (
	"fmt"
	"strings"

	"wtf_cli/pkg/commands"
	"wtf_cli/pkg/ui/components/utils"
	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// DecisionMsg is emitted when the user selects an option. The Model receives
// it, dispatches the corresponding reply on the agent loop's Reply channel,
// and hides the panel.
type DecisionMsg struct {
	// Request is the original ContinuationRequest the popup was shown for.
	Request *commands.ContinuationRequest
	// Continue is true when the user chose to keep going.
	Continue bool
}

// Panel is the continue-prompt component. Use NewPanel + Show to display, then
// drive its lifecycle through Update / View like other overlay components.
type Panel struct {
	visible bool
	width   int
	height  int
	request *commands.ContinuationRequest
	cursor  int // 0=continue, 1=stop
}

// NewPanel returns an empty, invisible panel.
func NewPanel() *Panel {
	return &Panel{}
}

// Show makes the panel visible for the given request and resets the cursor to
// "continue".
func (p *Panel) Show(req *commands.ContinuationRequest) {
	p.visible = true
	p.request = req
	p.cursor = 0
}

// Hide makes the panel invisible and forgets the request.
func (p *Panel) Hide() {
	p.visible = false
	p.request = nil
}

// IsVisible reports whether the panel should be rendered.
func (p *Panel) IsVisible() bool { return p.visible }

// Request returns the currently displayed request, or nil when hidden.
func (p *Panel) Request() *commands.ContinuationRequest { return p.request }

// SetSize records the terminal dimensions for centered rendering.
func (p *Panel) SetSize(width, height int) {
	p.width = width
	p.height = height
}

// Update handles a key press and returns a tea.Cmd that emits a DecisionMsg
// when the user picks an option. Esc/n/q stop (safe default); y/enter on the
// "continue" button continue.
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
		if p.cursor < 1 {
			p.cursor++
		}
		return nil
	case "tab":
		p.cursor = (p.cursor + 1) % 2
		return nil
	case "shift+tab":
		p.cursor = (p.cursor + 1) % 2
		return nil
	case "1", "y":
		return p.decide(true)
	case "2", "n", "q", "esc":
		return p.decide(false)
	case "enter":
		return p.decide(p.cursor == 0)
	}
	return nil
}

func (p *Panel) decide(cont bool) tea.Cmd {
	req := p.request
	return func() tea.Msg {
		return DecisionMsg{Request: req, Continue: cont}
	}
}

// View renders the modal. Caller composes this on top of the rest of the UI.
func (p *Panel) View() string {
	if !p.visible || p.request == nil {
		return ""
	}

	panelWidth := promptPanelWidth(p.width)
	boxStyle := styles.BoxStyleCompact
	contentWidth := panelWidth - boxStyle.GetHorizontalFrameSize()
	if contentWidth < 10 {
		contentWidth = 10
	}

	header := renderHeader(contentWidth)
	body := renderBody(p.request, contentWidth)
	buttons := p.renderButtons(contentWidth)
	help := renderHelp(contentWidth)

	parts := []string{header, "", body, "", buttons, "", help}
	content := lipgloss.JoinVertical(lipgloss.Left, parts...)
	return boxStyle.Width(panelWidth).Render(content)
}

func promptPanelWidth(screenWidth int) int {
	const (
		defaultWidth = 56
		minWidth     = 30
		maxWidth     = 72
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

func renderHeader(width int) string {
	title := "Continue?"
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

func renderBody(req *commands.ContinuationRequest, width int) string {
	text := fmt.Sprintf(
		"The assistant has made %d tool calls and wants to keep going. Continue?",
		req.ToolCalls,
	)
	return styles.DialogMetaValueStyle.Width(width).Render(text)
}

func (p *Panel) renderButtons(width int) string {
	labels := []string{"1. Continue", "2. Stop"}
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

func renderHelp(width int) string {
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
		styles.DialogHelpTextStyle.Render("stop"),
	}
	help := lipgloss.JoinHorizontal(lipgloss.Top, parts...)
	return styles.DialogHelpStyle.Width(width).Render(utils.TruncateToWidth(help, width))
}
