package fullscreen

import (
	"bytes"
	"strings"
	"sync"
	"wtf_cli/pkg/ui/styles"

	"github.com/vito/midterm"
)

// FullScreenPanel displays full-screen terminal applications (vim, nano, htop)
// using the midterm terminal emulator for proper escape sequence handling.
type FullScreenPanel struct {
	mu      sync.Mutex
	vterm   *midterm.Terminal
	visible bool
	width   int
	height  int
}

const fullScreenBorderSize = 1

// NewFullScreenPanel creates a new full-screen panel with the given dimensions
func NewFullScreenPanel(width, height int) *FullScreenPanel {
	contentWidth, contentHeight := ContentSize(width, height)
	vt := midterm.NewTerminal(contentHeight, contentWidth)
	return &FullScreenPanel{
		vterm:  vt,
		width:  width,
		height: height,
	}
}

// Write processes PTY output through the midterm terminal emulator
func (p *FullScreenPanel) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	return p.vterm.Write(data)
}

// View renders the terminal buffer as a string for Bubble Tea
func (p *FullScreenPanel) View() string {
	p.mu.Lock()
	defer p.mu.Unlock()

	contentWidth, contentHeight := ContentSize(p.width, p.height)
	if contentWidth == 0 || contentHeight == 0 {
		return ""
	}

	var lines []string
	for row := 0; row < contentHeight; row++ {
		var buf bytes.Buffer
		if err := p.vterm.RenderLine(&buf, row); err != nil {
			// On error, add empty line
			lines = append(lines, strings.Repeat(" ", contentWidth))
			continue
		}
		line := buf.String()
		// Pad line to full width if needed
		lineWidth := visibleWidth(line)
		if lineWidth < contentWidth {
			line += strings.Repeat(" ", contentWidth-lineWidth)
		}
		lines = append(lines, line)
	}

	content := strings.Join(lines, "\n")

	return styles.FullScreenBoxStyle.
		Width(contentWidth).
		Height(contentHeight).
		Render(content)
}

// Resize updates the terminal dimensions
func (p *FullScreenPanel) Resize(width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.width = width
	p.height = height
	contentWidth, contentHeight := ContentSize(width, height)
	p.vterm.Resize(contentHeight, contentWidth)
}

// Show makes the panel visible
func (p *FullScreenPanel) Show() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visible = true
}

// Hide makes the panel invisible
func (p *FullScreenPanel) Hide() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.visible = false
}

// IsVisible returns whether the panel is currently visible
func (p *FullScreenPanel) IsVisible() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.visible
}

// Reset clears the terminal state
func (p *FullScreenPanel) Reset() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.vterm.Reset()
}

// GetCursor returns the current cursor position
func (p *FullScreenPanel) GetCursor() (row, col int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.vterm.Cursor.Y, p.vterm.Cursor.X
}

// Size returns the current panel dimensions
func (p *FullScreenPanel) Size() (width, height int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.width, p.height
}

func ContentSize(width, height int) (contentWidth, contentHeight int) {
	contentWidth = width - fullScreenBorderSize*2
	contentHeight = height - fullScreenBorderSize*2
	if contentWidth < 1 {
		contentWidth = 1
	}
	if contentHeight < 1 {
		contentHeight = 1
	}
	return contentWidth, contentHeight
}

// visibleWidth calculates the visible width of a string (accounting for ANSI codes)
func visibleWidth(s string) int {
	// Strip ANSI escape codes for width calculation
	inEscape := false
	width := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				inEscape = false
			}
			continue
		}
		width++
	}
	return width
}
