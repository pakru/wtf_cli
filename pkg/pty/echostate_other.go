//go:build !linux && !darwin

package pty

import (
	"os"
)

// IsEchoDisabled returns false on unsupported platforms.
// Password protection via echo detection is not available.
func IsEchoDisabled(f *os.File) bool {
	return false
}
