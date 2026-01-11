package pty

import (
	"os"

	"golang.org/x/term"
)

// Terminal handles raw mode state management
type Terminal struct {
	oldState *term.State
}

// MakeRaw puts the terminal into raw mode and returns a cleanup function
func MakeRaw() (*Terminal, error) {
	// Save current terminal state
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return nil, err
	}

	return &Terminal{
		oldState: oldState,
	}, nil
}

// Restore restores the terminal to its original state
func (t *Terminal) Restore() error {
	if t.oldState != nil {
		return term.Restore(int(os.Stdin.Fd()), t.oldState)
	}
	return nil
}
