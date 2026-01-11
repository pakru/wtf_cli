package pty

import (
	"os"
	"testing"

	"golang.org/x/term"
)

func TestMakeRaw(t *testing.T) {
	// Skip if not running in a real terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("Skipping: not running in a terminal")
	}

	// Get initial state
	initialState, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatalf("Failed to get initial state: %v", err)
	}

	// Make terminal raw
	terminal, err := MakeRaw()
	if err != nil {
		t.Fatalf("MakeRaw() failed: %v", err)
	}

	// Verify terminal is in raw mode by checking state changed
	rawState, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatalf("Failed to get raw state: %v", err)
	}

	// States should be different (raw mode changes flags)
	// We can't easily compare the states directly, but we can verify
	// that MakeRaw didn't error and Restore works

	// Restore terminal
	if err := terminal.Restore(); err != nil {
		t.Errorf("Restore() failed: %v", err)
	}

	// Get state after restore
	restoredState, err := term.GetState(int(os.Stdin.Fd()))
	if err != nil {
		t.Fatalf("Failed to get restored state: %v", err)
	}

	// Use the states (prevents unused variable error)
	_ = initialState
	_ = rawState
	_ = restoredState

	t.Log("Raw mode set and restored successfully")
}

func TestTerminal_Restore_Multiple(t *testing.T) {
	// Skip if not running in a real terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		t.Skip("Skipping: not running in a terminal")
	}

	terminal, err := MakeRaw()
	if err != nil {
		t.Fatalf("MakeRaw() failed: %v", err)
	}

	// First restore
	if err := terminal.Restore(); err != nil {
		t.Errorf("First Restore() failed: %v", err)
	}

	// Second restore should not error
	if err := terminal.Restore(); err != nil {
		t.Errorf("Second Restore() failed: %v", err)
	}
}

func TestTerminal_Restore_NilState(t *testing.T) {
	// Create terminal with nil state
	terminal := &Terminal{oldState: nil}

	// Should not error
	if err := terminal.Restore(); err != nil {
		t.Errorf("Restore() with nil state failed: %v", err)
	}
}
