package pty

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// HandleResize sets up SIGWINCH signal handler to propagate terminal size changes to the PTY
func (w *Wrapper) HandleResize() {
	// Channel for window size change signals
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)

	// Handle resize in a goroutine
	go func() {
		for range ch {
			if err := w.resize(); err != nil {
				// Log error but don't exit - resizing is not critical
				continue
			}
		}
	}()

	// Trigger initial resize to sync current terminal size
	_ = w.resize()
}

// resize gets the current terminal size and updates the PTY
func (w *Wrapper) resize() error {
	// Get current terminal size
	width, height, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return err
	}

	// Set PTY size
	size := &pty.Winsize{
		Rows: uint16(height),
		Cols: uint16(width),
	}

	return pty.Setsize(w.ptmx, size)
}
