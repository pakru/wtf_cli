package ui

import (
	"fmt"
	"os"
	"strings"

	osc52 "github.com/aymanbagabas/go-osc52/v2"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/mattn/go-runewidth"
)

const (
	sidebarBorderSize  = 1
	sidebarPaddingH    = 1
	sidebarPaddingV    = 1
	sidebarFooterLabel = "Up/Down Scroll | y Copy | Esc/q Close"
)

// Sidebar displays AI responses alongside the terminal output.
type Sidebar struct {
	title   string
	content string
	visible bool
	width   int
	height  int
	scrollY int
	lines   []string
	follow  bool
}

// NewSidebar creates a new sidebar component.
func NewSidebar() *Sidebar {
	return &Sidebar{}
}

// Show displays the sidebar with a title and content.
func (s *Sidebar) Show(title, content string) {
	s.title = title
	s.visible = true
	s.scrollY = 0
	s.follow = true
	s.SetContent(content)
}

// Hide hides the sidebar.
func (s *Sidebar) Hide() {
	s.visible = false
}

// IsVisible returns whether the sidebar is visible.
func (s *Sidebar) IsVisible() bool {
	return s.visible
}

// SetSize sets the sidebar dimensions.
func (s *Sidebar) SetSize(width, height int) {
	s.width = width
	s.height = height
	s.reflow()
}

// SetContent updates the sidebar content.
func (s *Sidebar) SetContent(content string) {
	s.content = content
	s.reflow()
	if s.follow {
		s.scrollY = s.maxScroll()
	}
}

// ShouldHandleKey returns true when the sidebar should intercept the key.
func (s *Sidebar) ShouldHandleKey(msg tea.KeyMsg) bool {
	if !s.visible {
		return false
	}

	switch msg.Type {
	case tea.KeyEsc, tea.KeyUp, tea.KeyDown, tea.KeyPgUp, tea.KeyPgDown:
		return true
	}

	switch msg.String() {
	case "q", "y":
		return true
	}

	return false
}

// Update handles keyboard input for the sidebar.
func (s *Sidebar) Update(msg tea.KeyMsg) tea.Cmd {
	if !s.visible {
		return nil
	}

	maxScroll := s.maxScroll()

	switch msg.Type {
	case tea.KeyEsc:
		s.Hide()
		return nil

	case tea.KeyUp:
		if s.scrollY > 0 {
			s.scrollY--
			s.follow = false
		}
		return nil

	case tea.KeyDown:
		if s.scrollY < maxScroll {
			s.scrollY++
		}
		s.follow = s.scrollY >= maxScroll
		return nil

	case tea.KeyPgUp:
		s.scrollY -= 10
		if s.scrollY < 0 {
			s.scrollY = 0
		}
		s.follow = false
		return nil

	case tea.KeyPgDown:
		s.scrollY += 10
		if s.scrollY > maxScroll {
			s.scrollY = maxScroll
		}
		s.follow = s.scrollY >= maxScroll
		return nil
	}

	switch msg.String() {
	case "q":
		s.Hide()
		return nil
	case "y":
		return s.copyToClipboard()
	}

	return nil
}

// View renders the sidebar.
func (s *Sidebar) View() string {
	if !s.visible {
		return ""
	}

	contentWidth := s.contentWidth()
	contentHeight := s.contentHeight()

	titleLine := truncateToWidth(s.title, contentWidth)
	footerLine := truncateToWidth(sidebarFooterLabel, contentWidth)

	lines := make([]string, 0, contentHeight)

	if contentHeight >= 1 {
		lines = append(lines, padStyled(sidebarTitleStyle.Render(titleLine), contentWidth))
	}

	bodyHeight := s.bodyHeight()
	if bodyHeight > 0 {
		start := s.scrollY
		end := start + bodyHeight
		if end > len(s.lines) {
			end = len(s.lines)
		}
		for i := start; i < end; i++ {
			lines = append(lines, padStyled(s.lines[i], contentWidth))
		}
		for len(lines) < 1+bodyHeight {
			lines = append(lines, strings.Repeat(" ", contentWidth))
		}
	}

	if contentHeight >= 2 {
		lines = append(lines, padStyled(sidebarFooterStyle.Render(footerLine), contentWidth))
	}

	content := strings.Join(lines, "\n")

	box := sidebarBoxStyle.
		Width(s.width).
		Padding(sidebarPaddingV, sidebarPaddingH).
		Render(content)

	return box
}

func (s *Sidebar) copyToClipboard() tea.Cmd {
	text := s.content
	return func() tea.Msg {
		_, _ = fmt.Fprint(os.Stdout, osc52.New(text))
		return nil
	}
}

func (s *Sidebar) reflow() {
	width := s.contentWidth()
	if width <= 0 {
		s.lines = nil
		s.scrollY = 0
		return
	}
	s.lines = renderMarkdown(s.content, width)
	if s.scrollY > s.maxScroll() {
		s.scrollY = s.maxScroll()
	}
	if s.scrollY < 0 {
		s.scrollY = 0
	}
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

func (s *Sidebar) bodyHeight() int {
	contentHeight := s.contentHeight()
	if contentHeight < 2 {
		return 0
	}
	return contentHeight - 2
}

func (s *Sidebar) maxScroll() int {
	body := s.bodyHeight()
	if body <= 0 {
		return 0
	}
	max := len(s.lines) - body
	if max < 0 {
		return 0
	}
	return max
}

type markdownToken struct {
	text string
	bold bool
}

func renderMarkdown(content string, width int) []string {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	rawLines := strings.Split(normalized, "\n")

	var rendered []string
	inCode := false

	for _, line := range rawLines {
		line = strings.ReplaceAll(line, "\t", "    ")
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") {
			inCode = !inCode
			continue
		}

		if inCode {
			rendered = append(rendered, renderCodeLine(line, width)...)
			continue
		}

		rendered = append(rendered, renderMarkdownLine(line, width)...)
	}

	if len(rendered) == 0 {
		return []string{""}
	}
	return rendered
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

var (
	sidebarBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("141"))

	sidebarTitleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("141")).
				Bold(true)

	sidebarTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))

	sidebarBoldStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Bold(true)

	sidebarCodeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("213")).
				Background(lipgloss.Color("235"))

	sidebarFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("245")).
				Italic(true)
)
