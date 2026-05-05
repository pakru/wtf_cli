package sidebar

import (
	"fmt"
	"os"
	"strings"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ui/components/selection"
	"wtf_cli/pkg/ui/styles"

	"charm.land/bubbles/v2/textarea"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
	"github.com/mattn/go-runewidth"
)

const (
	sidebarBorderSize = 1
	sidebarPaddingH   = 1
	sidebarPaddingV   = 0
	sidebarTextareaH  = 2
)

// FocusTarget indicates which part of the chat sidebar has focus.
type FocusTarget int

const (
	FocusViewport FocusTarget = iota
	FocusInput
)

// Sidebar displays AI responses alongside the terminal output.
type Sidebar struct {
	content string
	visible bool
	width   int
	height  int
	scrollY int
	lines   []string
	follow  bool
	sel     selection.Selection

	// Chat fields
	textarea         textarea.Model   // Chat input
	focused          FocusTarget      // Input or Viewport
	messages         []ai.ChatMessage // Persistent conversation history
	streaming        bool             // True while assistant response streaming
	cmdSelectedIdx   int              // Active command index (-1 = none)
	cmdList          []CommandEntry   // Commands extracted from assistant messages
	cmdRawLines      []int            // Raw line indices of command entries in stripped content
	cmdRenderedLines []int            // Rendered line indices corresponding to cmdList entries
	cmdDirty         bool             // True when command extraction needs refresh
	activeProvider   string           // Currently selected LLM provider
	activeModel      string           // Currently selected LLM model
}

// NewSidebar creates a new sidebar component.
func NewSidebar() *Sidebar {
	ta := textarea.New()
	ta.Placeholder = "Type your message..."
	ta.ShowLineNumbers = false
	ta.SetHeight(sidebarTextareaH)
	ta.Focus()

	return &Sidebar{
		textarea:       ta,
		focused:        FocusInput,
		cmdSelectedIdx: -1,
		cmdDirty:       true,
		activeProvider: "unknown",
		activeModel:    "unknown",
	}
}

const defaultTitle = "WTF Analysis"

// Show makes the sidebar visible, re-rendering from message history if present.
func (s *Sidebar) Show() {
	s.visible = true
	s.scrollY = 0
	s.follow = true

	if len(s.messages) > 0 {
		s.RefreshView()
	}
}

// Hide hides the sidebar.
func (s *Sidebar) Hide() {
	s.visible = false
	s.sel.Clear()
}

// IsVisible returns whether the sidebar is visible.
func (s *Sidebar) IsVisible() bool {
	return s.visible
}

// SetSize sets the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	if s.width != width || s.height != height {
		s.sel.Clear()
	}
	s.width = width
	s.height = height
	s.reflow()
	s.updateActiveCommand()
}

// SetContent updates the sidebar content.
func (s *Sidebar) SetContent(content string) {
	s.content = content
	s.sel.Clear()
	if len(s.messages) == 0 {
		s.cmdList = nil
		s.cmdRawLines = nil
		s.cmdRenderedLines = nil
		s.cmdSelectedIdx = -1
		s.cmdDirty = false
	}
	s.reflow()
	if s.follow {
		s.scrollY = s.maxScroll()
	}
	s.updateActiveCommand()
}

// ShouldHandleKey returns true when the sidebar should intercept the key.
func (s *Sidebar) ShouldHandleKey(msg tea.KeyPressMsg) bool {
	if !s.visible {
		return false
	}

	// Handle more keys when input is focused.
	if s.focused == FocusInput {
		// Always handle navigation and action keys
		switch msg.String() {
		case "esc", "enter", "up", "down", "pgup", "pgdown", "home", "end":
			return true
		}

		// Handle printable keys when input is focused
		if msg.Key().Text != "" {
			return true
		}

		// Handle editing keys
		switch msg.String() {
		case "backspace", "delete", "ctrl+a", "ctrl+e", "ctrl+k", "ctrl+u":
			return true
		}

		return false
	}

	keyStr := msg.String()
	switch keyStr {
	case "esc", "enter", "up", "down", "pgup", "pgdown", "q", "y":
		return true
	}

	return false
}

