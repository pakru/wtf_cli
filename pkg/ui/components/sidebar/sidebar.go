package sidebar

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"wtf_cli/pkg/ai"
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
	sidebarPaddingV   = 1
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
	// Existing fields
	title   string
	content string
	visible bool
	width   int
	height  int
	scrollY int
	lines   []string
	follow  bool

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
	}
}

// Show displays the sidebar with a title and content.
func (s *Sidebar) Show(title, content string) {
	s.title = title
	s.visible = true
	s.scrollY = 0
	s.follow = true

	// Preserve message history when reopening.
	if len(s.messages) > 0 {
		s.RefreshView() // Re-render from existing messages
	} else {
		s.SetContent(content)
	}
}

// Hide hides the sidebar.
func (s *Sidebar) Hide() {
	s.visible = false
}

// IsVisible returns whether the sidebar is visible.
func (s *Sidebar) IsVisible() bool {
	return s.visible
}

// GetTitle returns the current sidebar title.
func (s *Sidebar) GetTitle() string {
	return s.title
}

// SetSize sets the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.reflow()
	s.updateActiveCommand()
}

// SetContent updates the sidebar content.
func (s *Sidebar) SetContent(content string) {
	s.content = content
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
	titleLine := truncateToWidth(s.title, contentWidth)
	viewportHeight := s.viewportHeight()

	lines := make([]string, 0, contentHeight)
	commandLineSet := make(map[int]struct{}, len(s.cmdRenderedLines))
	for _, idx := range s.cmdRenderedLines {
		if idx >= 0 {
			commandLineSet[idx] = struct{}{}
		}
	}

	activeCommandLine := -1
	if s.commandSelectionEnabled() && s.cmdSelectedIdx >= 0 && s.cmdSelectedIdx < len(s.cmdRenderedLines) {
		activeCommandLine = s.cmdRenderedLines[s.cmdSelectedIdx]
	}

	// Title
	if contentHeight >= 1 {
		lines = append(lines, padStyled(sidebarTitleStyle.Render(titleLine), contentWidth))
	}

	// Viewport (messages)
	if viewportHeight > 0 {
		start := s.scrollY
		end := start + viewportHeight
		if end > len(s.lines) {
			end = len(s.lines)
		}
		for i := start; i < end; i++ {
			line := s.lines[i]
			if _, ok := commandLineSet[i]; ok {
				plain := stripANSICodes(line)
				if activeCommandLine == i {
					line = sidebarCommandActiveStyle.Render(plain)
				} else {
					line = sidebarCommandStyle.Render(plain)
				}
			}
			lines = append(lines, padStyled(line, contentWidth))
		}
		// Pad remaining viewport space
		for len(lines) < 1+viewportHeight {
			lines = append(lines, strings.Repeat(" ", contentWidth))
		}
	}

	// Separator
	lines = append(lines, strings.Repeat("─", contentWidth))

	// Textarea
	s.textarea.SetWidth(contentWidth)
	textareaView := s.textarea.View()
	textareaLines := strings.Split(textareaView, "\n")
	for i, line := range textareaLines {
		if i >= sidebarTextareaH {
			break
		}
		lines = append(lines, padStyled(line, contentWidth))
	}
	for i := len(textareaLines); i < sidebarTextareaH; i++ {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}

	// Shortcut hint goes at the very bottom, under the input.
	if s.shouldShowCommandFooter() {
		footerText := truncateToWidth(s.commandFooterText(), contentWidth)
		footer := sidebarFooterStyle.
			Width(contentWidth).
			Align(lipgloss.Center).
			Render(footerText)
		lines = append(lines, footer)
	}

	if len(lines) > contentHeight {
		lines = lines[:contentHeight]
	}
	for len(lines) < contentHeight {
		lines = append(lines, strings.Repeat(" ", contentWidth))
	}

	content := strings.Join(lines, "\n")

	boxWidth := s.width
	if boxWidth < 1 {
		boxWidth = 1
	}

	box := sidebarBoxStyle.
		Width(boxWidth).
		Padding(sidebarPaddingV, sidebarPaddingH).
		Render(content)

	return box
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
		Content: errorPrefix() + errMsg,
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
			sb.WriteString(messagePrefix("user"))
		} else {
			sb.WriteString(messagePrefix("assistant"))
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

// HandleMouse handles mouse wheel scrolling.
// TODO: Implement mouse scrolling once correct Bubble Tea mouse API is confirmed
func (s *Sidebar) HandleMouse(msg tea.MouseMsg) tea.Cmd {
	// Mouse event support can be tested and implemented in follow-up
	// The Bubble Tea MouseMsg API varies by version
	return nil
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
	height := s.height - 2*(sidebarBorderSize+sidebarPaddingV)
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
	lines := 1 + 1 + sidebarTextareaH // title + separator + textarea
	if s.shouldShowCommandFooter() {
		lines++
	}
	return lines
}

func (s *Sidebar) viewportHeight() int {
	viewportHeight := s.contentHeight() - s.chromeLines()
	if viewportHeight < 1 {
		return 1
	}
	return viewportHeight
}

func (s *Sidebar) shouldShowCommandFooter() bool {
	return len(s.cmdList) > 0
}

func (s *Sidebar) commandFooterText() string {
	if s.canApplySelectedCommand() {
		return "Enter Apply | Up/Down Navigate | Shift+Tab TTY | Ctrl+T Hide"
	}
	return "Enter Send | Up/Down Scroll | Shift+Tab TTY | Ctrl+T Hide"
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

	target := lineIdx - s.viewportHeight()/2
	if target < 0 {
		target = 0
	}
	maxScroll := s.maxScroll()
	if target > maxScroll {
		target = maxScroll
	}

	s.scrollY = target
	s.follow = s.scrollY >= maxScroll
}

type markdownToken struct {
	text string
	bold bool
}

func renderMarkdown(content string, width int) []string {
	lines, _ := renderMarkdownWithCommandLines(content, width, nil)
	return lines
}

func renderMarkdownWithCommandLines(content string, width int, commandRawLines []int) ([]string, []int) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	normalized = strings.ReplaceAll(normalized, "\r", "\n")
	normalized = sanitizeContent(normalized)
	normalized = strings.ReplaceAll(normalized, "<br>", "\n")
	normalized = strings.ReplaceAll(normalized, "<br/>", "\n")
	normalized = strings.ReplaceAll(normalized, "<br />", "\n")
	rawLines := strings.Split(normalized, "\n")

	commandRawLineSet := make(map[int]struct{}, len(commandRawLines))
	for _, rawLine := range commandRawLines {
		if rawLine >= 0 {
			commandRawLineSet[rawLine] = struct{}{}
		}
	}
	rawLineToRendered := make(map[int]int, len(commandRawLines))
	markCommandLine := func(rawLine, start, count int) {
		if _, ok := commandRawLineSet[rawLine]; !ok {
			return
		}
		if _, exists := rawLineToRendered[rawLine]; exists {
			return
		}
		if count <= 0 {
			rawLineToRendered[rawLine] = -1
			return
		}
		rawLineToRendered[rawLine] = start
	}

	var rendered []string
	inCode := false

	for i := 0; i < len(rawLines); i++ {
		line := rawLines[i]
		line = strings.ReplaceAll(line, "\t", "    ")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}

		if inCode {
			start := len(rendered)
			chunk := renderCodeLine(line, width)
			rendered = append(rendered, chunk...)
			markCommandLine(i, start, len(chunk))
			continue
		}

		if isTableRow(line) {
			blockStart := i
			block := []string{}
			for i < len(rawLines) && isTableRow(rawLines[i]) {
				block = append(block, rawLines[i])
				i++
			}
			i--

			rows := make([][]string, 0, len(block))
			for _, rowLine := range block {
				cells := splitTableRow(rowLine)
				if len(cells) == 0 {
					continue
				}
				rows = append(rows, cells)
			}
			if len(rows) > 0 {
				header := false
				if len(rows) > 1 && isSeparatorRow(rows[1]) {
					header = true
					rows = append(rows[:1], rows[2:]...)
				}
				start := len(rendered)
				chunk := renderTable(rows, header, width)
				rendered = append(rendered, chunk...)
				for rawLine := blockStart; rawLine <= i; rawLine++ {
					markCommandLine(rawLine, start, len(chunk))
				}
				continue
			}
		}

		start := len(rendered)
		chunk := renderMarkdownLine(line, width)
		rendered = append(rendered, chunk...)
		markCommandLine(i, start, len(chunk))
	}

	if len(rendered) == 0 {
		rendered = []string{""}
	}

	cmdRenderedLines := make([]int, 0, len(commandRawLines))
	for _, rawLine := range commandRawLines {
		if idx, ok := rawLineToRendered[rawLine]; ok {
			cmdRenderedLines = append(cmdRenderedLines, idx)
			continue
		}
		cmdRenderedLines = append(cmdRenderedLines, -1)
	}
	return rendered, cmdRenderedLines
}

