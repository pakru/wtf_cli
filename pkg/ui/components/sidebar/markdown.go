package sidebar

import (
	"strings"

	"wtf_cli/pkg/ui/styles"

	"github.com/mattn/go-runewidth"
)

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
			rendered = append(rendered, styles.TextBoldStyle.Render(line))
			separator := buildTableSeparator(colWidths)
			rendered = append(rendered, styles.TextStyle.Render(separator))
			continue
		}
		rendered = append(rendered, styles.TextStyle.Render(line))
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
		rendered = append(rendered, styles.TextStyle.Render(line))
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
		return []string{styles.CodeStyle.Render(padPlain("", width))}
	}

	parts := splitByWidth(line, width)
	lines := make([]string, 0, len(parts))
	for _, part := range parts {
		padded := padPlain(part, width)
		lines = append(lines, styles.CodeStyle.Render(padded))
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
			sb.WriteString(styles.TextStyle.Render(" "))
		}
		if token.bold {
			sb.WriteString(styles.TextBoldStyle.Render(token.text))
		} else {
			sb.WriteString(styles.TextStyle.Render(token.text))
		}
	}
	return sb.String()
}
