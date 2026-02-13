//go:build linux || darwin

package pty

import (
	"os"
	"testing"

	cpty "github.com/creack/pty"
	"golang.org/x/sys/unix"
)

func TestIsEchoDisabled_NilFile(t *testing.T) {
	// Should return false for nil file
	if IsEchoDisabled(nil) {
		t.Error("Expected IsEchoDisabled(nil) to return false")
	}
}

func TestIsEchoDisabled_InvalidFd(t *testing.T) {
	// Create a temp file (not a TTY) - should return false or handle error gracefully
	f, err := os.CreateTemp("", "echotest")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	// Regular files are not TTYs, should return false (error case)
	result := IsEchoDisabled(f)
	// We don't care about the value, just that it doesn't panic
	t.Logf("IsEchoDisabled on regular file returned: %v", result)
}

func TestIsEchoDisabled_ClosedFile(t *testing.T) {
	// Create and close a file
	f, err := os.CreateTemp("", "echotest")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	name := f.Name()
	defer os.Remove(name)
	f.Close()

	// Closed file should return false (error case)
	if IsEchoDisabled(f) {
		t.Error("Expected IsEchoDisabled on closed file to return false")
	}
}

func TestIsSecretInputMode_NilFile(t *testing.T) {
	if IsSecretInputMode(nil) {
		t.Error("Expected IsSecretInputMode(nil) to return false")
	}
}

func TestIsSecretInputMode_InvalidFd(t *testing.T) {
	f, err := os.CreateTemp("", "secretmodetest")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(f.Name())
	defer f.Close()

	result := IsSecretInputMode(f)
	t.Logf("IsSecretInputMode on regular file returned: %v", result)
}

func TestIsSecretInputMode_ClosedFile(t *testing.T) {
	f, err := os.CreateTemp("", "secretmodetest")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	name := f.Name()
	defer os.Remove(name)
	f.Close()

	if IsSecretInputMode(f) {
		t.Error("Expected IsSecretInputMode on closed file to return false")
	}
}

func TestIsSecretInputMode_TermiosToggle(t *testing.T) {
	ptmx, tty, err := cpty.Open()
	if err != nil {
		if ptyUnavailable(err) {
			t.Skipf("PTY unavailable: %v", err)
		}
		t.Fatalf("Open PTY failed: %v", err)
	}
	defer ptmx.Close()
	defer tty.Close()

	ttyFD := int(tty.Fd())
	original, err := getTermiosForTest(ttyFD)
	if err != nil {
		t.Fatalf("Failed to get termios: %v", err)
	}

	restore := *original
	defer func() {
		_ = setTermiosForTest(ttyFD, &restore)
	}()

	setMode := func(echo, canon bool) {
		next := restore
		if echo {
			next.Lflag |= unix.ECHO
		} else {
			next.Lflag &^= unix.ECHO
		}
		if canon {
			next.Lflag |= unix.ICANON
		} else {
			next.Lflag &^= unix.ICANON
		}
		if err := setTermiosForTest(ttyFD, &next); err != nil {
			t.Fatalf("Failed to set termios (echo=%v canon=%v): %v", echo, canon, err)
		}
	}

	setMode(true, true)
	if IsSecretInputMode(ptmx) {
		t.Error("Expected IsSecretInputMode false for ECHO on + ICANON on")
	}

	setMode(false, false)
	if IsSecretInputMode(ptmx) {
		t.Error("Expected IsSecretInputMode false for ECHO off + ICANON off")
	}

	setMode(false, true)
	if !IsSecretInputMode(ptmx) {
		t.Error("Expected IsSecretInputMode true for ECHO off + ICANON on")
	}
}