func renderMarkdownLine(line string, width int) []string {
	if strings.TrimSpace(line) == "" {
		return []string{""}
	}

	tokens := tokenizeBoldWords(line)
	if len(tokens) == 0 {
		return []string{""}
	}

	return wrapTokens(tokens, width)
}

func renderTable(rows [][]string, header bool, width int) []string {
	if width <= 0 || len(rows) == 0 {
		return []string{""}
	}

	cols := 0
	for _, row := range rows {
		if len(row) > cols {
			cols = len(row)
		}
	}
	if cols == 0 {
		return []string{""}
	}

	for i := range rows {
		if len(rows[i]) < cols {
			padded := make([]string, cols)
			copy(padded, rows[i])
			rows[i] = padded
		}
	}

	colWidths := make([]int, cols)
	for _, row := range rows {
		for i, cell := range row {
			if w := runewidth.StringWidth(cell); w > colWidths[i] {
				colWidths[i] = w
			}
		}
	}

	fixedWidth := 3*cols + 1
	maxContent := width - fixedWidth
	if maxContent < cols {
		return renderTableFallback(rows, width)
	}

	colWidths = fitColumnWidths(colWidths, maxContent)

	var rendered []string
	for rowIndex, row := range rows {
		line := buildTableLine(row, colWidths)
		if runewidth.StringWidth(line) > width {
			line = trimToWidth(line, width)
		}
		if header && rowIndex == 0 {
			rendered = append(rendered, sidebarBoldStyle.Render(line))
			separator := buildTableSeparator(colWidths)
			rendered = append(rendered, sidebarTextStyle.Render(separator))
			continue
		}
		rendered = append(rendered, sidebarTextStyle.Render(line))
	}

	return rendered
}

