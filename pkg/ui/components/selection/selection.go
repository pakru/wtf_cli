package selection

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

const (
	inverseOn  = "\x1b[7m"
	inverseOff = "\x1b[27m"
)

// Selection tracks a mouse text selection range within panel-local content
// coordinates.
type Selection struct {
	Active    bool
	AnchorRow int
	AnchorCol int
	EndRow    int
	EndCol    int
}

// Start begins a selection at row/col.
func (s *Selection) Start(row, col int) {
	row, col = clampPoint(row, col)
	s.Active = true
	s.AnchorRow = row
	s.AnchorCol = col
	s.EndRow = row
	s.EndCol = col
}

// Update moves the selection endpoint.
func (s *Selection) Update(row, col int) {
	row, col = clampPoint(row, col)
	s.EndRow = row
	s.EndCol = col
}

// Finish marks the drag as complete while preserving the selected range.
func (s *Selection) Finish() {
	s.Active = false
}

// Clear resets all selection state.
func (s *Selection) Clear() {
	*s = Selection{}
}

// IsEmpty reports whether the selection range is a single point.
func (s Selection) IsEmpty() bool {
	return s.AnchorRow == s.EndRow && s.AnchorCol == s.EndCol
}

// Normalize returns the selection range in top-to-bottom order.
func (s Selection) Normalize() (startRow, startCol, endRow, endCol int) {
	startRow, startCol = s.AnchorRow, s.AnchorCol
	endRow, endCol = s.EndRow, s.EndCol

	if startRow > endRow || (startRow == endRow && startCol > endCol) {
		startRow, startCol, endRow, endCol = endRow, endCol, startRow, startCol
	}

	return startRow, startCol, endRow, endCol
}

// Contains reports whether row/col is inside the normalized selection range.
func (s Selection) Contains(row, col int) bool {
	startRow, startCol, endRow, endCol := s.Normalize()
	if row < startRow || row > endRow {
		return false
	}
	if startRow == endRow {
		return col >= startCol && col < endCol
	}
	if row == startRow {
		return col >= startCol
	}
	if row == endRow {
		return col < endCol
	}
	return true
}

// ExtractText returns ANSI-stripped text from lines within the selection range.
func ExtractText(lines []string, sel Selection) string {
	if sel.IsEmpty() || len(lines) == 0 {
		return ""
	}

	startRow, startCol, endRow, endCol := sel.Normalize()
	if endRow < 0 || startRow >= len(lines) {
		return ""
	}
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= len(lines) {
		endRow = len(lines) - 1
	}

	selected := make([]string, 0, endRow-startRow+1)
	for row := startRow; row <= endRow; row++ {
		line := lines[row]
		lineWidth := ansi.StringWidth(line)
		left, right := lineSelectionBounds(row, startRow, startCol, endRow, endCol, lineWidth)
		if right < left {
			right = left
		}
		segment := ansi.Cut(line, left, right)
		selected = append(selected, ansi.Strip(segment))
	}

	return strings.Join(selected, "\n")
}

// ApplyHighlight overlays reverse-video highlighting on selected cells.
func ApplyHighlight(content string, sel Selection) string {
	if sel.IsEmpty() || content == "" {
		return content
	}

	lines := strings.Split(content, "\n")
	startRow, startCol, endRow, endCol := sel.Normalize()
	if endRow < 0 || startRow >= len(lines) {
		return content
	}
	if startRow < 0 {
		startRow = 0
	}
	if endRow >= len(lines) {
		endRow = len(lines) - 1
	}

	for row := startRow; row <= endRow; row++ {
		lineWidth := ansi.StringWidth(lines[row])
		left, right := lineSelectionBounds(row, startRow, startCol, endRow, endCol, lineWidth)
		if right <= left {
			continue
		}
		lines[row] = ApplyLineHighlight(lines[row], left, right)
	}

	return strings.Join(lines, "\n")
}

// LineBounds returns the selected column range for row.
func LineBounds(sel Selection, row, lineWidth int) (left, right int, ok bool) {
	if sel.IsEmpty() {
		return 0, 0, false
	}
	startRow, startCol, endRow, endCol := sel.Normalize()
	if row < startRow || row > endRow {
		return 0, 0, false
	}
	left, right = lineSelectionBounds(row, startRow, startCol, endRow, endCol, lineWidth)
	return left, right, right > left
}

// ApplyLineHighlight overlays reverse-video highlighting on a single line.
func ApplyLineHighlight(line string, startCol, endCol int) string {
	if line == "" || endCol <= startCol {
		return line
	}
	if startCol < 0 {
		startCol = 0
	}

	var b strings.Builder
	b.Grow(len(line) + len(inverseOn) + len(inverseOff))

	state := byte(0)
	col := 0
	highlighting := false

	for i := 0; i < len(line); {
		seq, width, n, newState := ansi.DecodeSequence(line[i:], state, nil)
		if n <= 0 {
			break
		}
		state = newState

		if width == 0 {
			b.WriteString(seq)
			if highlighting && isSGRReset(seq) {
				b.WriteString(inverseOn)
			}
			i += n
			continue
		}

		if highlighting && col >= endCol {
			b.WriteString(inverseOff)
			highlighting = false
		}

		overlaps := col < endCol && col+width > startCol
		if overlaps && !highlighting {
			b.WriteString(inverseOn)
			highlighting = true
		}
		if !overlaps && highlighting {
			b.WriteString(inverseOff)
			highlighting = false
		}

		b.WriteString(seq)
		col += width
		i += n

		if highlighting && col >= endCol {
			b.WriteString(inverseOff)
			highlighting = false
		}
	}

	if highlighting {
		b.WriteString(inverseOff)
	}

	return b.String()
}

func lineSelectionBounds(row, startRow, startCol, endRow, endCol, lineWidth int) (left, right int) {
	left = 0
	right = lineWidth

	if startRow == endRow {
		left = startCol
		right = endCol
	} else {
		if row == startRow {
			left = startCol
		}
		if row == endRow {
			right = endCol
		}
	}

	if left < 0 {
		left = 0
	}
	if right < 0 {
		right = 0
	}
	if left > lineWidth {
		left = lineWidth
	}
	if right > lineWidth {
		right = lineWidth
	}
	return left, right
}

func isSGRReset(seq string) bool {
	if !strings.HasPrefix(seq, "\x1b[") || !strings.HasSuffix(seq, "m") {
		return false
	}
	params := strings.TrimSuffix(strings.TrimPrefix(seq, "\x1b["), "m")
	if params == "" {
		return true
	}
	for _, part := range strings.Split(params, ";") {
		if part == "" {
			return true
		}
		value, err := strconv.Atoi(part)
		if err != nil {
			continue
		}
		if value == 0 || value == 27 {
			return true
		}
	}
	return false
}

func clampPoint(row, col int) (int, int) {
	if row < 0 {
		row = 0
	}
	if col < 0 {
		col = 0
	}
	return row, col
}
