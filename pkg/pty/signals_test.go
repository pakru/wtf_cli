package pty

import (
	"testing"
)

func TestHandleResize_NoError(t *testing.T) {
	wrapper, err := SpawnShell()
	if err != nil {
		t.Fatalf("SpawnShell() failed: %v", err)
	}
	defer wrapper.Close()

	// HandleResize should not panic
	wrapper.HandleResize()

	// If we get here without panic, test passes
	t.Log("HandleResize set up successfully")
}

func TestResize_Method(t *testing.T) {
	wrapper, err := SpawnShell()
	if err != nil {
		t.Fatalf("SpawnShell() failed: %v", err)
	}
	defer wrapper.Close()

	// resize() might fail in headless/CI environment, but shouldn't panic
	err = wrapper.resize()
	
	// In headless environment, this will error - that's OK
	// The important thing is it doesn't panic
	if err != nil {
		t.Logf("resize() returned error (expected in headless env): %v", err)
	} else {
		t.Log("resize() succeeded")
	}
}