// Update handles keyboard input for the sidebar.
func (s *Sidebar) Update(msg tea.KeyPressMsg) tea.Cmd {
	if !s.visible {
		return nil
	}

	// Handle input focus.
	if s.focused == FocusInput {
		switch msg.String() {
		case "enter":
			if !s.streaming {
				content, ok := s.SubmitMessage()
				if ok && content != "" {
					// Return ChatSubmitMsg to be handled by model.go
					return func() tea.Msg {
						return ChatSubmitMsg{Content: content}
					}
				}
				// When input is empty, Enter applies the selected command.
				if s.canApplySelectedCommand() {
					return s.commandExecuteCmd()
				}
			}
			return nil
		case "esc":
			// Esc closes the sidebar
			s.Hide()
			return nil
		case "up", "down", "pgup", "pgdown":
			return s.handleScroll(msg.String())
		default:
			// Route to textarea
			var cmd tea.Cmd
			s.textarea, cmd = s.textarea.Update(msg)
			return cmd
		}
	}

	// Viewport-focused navigation.
	keyStr := msg.String()

	switch keyStr {
	case "esc", "q":
		s.Hide()
		return nil

	case "enter":
		if s.canApplySelectedCommand() {
			return s.commandExecuteCmd()
		}
		return nil

	case "up", "down", "pgup", "pgdown":
		return s.handleScroll(keyStr)

	case "y":
		return s.copyToClipboard()
	}

	return nil
}

// handleScroll processes scroll key events and returns nil command.
func (s *Sidebar) handleScroll(key string) tea.Cmd {
	if s.commandSelectionEnabled() {
		switch key {
		case "up":
			s.stepCommandSelection(-1)
			return nil
		case "down":
			s.stepCommandSelection(1)
			return nil
		}
	}

	maxScroll := s.maxScroll()

	switch key {
	case "up":
		if s.scrollY > 0 {
			s.scrollY--
			s.follow = false
		}
	case "down":
		if s.scrollY < maxScroll {
			s.scrollY++
		}
		s.follow = s.scrollY >= maxScroll
	case "pgup":
		s.scrollY -= 10
		if s.scrollY < 0 {
			s.scrollY = 0
		}
		s.follow = false
	case "pgdown":
		s.scrollY += 10
		if s.scrollY > maxScroll {
			s.scrollY = maxScroll
		}
		s.follow = s.scrollY >= maxScroll
	}

	s.updateActiveCommand()

	return nil
}

// ChatSubmitMsg is returned when the user submits a chat message.
type ChatSubmitMsg struct {
	Content string
}

// CommandExecuteMsg is emitted when a selected command should be applied to PTY input.
type CommandExecuteMsg struct {
	Command string
}

// View renders the sidebar.
func (s *Sidebar) View() string {
	if !s.visible {
		return ""
	}

	contentWidth := s.contentWidth()
	contentHeight := s.contentHeight()

	return s.renderChatView(contentWidth, contentHeight)
}

func (s *Sidebar) renderChatView(contentWidth, contentHeight int) string {
	viewportHeight := s.viewportHeight()

	lines := make([]string, 0, contentHeight)
	lines = append(lines, s.renderTitle(contentWidth))
	lines = append(lines, "")
	lines = append(lines, s.renderViewport(contentWidth, viewportHeight)...)
	lines = append(lines, strings.Repeat("─", contentWidth))
	lines = append(lines, s.renderTextarea(contentWidth)...)
	lines = append(lines, s.renderFooter(contentWidth))

	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	for len(lines) < contentHeight {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}

	return sidebarBoxStyle.
		Width(max(s.width, 1)).
		Padding(sidebarPaddingV, sidebarPaddingH, 0).
		Render(strings.Join(lines, "\n"))
}

func (s *Sidebar) renderTitle(contentWidth int) string {
	title := truncateToWidth(defaultTitle, contentWidth)
	titleRendered := styles.DialogTitleStyle.Render(title)
	fillWidth := contentWidth - lipgloss.Width(title) - 1
	if fillWidth <= 0 {
		return titleRendered
	}
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		titleRendered,
		" ",
		styles.DialogTitleFillStyle.Render(strings.Repeat("=", fillWidth)),
	)
}

