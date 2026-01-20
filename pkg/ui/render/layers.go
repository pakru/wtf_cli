package render

// CenterRect returns a rectangle centered within the screen bounds.
// Width/height are clamped to the screen size before centering.
func CenterRect(panelW, panelH, screenW, screenH int) (x, y, w, h int) {
	w = panelW
	h = panelH
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if screenW < 0 {
		screenW = 0
	}
	if screenH < 0 {
		screenH = 0
	}
	if w > screenW {
		w = screenW
	}
	if h > screenH {
		h = screenH
	}
	if screenW > w {
		x = (screenW - w) / 2
	}
	if screenH > h {
		y = (screenH - h) / 2
	}
	return ClampRect(x, y, w, h, screenW, screenH)
}

// ClampRect clamps a rectangle to the screen bounds.
func ClampRect(x, y, w, h, screenW, screenH int) (int, int, int, int) {
	if screenW < 0 {
		screenW = 0
	}
	if screenH < 0 {
		screenH = 0
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > screenW {
		x = screenW
	}
	if y > screenH {
		y = screenH
	}
	if x+w > screenW {
		w = screenW - x
	}
	if y+h > screenH {
		h = screenH - y
	}
	if w < 0 {
		w = 0
	}
	if h < 0 {
		h = 0
	}
	return x, y, w, h
}

// ViewportHeight reserves one line for the status bar.
func ViewportHeight(height int) int {
	if height <= 1 {
		return 0
	}
	return height - 1
}
