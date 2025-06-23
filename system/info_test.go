package system

import (
	"runtime"
	"testing"
)

func TestGetOSInfo(t *testing.T) {
	info, err := GetOSInfo()
	if err != nil {
		t.Fatalf("Failed to get OS info: %v", err)
	}

	// Verify that the OS type matches the runtime.GOOS
	if info.Type != runtime.GOOS {
		t.Errorf("Expected OS type %s, got %s", runtime.GOOS, info.Type)
	}

	// Verify that we got some information
	if info.Type == "linux" {
		if info.Distribution == "" {
			t.Error("Expected non-empty distribution name for Linux")
		}
		if info.Version == "" {
			t.Error("Expected non-empty version for Linux")
		}
	}

	// Kernel version should be available on Linux and macOS
	if info.Type == "linux" || info.Type == "darwin" {
		if info.Kernel == "" {
			t.Error("Expected non-empty kernel version")
		}
	}

	// Log the detected information for debugging
	t.Logf("OS Type: %s", info.Type)
	t.Logf("Distribution: %s", info.Distribution)
	t.Logf("Version: %s", info.Version)
	t.Logf("Kernel: %s", info.Kernel)
}