func (s *Sidebar) renderViewport(contentWidth, viewportHeight int) []string {
	commandLines := make(map[int]struct{}, len(s.cmdRenderedLines))
	for _, idx := range s.cmdRenderedLines {
		if idx >= 0 {
			commandLines[idx] = struct{}{}
		}
	}

	activeCommandLine := -1
	if s.commandSelectionEnabled() && s.cmdSelectedIdx >= 0 && s.cmdSelectedIdx < len(s.cmdRenderedLines) {
		activeCommandLine = s.cmdRenderedLines[s.cmdSelectedIdx]
	}

	lines := make([]string, 0, viewportHeight)
	for i := s.scrollY; i < min(s.scrollY+viewportHeight, len(s.lines)); i++ {
		line := s.lines[i]
		if _, ok := commandLines[i]; ok {
			plain := stripANSICodes(line)
			if activeCommandLine == i {
				line = styles.CommandActiveStyle.Render(plain)
			} else {
				line = styles.CommandStyle.Render(plain)
			}
		}
		if left, right, ok := selection.LineBounds(s.sel, i, lipgloss.Width(line)); ok {
			line = selection.ApplyLineHighlight(line, left, right)
		}
		lines = append(lines, padStyled(line, contentWidth))
	}
	for len(lines) < viewportHeight {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}
	return lines
}

func (s *Sidebar) renderTextarea(contentWidth int) []string {
	s.textarea.SetWidth(contentWidth)
	textareaLines := strings.Split(s.textarea.View(), "\n")
	lines := make([]string, sidebarTextareaH)
	for i := range sidebarTextareaH {
		if i < len(textareaLines) {
			lines[i] = padStyled(textareaLines[i], contentWidth)
		} else {
			lines[i] = strings.Repeat(" ", contentWidth)
		}
	}
	return lines
}

func (s *Sidebar) renderFooter(contentWidth int) string {
	return styles.FooterStyle.
		Width(contentWidth).
		Align(lipgloss.Left).
		Render(truncateToWidth(s.commandFooterText(contentWidth), contentWidth))
}

func (s *Sidebar) copyToClipboard() tea.Cmd {
	text := StripCommandMarkers(s.content)
	return func() tea.Msg {
		_, _ = fmt.Fprint(os.Stdout, osc52.New(text))
		return nil
	}
}

func (s *Sidebar) commandExecuteCmd() tea.Cmd {
	if !s.canApplySelectedCommand() {
		return nil
	}
	if s.cmdSelectedIdx < 0 || s.cmdSelectedIdx >= len(s.cmdList) {
		return nil
	}
	command := s.cmdList[s.cmdSelectedIdx].Command
	return func() tea.Msg {
		return CommandExecuteMsg{Command: command}
	}
}

// ToggleFocus switches focus between viewport and input.
func (s *Sidebar) ToggleFocus() {
	if s.focused == FocusInput {
		s.focused = FocusViewport
		s.textarea.Blur()
	} else {
		s.focused = FocusInput
		s.textarea.Focus()
	}
}

// FocusInput switches focus to the text input.
func (s *Sidebar) FocusInput() {
	s.focused = FocusInput
	s.textarea.Focus()
}

// BlurInput switches focus to the message viewport.
func (s *Sidebar) BlurInput() {
	s.focused = FocusViewport
	s.textarea.Blur()
}

// IsFocusedOnInput returns true if the text input is focused.
func (s *Sidebar) IsFocusedOnInput() bool {
	return s.focused == FocusInput
}

// IsStreaming returns true if an assistant response is currently streaming.
func (s *Sidebar) IsStreaming() bool {
	return s.streaming
}

// SetStreaming sets the streaming state.
func (s *Sidebar) SetStreaming(active bool) {
	s.streaming = active
}

// SetActiveLLM updates the active provider/model shown in the footer.
func (s *Sidebar) SetActiveLLM(provider, model string) {
	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	if provider == "" {
		provider = "unknown"
	}
	if model == "" {
		model = "unknown"
	}
	s.activeProvider = provider
	s.activeModel = model
}

// ActiveLLMLabel returns the formatted footer label for the active provider/model.
func (s *Sidebar) ActiveLLMLabel() string {
	return "LLM: " + s.activeProvider + "-" + s.activeModel
}

