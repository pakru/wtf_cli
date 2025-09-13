package main

import (
	"os"
	"strings"
	"testing"

	"wtf_cli/logger"
)

func TestGetCurrentWorkingDir(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	wd := getCurrentWorkingDir()

	// Should return a non-empty directory path
	if wd == "" {
		t.Error("Expected non-empty working directory")
	}

	// Should not contain leading/trailing whitespace
	if strings.TrimSpace(wd) != wd {
		t.Error("Working directory should not contain leading/trailing whitespace")
	}

	t.Logf("Working Directory: %s", wd)
}

func TestGetShellInfo(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	shell := getShellInfo()

	// Should return a non-empty shell path
	if shell == "" {
		t.Error("Expected non-empty shell information")
	}

	// Should not contain leading/trailing whitespace
	if strings.TrimSpace(shell) != shell {
		t.Error("Shell info should not contain leading/trailing whitespace")
	}

	// Should be a valid shell path
	if !strings.HasPrefix(shell, "/") {
		t.Error("Shell path should start with /")
	}

	t.Logf("Shell: %s", shell)
}

func TestGetUserInfo(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	user := getUserInfo()

	// User might be empty in some environments, so we don't require it
	// But if it's not empty, it should be valid
	if user != "" {
		if strings.TrimSpace(user) != user {
			t.Error("User info should not contain leading/trailing whitespace")
		}
	}

	t.Logf("User: %s", user)
}

func TestGetHomeDir(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	home := getHomeDir()

	// Home directory might be empty in some environments, so we don't require it
	// But if it's not empty, it should be valid
	if home != "" {
		if strings.TrimSpace(home) != home {
			t.Error("Home directory should not contain leading/trailing whitespace")
		}
		if !strings.HasPrefix(home, "/") {
			t.Error("Home directory path should start with /")
		}
	}

	t.Logf("Home Directory: %s", home)
}

func TestHelperFunctionsConsistency(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	// Call helper functions multiple times and ensure consistent results
	wd1 := getCurrentWorkingDir()
	wd2 := getCurrentWorkingDir()
	
	shell1 := getShellInfo()
	shell2 := getShellInfo()
	
	user1 := getUserInfo()
	user2 := getUserInfo()
	
	home1 := getHomeDir()
	home2 := getHomeDir()

	// Results should be identical
	if wd1 != wd2 {
		t.Errorf("Inconsistent working directory: %s vs %s", wd1, wd2)
	}
	if shell1 != shell2 {
		t.Errorf("Inconsistent shell: %s vs %s", shell1, shell2)
	}
	if user1 != user2 {
		t.Errorf("Inconsistent user: %s vs %s", user1, user2)
	}
	if home1 != home2 {
		t.Errorf("Inconsistent home: %s vs %s", home1, home2)
	}
}

func TestHelperFunctionsWithModifiedEnvironment(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	// Save original environment variables
	originalUser := os.Getenv("USER")
	originalHome := os.Getenv("HOME")
	originalShell := os.Getenv("SHELL")

	// Temporarily modify environment variables
	os.Setenv("USER", "test-user")
	os.Setenv("HOME", "/tmp/test-home")
	os.Setenv("SHELL", "/bin/test-shell")

	// Restore original values after test
	defer func() {
		if originalUser != "" {
			os.Setenv("USER", originalUser)
		} else {
			os.Unsetenv("USER")
		}
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		} else {
			os.Unsetenv("HOME")
		}
		if originalShell != "" {
			os.Setenv("SHELL", originalShell)
		} else {
			os.Unsetenv("SHELL")
		}
	}()

	// Verify that the functions pick up the modified environment
	if getUserInfo() != "test-user" {
		t.Errorf("Expected user 'test-user', got '%s'", getUserInfo())
	}
	if getHomeDir() != "/tmp/test-home" {
		t.Errorf("Expected home '/tmp/test-home', got '%s'", getHomeDir())
	}
	if getShellInfo() != "/bin/test-shell" {
		t.Errorf("Expected shell '/bin/test-shell', got '%s'", getShellInfo())
	}
}

func TestHelperFunctionsWithMissingEnvironment(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	// Save original environment variables
	originalUser := os.Getenv("USER")
	originalHome := os.Getenv("HOME")
	originalShell := os.Getenv("SHELL")

	// Temporarily remove environment variables
	os.Unsetenv("USER")
	os.Unsetenv("HOME")
	os.Unsetenv("SHELL")

	// Restore original values after test
	defer func() {
		if originalUser != "" {
			os.Setenv("USER", originalUser)
		}
		if originalHome != "" {
			os.Setenv("HOME", originalHome)
		}
		if originalShell != "" {
			os.Setenv("SHELL", originalShell)
		}
	}()

	// Functions should handle missing environment gracefully
	user := getUserInfo()
	home := getHomeDir()
	shell := getShellInfo()

	// User and home might be empty, but shell should have a fallback
	if shell == "" {
		t.Error("Shell should have a fallback value when SHELL env var is missing")
	}
	if shell != "/bin/bash" {
		t.Errorf("Expected fallback shell '/bin/bash', got '%s'", shell)
	}

	t.Logf("With missing env - User: %s, Home: %s, Shell: %s", user, home, shell)
}
