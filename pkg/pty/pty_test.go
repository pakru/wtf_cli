package pty

import (
	"os"
	"testing"
)

func TestSpawnShell(t *testing.T) {
	// Test that SpawnShell creates a valid wrapper
	wrapper, err := SpawnShell()
	if err != nil {
		t.Fatalf("SpawnShell() failed: %v", err)
	}
	defer wrapper.Close()

	if wrapper.ptmx == nil {
		t.Error("Expected ptmx to be non-nil")
	}

	if wrapper.cmd == nil {
		t.Error("Expected cmd to be non-nil")
	}

	// Verify the process was started
	if wrapper.cmd.Process == nil {
		t.Error("Expected process to be started")
	}
}

func TestSpawnShell_InheritsEnvironment(t *testing.T) {
	// Set a test environment variable
	testKey := "WTF_TEST_VAR"
	testValue := "test_value_123"
	os.Setenv(testKey, testValue)
	defer os.Unsetenv(testKey)

	wrapper, err := SpawnShell()
	if err != nil {
		t.Fatalf("SpawnShell() failed: %v", err)
	}
	defer wrapper.Close()

	// Verify environment was inherited
	found := false
	for _, env := range wrapper.cmd.Env {
		if env == testKey+"="+testValue {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Environment variable %s not inherited", testKey)
	}
}

func TestWrapper_Close(t *testing.T) {
	wrapper, err := SpawnShell()
	if err != nil {
		t.Fatalf("SpawnShell() failed: %v", err)
	}

	// Close should not error
	if err := wrapper.Close(); err != nil {
		t.Errorf("Close() failed: %v", err)
	}

	// Calling Close again should not panic
	if err := wrapper.Close(); err != nil {
		t.Errorf("Second Close() failed: %v", err)
	}
}