// AppendUserMessage adds a user message to the chat history.
func (s *Sidebar) AppendUserMessage(content string) {
	s.messages = append(s.messages, ai.ChatMessage{
		Role:    "user",
		Content: content,
	})
	s.cmdDirty = true
}

// StartAssistantMessage creates a new empty assistant message.
func (s *Sidebar) StartAssistantMessage() {
	s.messages = append(s.messages, ai.ChatMessage{
		Role:    "assistant",
		Content: "",
	})
	s.cmdDirty = true
}

// StartAssistantMessageWithContent creates a new assistant message with content.
func (s *Sidebar) StartAssistantMessageWithContent(content string) {
	s.messages = append(s.messages, ai.ChatMessage{
		Role:    "assistant",
		Content: content,
	})
	s.cmdDirty = true
}

// AppendErrorMessage adds an error message to the chat.
func (s *Sidebar) AppendErrorMessage(errMsg string) {
	s.messages = append(s.messages, ai.ChatMessage{
		Role:    "assistant",
		Content: MessagePrefix("error") + errMsg,
	})
	s.cmdDirty = true
}

// UpdateLastMessage appends delta to the last assistant message.
func (s *Sidebar) UpdateLastMessage(delta string) {
	if len(s.messages) > 0 {
		s.messages[len(s.messages)-1].Content += delta
		s.cmdDirty = true
	}
}

// SetLastMessageContent replaces the content of the last message.
func (s *Sidebar) SetLastMessageContent(content string) {
	if len(s.messages) > 0 {
		s.messages[len(s.messages)-1].Content = content
		s.cmdDirty = true
	}
}

// RemoveLastMessage removes the most recent message.
func (s *Sidebar) RemoveLastMessage() {
	if len(s.messages) > 0 {
		s.messages = s.messages[:len(s.messages)-1]
		s.cmdDirty = true
	}
}

// GetMessages returns the chat message history.
func (s *Sidebar) GetMessages() []ai.ChatMessage {
	return s.messages
}

// SubmitMessage returns the input content and clears the textarea.
func (s *Sidebar) SubmitMessage() (string, bool) {
	content := strings.TrimSpace(s.textarea.Value())
	if content == "" {
		return "", false
	}
	s.textarea.Reset()
	return content, true
}

// RefreshView re-renders the viewport from messages.
func (s *Sidebar) RefreshView() {
	s.content = s.RenderMessages()
	s.sel.Clear()
	s.reflow()
	if s.follow {
		s.scrollY = s.maxScroll()
		s.selectLastCommand()
		return
	}
	s.updateActiveCommand()
}

// RenderMessages renders all messages as markdown.
func (s *Sidebar) RenderMessages() string {
	var sb strings.Builder
	for i, msg := range s.messages {
		if i > 0 {
			sb.WriteString("\n\n")
		}
		if msg.Role == "user" {
			// Add separator line before user messages for readability
			if i > 0 {
				sb.WriteString("───────────────────────\n\n")
			}
			sb.WriteString(MessagePrefix("user"))
		} else {
			sb.WriteString(MessagePrefix("assistant"))
		}
		sb.WriteString(msg.Content)
	}
	return sb.String()
}

// HandlePaste routes paste content to the textarea.
func (s *Sidebar) HandlePaste(content string) {
	if s.focused == FocusInput {
		s.textarea.InsertString(content)
	}
}

// HandleWheel handles mouse wheel scrolling.
func (s *Sidebar) HandleWheel(msg tea.MouseWheelMsg) tea.Cmd {
	switch msg.Mouse().Button {
	case tea.MouseWheelUp:
		return s.handleScroll("up")
	case tea.MouseWheelDown:
		return s.handleScroll("down")
	}
	return nil
}

