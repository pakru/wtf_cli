package sidebar

import (
	"fmt"
	"os"
	"strings"

	"wtf_cli/pkg/ui/styles"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	osc52 "github.com/aymanbagabas/go-osc52/v2"
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
func (s *Sidebar) ShouldHandleKey(msg tea.KeyPressMsg) bool {
	if !s.visible {
		return false
	}

	keyStr := msg.String()
	switch keyStr {
	case "esc", "up", "down", "pgup", "pgdown", "q", "y":
		return true
	}

	return false
}

// Update handles keyboard input for the sidebar.
func (s *Sidebar) Update(msg tea.KeyPressMsg) tea.Cmd {
	if !s.visible {
		return nil
	}

	maxScroll := s.maxScroll()
	keyStr := msg.String()

	switch keyStr {
	case "esc":
		s.Hide()
		return nil

	case "up":
		if s.scrollY > 0 {
			s.scrollY--
			s.follow = false
		}
		return nil

	case "down":
		if s.scrollY < maxScroll {
			s.scrollY++
		}
		s.follow = s.scrollY >= maxScroll
		return nil

	case "pgup":
		s.scrollY -= 10
		if s.scrollY < 0 {
			s.scrollY = 0
		}
		s.follow = false
		return nil

	case "pgdown":
		s.scrollY += 10
		if s.scrollY > maxScroll {
			s.scrollY = maxScroll
		}
		s.follow = s.scrollY >= maxScroll
		return nil

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
	normalized = strings.ReplaceAll(normalized, "<br>", "\n")
	normalized = strings.ReplaceAll(normalized, "<br/>", "\n")
	normalized = strings.ReplaceAll(normalized, "<br />", "\n")
	rawLines := strings.Split(normalized, "\n")

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
			rendered = append(rendered, renderCodeLine(line, width)...)
			continue
		}

		if isTableRow(line) {
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
				rendered = append(rendered, renderTable(rows, header, width)...)
				continue
			}
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

var (
	sidebarBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(styles.ColorBorder)

	sidebarTitleStyle  = styles.TitleStyle
	sidebarTextStyle   = styles.TextStyle
	sidebarBoldStyle   = styles.TextBoldStyle
	sidebarCodeStyle   = styles.CodeStyle
	sidebarFooterStyle = styles.FooterStyle
)
