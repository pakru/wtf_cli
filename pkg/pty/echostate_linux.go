//go:build linux

package pty

import (
	"os"

	"golang.org/x/sys/unix"
)

// IsEchoDisabled checks if the PTY has echo disabled (password entry mode).
// This allows detecting when sudo or similar programs are prompting for passwords.
func IsEchoDisabled(f *os.File) bool {
	if f == nil {
		return false
	}
	termios, err := unix.IoctlGetTermios(int(f.Fd()), unix.TCGETS)
	if err != nil {
		return false
	}
	return (termios.Lflag & unix.ECHO) == 0
}