// SelectionPoint translates a screen coordinate into a message line coordinate.
func (s *Sidebar) SelectionPoint(screenX, screenY, originX int) (row, col int, ok bool) {
	if !s.visible || s.width <= 0 || s.height <= 0 {
		return 0, 0, false
	}

	contentLeft := originX + sidebarBorderSize + sidebarPaddingH
	contentTop := sidebarBorderSize + sidebarPaddingV
	contentCol := screenX - contentLeft
	contentRow := screenY - contentTop
	if contentCol < 0 || contentCol >= s.contentWidth() {
		return 0, 0, false
	}

	viewportRow := contentRow - 2 // row 0 is the title, row 1 is the empty separator line
	if viewportRow < 0 || viewportRow >= s.viewportHeight() {
		return 0, 0, false
	}

	lineRow := s.scrollY + viewportRow
	if lineRow < 0 || lineRow >= len(s.lines) {
		return 0, 0, false
	}
	return lineRow, contentCol, true
}

// StartSelection begins a sidebar text selection in message-line coordinates.
func (s *Sidebar) StartSelection(row, col int) {
	s.sel.Start(row, col)
}

// UpdateSelection moves the active sidebar selection endpoint.
func (s *Sidebar) UpdateSelection(row, col int) {
	if !s.sel.Active {
		return
	}
	s.sel.Update(row, col)
}

// FinishSelection returns the selected sidebar text and clears the highlight.
func (s *Sidebar) FinishSelection() string {
	if !s.sel.Active && s.sel.IsEmpty() {
		return ""
	}
	s.sel.Finish()
	text := selection.ExtractText(s.lines, s.sel)
	s.sel.Clear()
	return text
}

// ClearSelection removes the current sidebar selection.
func (s *Sidebar) ClearSelection() {
	s.sel.Clear()
}

// HasActiveSelection reports whether a sidebar drag selection is in progress.
func (s *Sidebar) HasActiveSelection() bool {
	return s.sel.Active
}

// RefreshCommands rebuilds extracted command metadata when command state is dirty.
func (s *Sidebar) RefreshCommands() {
	if !s.cmdDirty {
		return
	}

	s.cmdList = s.cmdList[:0]
	s.cmdRawLines = s.cmdRawLines[:0]

	if len(s.messages) == 0 {
		s.cmdDirty = false
		s.cmdSelectedIdx = -1
		return
	}

	currentLine := 0
	for i, msg := range s.messages {
		if i > 0 {
			currentLine += 2 // blank line spacing between messages
		}
		if msg.Role == "user" && i > 0 {
			currentLine += 2 // separator + blank line before user messages
		}

		if msg.Role == "assistant" {
			entries := ExtractCommands(msg.Content)
			for _, entry := range entries {
				lineOffset := 0
				if entry.SourceIndex > 0 && entry.SourceIndex <= len(msg.Content) {
					lineOffset = strings.Count(msg.Content[:entry.SourceIndex], "\n")
				}
				s.cmdList = append(s.cmdList, entry)
				s.cmdRawLines = append(s.cmdRawLines, currentLine+lineOffset)
			}
		}

		currentLine += strings.Count(msg.Content, "\n")
	}

	s.cmdDirty = false
}

func (s *Sidebar) reflow() {
	width := s.contentWidth()
	if width <= 0 {
		s.lines = nil
		s.scrollY = 0
		s.cmdRenderedLines = nil
		s.cmdSelectedIdx = -1
		return
	}

	s.RefreshCommands()
	content := StripCommandMarkers(s.content)
	s.lines, s.cmdRenderedLines = renderMarkdownWithCommandLines(content, width, s.cmdRawLines)

	if s.scrollY > s.maxScroll() {
		s.scrollY = s.maxScroll()
	}
	if s.scrollY < 0 {
		s.scrollY = 0
	}
	s.updateActiveCommand()
}

func (s *Sidebar) contentWidth() int {
	width := s.width - 2*(sidebarBorderSize+sidebarPaddingH)
	if width < 1 {
		return 1
	}
	return width
}

func (s *Sidebar) contentHeight() int {
	// Top padding + top border + bottom border (no bottom padding).
	height := s.height - 2*sidebarBorderSize - sidebarPaddingV
	if height < 1 {
		return 1
	}
	return height
}

func (s *Sidebar) maxScroll() int {
	viewportHeight := s.viewportHeight()

	max := len(s.lines) - viewportHeight
	if max < 0 {
		return 0
	}
	return max
}

func (s *Sidebar) chromeLines() int {
	return 1 + 1 + 1 + sidebarTextareaH + 1 // title + empty line + separator + textarea + footer
}