func renderTableFallback(rows [][]string, width int) []string {
	var rendered []string
	for _, row := range rows {
		line := strings.Join(row, " | ")
		if width > 0 {
			line = trimToWidth(line, width)
		}
		rendered = append(rendered, sidebarTextStyle.Render(line))
	}
	return rendered
}

func buildTableLine(row []string, widths []int) string {
	var sb strings.Builder
	sb.WriteString("|")
	for i, cell := range row {
		if i >= len(widths) {
			break
		}
		text := trimToWidth(cell, widths[i])
		text = padPlain(text, widths[i])
		sb.WriteString(" ")
		sb.WriteString(text)
		sb.WriteString(" |")
	}
	return sb.String()
}

func buildTableSeparator(widths []int) string {
	var sb strings.Builder
	sb.WriteString("|")
	for _, w := range widths {
		if w < 1 {
			w = 1
		}
		sb.WriteString(" ")
		sb.WriteString(strings.Repeat("-", w))
		sb.WriteString(" |")
	}
	return sb.String()
}

func fitColumnWidths(widths []int, maxContent int) []int {
	if maxContent <= 0 {
		out := make([]int, len(widths))
		for i := range out {
			out[i] = 1
		}
		return out
	}

	out := make([]int, len(widths))
	copy(out, widths)

	total := 0
	for _, w := range out {
		if w < 1 {
			w = 1
		}
		total += w
	}
	if total <= maxContent {
		return out
	}

	for total > maxContent {
		maxIdx := -1
		maxVal := 0
		for i, w := range out {
			if w > maxVal {
				maxVal = w
				maxIdx = i
			}
		}
		if maxIdx == -1 || maxVal <= 1 {
			break
		}
		out[maxIdx]--
		total--
	}

	for i, w := range out {
		if w < 1 {
			out[i] = 1
		}
	}
	return out
}

func renderCodeLine(line string, width int) []string {
	if width <= 0 {
		return []string{line}
	}

	if line == "" {
		return []string{sidebarCodeStyle.Render(padPlain("", width))}
	}

	parts := splitByWidth(line, width)
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		padded := padPlain(part, width)
		lines = append(lines, sidebarCodeStyle.Render(padded))
	}
	return lines
}

func isTableRow(line string) bool {
	if strings.Count(line, "|") < 2 {
		return false
	}
	cells := splitTableRow(line)
	if len(cells) < 2 {
		return false
	}
	for _, cell := range cells {
		if strings.TrimSpace(cell) != "" {
			return true
		}
	}
	return false
}

func splitTableRow(line string) []string {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return nil
	}
	trimmed = strings.TrimPrefix(trimmed, "|")
	trimmed = strings.TrimSuffix(trimmed, "|")
	parts := strings.Split(trimmed, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, cell := range cells {
		clean := strings.TrimSpace(cell)
		if clean == "" {
			return false
		}
		clean = strings.Trim(clean, ":")
		if len(clean) < 3 {
			return false
		}
		for _, r := range clean {
			if r != '-' {
				return false
			}
		}
	}
	return true
}

func tokenizeBoldWords(line string) []markdownToken {
	var tokens []markdownToken
	bold := false

	for len(line) > 0 {
		idx := strings.Index(line, "**")
		segment := line
		if idx >= 0 {
			segment = line[:idx]
		}
		if segment != "" {
			words := strings.Fields(segment)
			for _, word := range words {
				tokens = append(tokens, markdownToken{text: word, bold: bold})
			}
		}
		if idx < 0 {
			break
		}
		bold = !bold
		line = line[idx+2:]
	}

	return tokens
}

