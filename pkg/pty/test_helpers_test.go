package pty

import (
	"errors"
	"strings"
	"syscall"
	"testing"
)

func requirePTY(t *testing.T) *Wrapper {
	t.Helper()
	wrapper, err := SpawnShell()
	if err != nil {
		if ptyUnavailable(err) {
			t.Skipf("PTY unavailable: %v", err)
		}
		t.Fatalf("SpawnShell() failed: %v", err)
	}
	return wrapper
}

func requireBufferedPTY(t *testing.T, size int) *BufferedWrapper {
	t.Helper()
	wrapper, err := SpawnShellWithBuffer(size)
	if err != nil {
		if ptyUnavailable(err) {
			t.Skipf("PTY unavailable: %v", err)
		}
		t.Fatalf("SpawnShellWithBuffer() failed: %v", err)
	}
	return wrapper
}

func ptyUnavailable(err error) bool {
	if err == nil {
		return false
	}

	msg := strings.ToLower(err.Error())
	if !strings.Contains(msg, "ptmx") && !strings.Contains(msg, "pty") {
		return false
	}

	if errors.Is(err, syscall.EPERM) || errors.Is(err, syscall.EACCES) || errors.Is(err, syscall.ENODEV) {
		return true
	}
	if errors.Is(err, syscall.ENOENT) && strings.Contains(msg, "ptmx") {
		return true
	}
	if strings.Contains(msg, "permission denied") || strings.Contains(msg, "operation not permitted") ||
		strings.Contains(msg, "not permitted") || strings.Contains(msg, "no such device") ||
		strings.Contains(msg, "not supported") {
		return true
	}

	return false
}