func (s *Sidebar) viewportHeight() int {
	viewportHeight := s.contentHeight() - s.chromeLines()
	if viewportHeight < 1 {
		return 1
	}
	return viewportHeight
}

func (s *Sidebar) commandFooterText(contentWidth int) string {
	label := s.ActiveLLMLabel()
	if s.canApplySelectedCommand() {
		hint := "Enter Apply | Up/Down Navigate | Shift+Tab TTY | Ctrl+T Hide"
		full := label + " | " + hint
		if runewidth.StringWidth(full) <= contentWidth {
			return full
		}
		return label + " | Apply"
	}
	return label
}

func (s *Sidebar) commandSelectionEnabled() bool {
	return !s.streaming && s.textarea.Value() == "" && len(s.cmdList) > 0
}

func (s *Sidebar) canApplySelectedCommand() bool {
	if !s.commandSelectionEnabled() {
		return false
	}
	if s.cmdSelectedIdx < 0 || s.cmdSelectedIdx >= len(s.cmdList) {
		return false
	}
	if s.cmdSelectedIdx >= len(s.cmdRenderedLines) {
		return false
	}
	return s.cmdRenderedLines[s.cmdSelectedIdx] >= 0
}

func (s *Sidebar) updateActiveCommand() {
	if len(s.cmdRenderedLines) == 0 || len(s.cmdList) == 0 {
		s.cmdSelectedIdx = -1
		return
	}

	center := s.scrollY + s.viewportHeight()/2
	bestIdx := -1
	bestDistance := 1 << 30

	for i, lineIdx := range s.cmdRenderedLines {
		if i >= len(s.cmdList) || lineIdx < 0 {
			continue
		}
		distance := lineIdx - center
		if distance < 0 {
			distance = -distance
		}
		if distance < bestDistance {
			bestDistance = distance
			bestIdx = i
		}
	}

	s.cmdSelectedIdx = bestIdx
}

func (s *Sidebar) stepCommandSelection(delta int) {
	if delta == 0 || len(s.cmdList) == 0 {
		return
	}

	isSelectable := func(i int) bool {
		return i >= 0 && i < len(s.cmdRenderedLines) && s.cmdRenderedLines[i] >= 0
	}

	hasSelectable := false
	for i := range s.cmdList {
		if isSelectable(i) {
			hasSelectable = true
			break
		}
	}
	if !hasSelectable {
		return
	}

	idx := s.cmdSelectedIdx
	if idx < 0 || !isSelectable(idx) {
		if delta > 0 {
			for i := 0; i < len(s.cmdList); i++ {
				if isSelectable(i) {
					idx = i
					break
				}
			}
		} else {
			for i := len(s.cmdList) - 1; i >= 0; i-- {
				if isSelectable(i) {
					idx = i
					break
				}
			}
		}
	}

	next := idx + delta
	for next >= 0 && next < len(s.cmdList) {
		if isSelectable(next) {
			break
		}
		next += delta
	}
	if next < 0 || next >= len(s.cmdList) {
		return
	}
	if next == s.cmdSelectedIdx {
		return
	}

	s.cmdSelectedIdx = next
	s.revealSelectedCommand()
}

func (s *Sidebar) selectLastCommand() {
	for i := len(s.cmdRenderedLines) - 1; i >= 0; i-- {
		if s.cmdRenderedLines[i] < 0 {
			continue
		}
		s.cmdSelectedIdx = i
		s.revealSelectedCommand()
		return
	}
	s.cmdSelectedIdx = -1
}

func (s *Sidebar) revealSelectedCommand() {
	if s.cmdSelectedIdx < 0 || s.cmdSelectedIdx >= len(s.cmdRenderedLines) {
		return
	}
	lineIdx := s.cmdRenderedLines[s.cmdSelectedIdx]
	if lineIdx < 0 {
		return
	}

	target := max(lineIdx-s.viewportHeight()/2, 0)
	maxScroll := s.maxScroll()
	if target > maxScroll {
		target = maxScroll
	}

	s.scrollY = target
	s.follow = s.scrollY >= maxScroll
}

var sidebarBoxStyle = lipgloss.NewStyle().
	Border(lipgloss.RoundedBorder()).
	BorderForeground(styles.ColorBorder)
