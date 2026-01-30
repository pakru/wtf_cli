package terminal

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

const lineRendererTabWidth = 4

type lineCell struct {
	text  string
	width int
}

type lineBuffer struct {
	cells []lineCell
}

func (l *lineBuffer) visibleLen() int {
	count := 0
	for _, c := range l.cells {
		count += c.width
	}
	return count
}

func (l *lineBuffer) indexForCol(col int) (int, bool) {
	visible := 0
	insertIdx := 0
	for i, c := range l.cells {
		if c.width == 0 {
			if visible == col {
				insertIdx = i + 1
			}
			continue
		}
		if visible == col {
			if insertIdx > 0 {
				return insertIdx, true
			}
			return i, true
		}
		visible += c.width
		insertIdx = 0
	}
	if visible == col {
		if insertIdx > 0 {
			return insertIdx, false
		}
		return len(l.cells), false
	}
	return len(l.cells), false
}

func (l *lineBuffer) insertCell(idx int, cell lineCell) {
	if idx >= len(l.cells) {
		l.cells = append(l.cells, cell)
		return
	}
	l.cells = append(l.cells, lineCell{})
	copy(l.cells[idx+1:], l.cells[idx:])
	l.cells[idx] = cell
}

func (l *lineBuffer) padToCol(col int) {
	for l.visibleLen() < col {
		idx, _ := l.indexForCol(l.visibleLen())
		l.insertCell(idx, lineCell{text: " ", width: 1})
	}
}

func (l *lineBuffer) setCellAt(col int, text string, width int) {
	if width < 1 {
		width = 1
	}
	l.padToCol(col)
	idx, hasVisible := l.indexForCol(col)
	if hasVisible && idx < len(l.cells) && l.cells[idx].width > 0 {
		l.cells[idx] = lineCell{text: text, width: width}
		return
	}
	l.insertCell(idx, lineCell{text: text, width: width})
}

func (l *lineBuffer) insertZeroWidthAt(col int, seq string) {
	idx, _ := l.indexForCol(col)
	l.insertCell(idx, lineCell{text: seq, width: 0})
}

func (l *lineBuffer) deleteCellsAtCol(col int, count int) {
	if count < 1 || len(l.cells) == 0 {
		return
	}
	if col < 0 {
		col = 0
	}
	visible := 0
	i := 0
	for i < len(l.cells) && count > 0 {
		c := l.cells[i]
		if c.width == 0 {
			i++
			continue
		}
		if visible+c.width <= col {
			visible += c.width
			i++
			continue
		}
		l.cells = append(l.cells[:i], l.cells[i+1:]...)
		count -= c.width
	}
}

func (l *lineBuffer) truncateFromCol(col int) {
	idx, _ := l.indexForCol(col)
	if idx < len(l.cells) {
		l.cells = l.cells[:idx]
	}
}

func (l *lineBuffer) String() string {
	if len(l.cells) == 0 {
		return ""
	}
	var b strings.Builder
	for _, c := range l.cells {
		b.WriteString(c.text)
	}
	return b.String()
}

// LineRenderer tracks a minimal terminal line buffer for normal shell output.
// It supports cursor movement and overwriting without deleting visible text.
type LineRenderer struct {
	lines     []lineBuffer
	row       int
	col       int
	inEscape  bool
	inCSI     bool
	inOSC     bool
	oscEsc    bool
	csiParam  int
	csiHas    bool
	csiParams []int
}

// NewLineRenderer creates a new line renderer.
func NewLineRenderer() *LineRenderer {
	return &LineRenderer{}
}

// Reset clears renderer state.
func (r *LineRenderer) Reset() {
	r.lines = nil
	r.row = 0
	r.col = 0
	r.inEscape = false
	r.inCSI = false
	r.inOSC = false
	r.oscEsc = false
	r.csiParam = 0
	r.csiHas = false
	r.csiParams = nil
}

// CursorPosition returns the current cursor row/col (0-indexed).
func (r *LineRenderer) CursorPosition() (row, col int) {
	return r.row, r.col
}

func (r *LineRenderer) ensureLine(row int) {
	for len(r.lines) <= row {
		r.lines = append(r.lines, lineBuffer{})
	}
}

func (r *LineRenderer) moveCursorLeft(n int) {
	if n < 1 {
		n = 1
	}
	r.col -= n
	if r.col < 0 {
		r.col = 0
	}
}

func (r *LineRenderer) moveCursorRight(n int) {
	if n < 1 {
		n = 1
	}
	r.col += n
}

func (r *LineRenderer) pushCSIParam() {
	if r.csiHas || len(r.csiParams) > 0 {
		r.csiParams = append(r.csiParams, r.csiParam)
	}
	r.csiParam = 0
	r.csiHas = false
}

