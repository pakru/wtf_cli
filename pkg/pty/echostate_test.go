//go:build linux || darwin

package pty

import (
	"os"
	"testing"
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
