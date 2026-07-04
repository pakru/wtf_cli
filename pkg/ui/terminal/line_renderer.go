package terminal

import (
	"strings"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
)

const lineRendererTabWidth = 4

// cellStyle is the rendition (SGR state) attached to a visible cell. The
// zero value means "default style". Values are interned (see internStyle)
// and, once stored on a cell, must never be mutated in place — doing so
// would retroactively recolor every cell already sharing that pointer.
type cellStyle struct {
	fg    string // canonical SGR fragment: "31", "38;5;208", "38;2;r;g;b"; "" = default
	bg    string // "41", "48;5;n", "48;2;r;g;b"; "" = default
	attrs uint16
}

const (
	attrBold uint16 = 1 << iota
	attrDim
	attrItalic
	attrUnderline
	attrBlink
	attrReverse
	attrConceal
	attrStrike
)

// sgrCodes returns the SGR parameter codes representing this style, in a
// fixed, deterministic order. A nil receiver yields no codes.
func (s *cellStyle) sgrCodes() []string {
	if s == nil {
		return nil
	}
	var codes []string
	if s.attrs&attrBold != 0 {
		codes = append(codes, "1")
	}
	if s.attrs&attrDim != 0 {
		codes = append(codes, "2")
	}
	if s.attrs&attrItalic != 0 {
		codes = append(codes, "3")
	}
	if s.attrs&attrUnderline != 0 {
		codes = append(codes, "4")
	}
	if s.attrs&attrBlink != 0 {
		codes = append(codes, "5")
	}
	if s.attrs&attrReverse != 0 {
		codes = append(codes, "7")
	}
	if s.attrs&attrConceal != 0 {
		codes = append(codes, "8")
	}
	if s.attrs&attrStrike != 0 {
		codes = append(codes, "9")
	}
	if s.fg != "" {
		codes = append(codes, s.fg)
	}
	if s.bg != "" {
		codes = append(codes, s.bg)
	}
	return codes
}

func (s *cellStyle) sgrString() string {
	return "\x1b[" + strings.Join(s.sgrCodes(), ";") + "m"
}