func wrapTokens(tokens []markdownToken, width int) []string {
	if width <= 0 {
		return []string{""}
	}

	var lines []string
	var lineTokens []markdownToken
	lineWidth := 0

	flush := func() {
		if len(lineTokens) == 0 {
			lines = append(lines, "")
			return
		}
		lines = append(lines, renderTokenLine(lineTokens))
		lineTokens = nil
		lineWidth = 0
	}

	for _, token := range tokens {
		if token.text == "" {
			continue
		}

		parts := splitByWidth(token.text, width)
		for _, part := range parts {
			partWidth := runewidth.StringWidth(part)
			if lineWidth > 0 && lineWidth+1+partWidth > width {
				flush()
			}

			if lineWidth > 0 {
				lineWidth++
			}
			lineTokens = append(lineTokens, markdownToken{text: part, bold: token.bold})
			lineWidth += partWidth
		}
	}

	if len(lineTokens) > 0 {
		flush()
	}

	return lines
}

func renderTokenLine(tokens []markdownToken) string {
	var sb strings.Builder
	for i, token := range tokens {
		if i > 0 {
			sb.WriteString(sidebarTextStyle.Render(" "))
		}
		if token.bold {
			sb.WriteString(sidebarBoldStyle.Render(token.text))
		} else {
			sb.WriteString(sidebarTextStyle.Render(token.text))
		}
	}
	return sb.String()
}

func splitByWidth(text string, width int) []string {
	if width <= 0 {
		return []string{text}
	}
	if text == "" {
		return []string{""}
	}

	var parts []string
	var sb strings.Builder
	currentWidth := 0

	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width && currentWidth > 0 {
			parts = append(parts, sb.String())
			sb.Reset()
			currentWidth = 0
		}
		sb.WriteRune(r)
		currentWidth += runeWidth
	}

	if sb.Len() > 0 {
		parts = append(parts, sb.String())
	}

	if len(parts) == 0 {
		return []string{""}
	}
	return parts
}

func truncateToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	if runewidth.StringWidth(text) <= width {
		return text
	}
	if width <= 3 {
		return trimToWidth(text, width)
	}
	return trimToWidth(text, width-3) + "..."
}

func trimToWidth(text string, width int) string {
	if width <= 0 {
		return ""
	}
	var sb strings.Builder
	currentWidth := 0
	for _, r := range text {
		runeWidth := runewidth.RuneWidth(r)
		if currentWidth+runeWidth > width {
			break
		}
		sb.WriteRune(r)
		currentWidth += runeWidth
	}
	return sb.String()
}

func padPlain(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := runewidth.StringWidth(text)
	if textWidth >= width {
		return text
	}
	return text + strings.Repeat(" ", width-textWidth)
}

func padStyled(text string, width int) string {
	if width <= 0 {
		return text
	}
	textWidth := lipgloss.Width(text)
	if textWidth >= width {
		return text
	}
	return text + strings.Repeat(" ", width-textWidth)
}

func sanitizeContent(content string) string {
	if content == "" {
		return content
	}
	var sb strings.Builder
	sb.Grow(len(content))
	for _, r := range content {
		switch r {
		case '\n', '\t':
			sb.WriteRune(r)
			continue
		}
		if r < 0x20 || r == 0x7f {
			continue
		}
		sb.WriteRune(r)
	}
	return sb.String()
}

func stripANSICodes(s string) string {
	if s == "" {
		return s
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == 0x1b { // ESC
			if i+1 >= len(s) {
				continue
			}
			next := s[i+1]
			switch next {
			case '[': // CSI
				i += 2
				for i < len(s) {
					ch = s[i]
					if ch >= 0x40 && ch <= 0x7E {
						break
					}
					i++
				}
			case ']': // OSC
				i += 2
				for i < len(s) {
					if s[i] == 0x07 {
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i++
						break
					}
					i++
				}
			default:
				i++
			}
			continue
		}

		sb.WriteByte(ch)
	}

	return sb.String()
}

func messagePrefix(role string) string {
	useEmoji := runtime.GOOS != "darwin"
	switch role {
	case "user":
		if useEmoji {
			return "👤 **You:** "
		}
		return "**You:** "
	default:
		if useEmoji {
			return "🖥️ **Assistant:** "
		}
		return "**Assistant:** "
	}
}

func errorPrefix() string {
	if runtime.GOOS == "darwin" {
		return "Error: "
	}
	return "❌ Error: "
}

var (
	sidebarBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorBorder)

	sidebarTitleStyle         = styles.TitleStyle
	sidebarTextStyle          = styles.TextStyle
	sidebarBoldStyle          = styles.TextBoldStyle
	sidebarCodeStyle          = styles.CodeStyle
	sidebarFooterStyle        = styles.FooterStyle
	sidebarCommandStyle       = styles.CommandStyle
	sidebarCommandActiveStyle = styles.CommandActiveStyle
)