// Append processes raw PTY data and updates the line buffer.
func (r *LineRenderer) Append(data []byte) {
	if len(data) == 0 {
		return
	}

	for i := 0; i < len(data); {
		b := data[i]
		if r.inOSC {
			if r.oscEsc {
				if b == '\\' {
					r.inOSC = false
				}
				r.oscEsc = false
				i++
				continue
			}
			if b == 0x07 {
				r.inOSC = false
				i++
				continue
			}
			if b == 0x1b {
				r.oscEsc = true
				i++
				continue
			}
			i++
			continue
		}

		if r.inEscape {
			if b == '[' {
				r.inCSI = true
				r.inEscape = false
				r.csiParam = 0
				r.csiHas = false
				r.csiParams = nil
				i++
				continue
			}
			if b == ']' {
				r.inEscape = false
				r.inOSC = true
				i++
				continue
			}
			r.inEscape = false
			i++
			continue
		}

		if r.inCSI {
			switch {
			case b >= '0' && b <= '9':
				r.csiParam = r.csiParam*10 + int(b-'0')
				r.csiHas = true
				i++
				continue
			case b == ';':
				r.pushCSIParam()
				i++
				continue
			case b >= 0x40 && b <= 0x7E:
				r.pushCSIParam()
				r.handleCSI(b)
				r.inCSI = false
				r.csiParams = nil
				i++
				continue
			default:
				i++
				continue
			}
		}

		if b == 0x1b {
			r.inEscape = true
			i++
			continue
		}

		switch b {
		case '\r':
			r.col = 0
			i++
		case '\n':
			r.row++
			r.col = 0
			r.ensureLine(r.row)
			i++
		case '\t':
			r.ensureLine(r.row)
			steps := lineRendererTabWidth - (r.col % lineRendererTabWidth)
			for i := 0; i < steps; i++ {
				r.lines[r.row].setCellAt(r.col, " ", 1)
				r.col++
			}
			i++
		case 0x08, 0x7f:
			r.moveCursorLeft(1)
			i++
		case 0x01: // Ctrl+A (home)
			r.col = 0
			i++
		case 0x05: // Ctrl+E (end)
			r.ensureLine(r.row)
			r.col = r.lines[r.row].visibleLen()
			i++
		default:
			if b >= 0x20 {
				r.ensureLine(r.row)
				rn, size := utf8.DecodeRune(data[i:])
				if rn == utf8.RuneError && size == 1 {
					r.lines[r.row].setCellAt(r.col, string(rune(b)), 1)
					r.col++
					i++
					continue
				}
				width := runewidth.RuneWidth(rn)
				if width < 1 {
					width = 1
				}
				r.lines[r.row].setCellAt(r.col, string(rn), width)
				r.col += width
				i += size
			} else {
				i++
			}
		}
	}
}

func (r *LineRenderer) handleCSI(final byte) {
	switch final {
	case 'm':
		r.ensureLine(r.row)
		seq := "\x1b["
		if len(r.csiParams) > 0 {
			params := make([]string, 0, len(r.csiParams))
			for _, p := range r.csiParams {
				params = append(params, itoa(p))
			}
			seq += strings.Join(params, ";")
		}
		seq += "m"
		r.lines[r.row].insertZeroWidthAt(r.col, seq)
	case 'D':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.moveCursorLeft(n)
	case 'C':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.moveCursorRight(n)
	case 'F':
		r.ensureLine(r.row)
		r.col = r.lines[r.row].visibleLen()
	case 'K':
		r.ensureLine(r.row)
		r.lines[r.row].truncateFromCol(r.col)
	case 'P':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.ensureLine(r.row)
		r.lines[r.row].deleteCellsAtCol(r.col, n)
	case 'X':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.ensureLine(r.row)
		for i := 0; i < n; i++ {
			r.lines[r.row].setCellAt(r.col+i, " ", 1)
		}
	case 'H', 'f':
		if len(r.csiParams) == 0 {
			r.col = 0
			break
		}
		row := 1
		col := 1
		if len(r.csiParams) >= 1 && r.csiParams[0] > 0 {
			row = r.csiParams[0]
		}
		if len(r.csiParams) >= 2 && r.csiParams[1] > 0 {
			col = r.csiParams[1]
		}
		r.row = row - 1
		if r.row < 0 {
			r.row = 0
		}
		r.col = col - 1
		if r.col < 0 {
			r.col = 0
		}
		r.ensureLine(r.row)
	}
}

// Content returns the current rendered content.
func (r *LineRenderer) Content() string {
	if len(r.lines) == 0 {
		return ""
	}
	parts := make([]string, len(r.lines))
	for i, line := range r.lines {
		parts[i] = line.String()
	}
	return strings.Join(parts, "\n")
}

func itoa(v int) string {
	if v == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