type lineCell struct {
	text  string
	width int
	style *cellStyle // nil = default style
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

// indexForCol returns the cell index whose visible column is col, and
// whether a cell actually starts there. If col falls beyond the end of the
// line (or inside a wide cell), it returns the insertion point and false.
func (l *lineBuffer) indexForCol(col int) (int, bool) {
	visible := 0
	for i, c := range l.cells {
		if visible == col {
			return i, true
		}
		visible += c.width
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

func (l *lineBuffer) setCellAt(col int, text string, width int, style *cellStyle) {
	if width < 1 {
		width = 1
	}
	l.padToCol(col)
	idx, hasVisible := l.indexForCol(col)
	if hasVisible && idx < len(l.cells) && l.cells[idx].width > 0 {
		l.cells[idx] = lineCell{text: text, width: width, style: style}
		return
	}
	l.insertCell(idx, lineCell{text: text, width: width, style: style})
}

func (l *lineBuffer) insertSpacesAtCol(col int, count int) int {
	if count < 1 {
		return 0
	}
	if col < 0 {
		col = 0
	}
	l.padToCol(col)
	idx, _ := l.indexForCol(col)
	for i := 0; i < count; i++ {
		l.insertCell(idx+i, lineCell{text: " ", width: 1})
	}
	return idx
}

func (l *lineBuffer) deleteCellsAtIndex(idx int, count int) {
	if count < 1 || idx < 0 || idx >= len(l.cells) {
		return
	}
	if idx+count > len(l.cells) {
		count = len(l.cells) - idx
	}
	l.cells = append(l.cells[:idx], l.cells[idx+count:]...)
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

// String renders the line, emitting SGR transitions only where the style
// actually changes between adjacent cells. Every line is self-balanced: if
// it ends styled, a trailing reset is appended, so styling never bleeds
// into a following line.
func (l *lineBuffer) String() string {
	if len(l.cells) == 0 {
		return ""
	}
	var b strings.Builder
	var current *cellStyle
	for _, c := range l.cells {
		if c.style != current {
			emitStyleTransition(&b, current, c.style)
			current = c.style
		}
		b.WriteString(c.text)
	}
	if current != nil {
		b.WriteString("\x1b[0m")
	}
	return b.String()
}

// emitStyleTransition writes the escape sequence needed to move the pen
// from "from" to "to". Moving to default only needs a reset. Moving from
// default to a style needs no leading reset (there is nothing to cancel).
// Moving between two non-default styles conservatively resets first, since
// partial-reset codes (22/24/...) can't be derived from a diff alone.
func emitStyleTransition(b *strings.Builder, from, to *cellStyle) {
	if to == nil {
		b.WriteString("\x1b[0m")
		return
	}
	if from != nil {
		b.WriteString("\x1b[0m")
	}
	b.WriteString(to.sgrString())
}

// LineRenderer tracks a minimal terminal line buffer for normal shell output.
// It supports cursor movement and overwriting without deleting visible text.
type LineRenderer struct {
	lines      []lineBuffer
	row        int
	col        int
	insertMode bool
	inEscape   bool
	inCSI      bool
	inOSC      bool
	oscEsc     bool
	csiParam   int
	csiHas     bool
	csiSep     bool
	csiParams  []int

	pen        *cellStyle
	styleCache map[cellStyle]*cellStyle

	savedRow   int
	savedCol   int
	savedPen   *cellStyle
	savedValid bool
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
	r.insertMode = false
	r.inEscape = false
	r.inCSI = false
	r.inOSC = false
	r.oscEsc = false
	r.csiParam = 0
	r.csiHas = false
	r.csiSep = false
	r.csiParams = nil
	r.pen = nil
	r.styleCache = nil
	r.savedRow = 0
	r.savedCol = 0
	r.savedPen = nil
	r.savedValid = false
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

func (r *LineRenderer) saveCursor() {
	r.savedRow = r.row
	r.savedCol = r.col
	r.savedPen = r.pen
	r.savedValid = true
}

func (r *LineRenderer) restoreCursor() {
	if !r.savedValid {
		return
	}
	r.row = r.savedRow
	r.col = r.savedCol
	r.pen = r.savedPen
	r.ensureLine(r.row)
}

// internStyle returns a shared pointer for the given style value, or nil
// for the default (zero) style. Returned pointers are never mutated after
// creation — applySGR always derives a fresh value and interns that.
func (r *LineRenderer) internStyle(s cellStyle) *cellStyle {
	if s == (cellStyle{}) {
		return nil
	}
	if r.styleCache == nil {
		r.styleCache = make(map[cellStyle]*cellStyle)
	}
	if p, ok := r.styleCache[s]; ok {
		return p
	}
	p := &s
	r.styleCache[s] = p
	return p
}

// pushCSISeparatorParam handles a ';' inside a CSI sequence: a separator
// always yields a parameter (defaulting to 0 if no digits preceded it).
func (r *LineRenderer) pushCSISeparatorParam() {
	r.csiParams = append(r.csiParams, r.csiParam)
	r.csiParam = 0
	r.csiHas = false
	r.csiSep = true
}

// finishCSIParams handles the final byte of a CSI sequence: a trailing
// parameter is appended only if a digit or a previous separator was seen,
// so a bare "CSI m" yields nil params (an implicit reset for applySGR).
func (r *LineRenderer) finishCSIParams() {
	if r.csiHas || r.csiSep {
		r.csiParams = append(r.csiParams, r.csiParam)
	}
	r.csiParam = 0
	r.csiHas = false
	r.csiSep = false
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
			switch b {
			case '[':
				r.inCSI = true
				r.inEscape = false
				r.csiParam = 0
				r.csiHas = false
				r.csiSep = false
				r.csiParams = nil
			case ']':
				r.inEscape = false
				r.inOSC = true
			case '7':
				r.saveCursor()
				r.inEscape = false
			case '8':
				r.restoreCursor()
				r.inEscape = false
			default:
				r.inEscape = false
			}
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
				r.pushCSISeparatorParam()
				i++
				continue
			case b >= 0x40 && b <= 0x7E:
				r.finishCSIParams()
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
				r.lines[r.row].setCellAt(r.col, " ", 1, r.pen)
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
					if r.insertMode {
						r.lines[r.row].insertSpacesAtCol(r.col, 1)
					}
					r.lines[r.row].setCellAt(r.col, string(rune(b)), 1, r.pen)
					r.col++
					i++
					continue
				}
				width := runewidth.RuneWidth(rn)
				if width < 1 {
					width = 1
				}
				if r.insertMode {
					insertIdx := r.lines[r.row].insertSpacesAtCol(r.col, width)
					r.lines[r.row].setCellAt(r.col, string(rn), width, r.pen)
					if width > 1 {
						r.lines[r.row].deleteCellsAtIndex(insertIdx+1, width-1)
					}
				} else {
					r.lines[r.row].setCellAt(r.col, string(rn), width, r.pen)
				}
				r.col += width
				i += size
			} else {
				i++
			}
		}
	}
}

// applySGR updates the current pen (rendition) from CSI "m" parameters. An
// empty/nil params slice means a bare "CSI m", equivalent to SGR 0 (reset).
func (r *LineRenderer) applySGR(params []int) {
	if len(params) == 0 {
		params = []int{0}
	}
	cur := cellStyle{}
	if r.pen != nil {
		cur = *r.pen
	}
	for i := 0; i < len(params); i++ {
		p := params[i]
		switch {
		case p == 0:
			cur = cellStyle{}
		case p == 1:
			cur.attrs |= attrBold
		case p == 2:
			cur.attrs |= attrDim
		case p == 3:
			cur.attrs |= attrItalic
		case p == 4:
			cur.attrs |= attrUnderline
		case p == 5:
			cur.attrs |= attrBlink
		case p == 7:
			cur.attrs |= attrReverse
		case p == 8:
			cur.attrs |= attrConceal
		case p == 9:
			cur.attrs |= attrStrike
		case p == 21:
			// SGR 21 is double-underline (ECMA-48), not a bold-off partial
			// reset. Not modeled; explicitly ignored rather than misapplied.
		case p == 22:
			cur.attrs &^= attrBold | attrDim
		case p == 23:
			cur.attrs &^= attrItalic
		case p == 24:
			cur.attrs &^= attrUnderline
		case p == 25:
			cur.attrs &^= attrBlink
		case p == 27:
			cur.attrs &^= attrReverse
		case p == 28:
			cur.attrs &^= attrConceal
		case p == 29:
			cur.attrs &^= attrStrike
		case p >= 30 && p <= 37:
			cur.fg = itoa(p)
		case p == 38:
			if code, next, ok := parseExtendedColor(params, i+1); ok {
				cur.fg = "38;" + code
				i = next - 1
			} else {
				i = len(params)
			}
		case p == 39:
			cur.fg = ""
		case p >= 40 && p <= 47:
			cur.bg = itoa(p)
		case p == 48:
			if code, next, ok := parseExtendedColor(params, i+1); ok {
				cur.bg = "48;" + code
				i = next - 1
			} else {
				i = len(params)
			}
		case p == 49:
			cur.bg = ""
		case p >= 90 && p <= 97:
			cur.fg = itoa(p)
		case p >= 100 && p <= 107:
			cur.bg = itoa(p)
		default:
			// Unknown/unsupported SGR parameter: ignore.
		}
	}
	r.pen = r.internStyle(cur)
}

// parseExtendedColor parses the parameters following a 38 or 48 SGR
// introducer, starting at params[start]. It returns the color fragment
// (e.g. "5;208" or "2;10;20;30", without the leading 38/48), the index just
// past the consumed parameters, and whether the form was well-formed.
// Malformed input (missing index/channels) reports ok=false; the caller
// must then consume the rest of the sequence rather than reinterpret
// leftover parameters as unrelated attributes.
func parseExtendedColor(params []int, start int) (code string, next int, ok bool) {
	if start >= len(params) {
		return "", start, false
	}
	switch params[start] {
	case 5:
		if start+1 >= len(params) {
			return "", start, false
		}
		return "5;" + itoa(params[start+1]), start + 2, true
	case 2:
		if start+3 >= len(params) {
			return "", start, false
		}
		return "2;" + itoa(params[start+1]) + ";" + itoa(params[start+2]) + ";" + itoa(params[start+3]), start + 4, true
	default:
		return "", start, false
	}
}

func (r *LineRenderer) handleCSI(final byte) {
	switch final {
	case 'm':
		r.applySGR(r.csiParams)
	case 's':
		r.saveCursor()
	case 'u':
		r.restoreCursor()
	case 'A':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.row -= n
		if r.row < 0 {
			r.row = 0
		}
	case 'B':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.row += n
		r.ensureLine(r.row)
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
	case 'G':
		col := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			col = r.csiParams[0]
		}
		r.col = col - 1
		if r.col < 0 {
			r.col = 0
		}
	case 'F':
		r.ensureLine(r.row)
		r.col = r.lines[r.row].visibleLen()
	case 'J':
		param := 0
		if len(r.csiParams) > 0 {
			param = r.csiParams[0]
		}
		switch param {
		case 0: // Erase from cursor to end of display
			r.ensureLine(r.row)
			r.lines[r.row].truncateFromCol(r.col)
			if r.row+1 < len(r.lines) {
				r.lines = r.lines[:r.row+1]
			}
		case 2: // Erase entire display
			r.lines = r.lines[:0]
			r.row = 0
			r.col = 0
			r.ensureLine(0)
		}
	case 'K':
		r.ensureLine(r.row)
		param := 0
		if len(r.csiParams) > 0 {
			param = r.csiParams[0]
		}
		switch param {
		case 0: // Clear from cursor to end of line
			r.lines[r.row].truncateFromCol(r.col)
		case 2: // Clear entire line
			r.lines[r.row] = lineBuffer{}
		}
	case 'P':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.ensureLine(r.row)
		r.lines[r.row].deleteCellsAtCol(r.col, n)
	case '@':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.ensureLine(r.row)
		r.lines[r.row].insertSpacesAtCol(r.col, n)
	case 'X':
		n := 1
		if len(r.csiParams) > 0 && r.csiParams[0] > 0 {
			n = r.csiParams[0]
		}
		r.ensureLine(r.row)
		for i := 0; i < n; i++ {
			r.lines[r.row].setCellAt(r.col+i, " ", 1, nil)
		}
	case 'h':
		if hasCSIParam(r.csiParams, 4) {
			r.insertMode = true
		}
	case 'l':
		if hasCSIParam(r.csiParams, 4) {
			r.insertMode = false
		}
	case 'H', 'f':
		if len(r.csiParams) == 0 {
			r.row = 0
			r.col = 0
			r.ensureLine(0)
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

func hasCSIParam(params []int, target int) bool {
	for _, p := range params {
		if p == target {
			return true
		}
	}
	return false
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
