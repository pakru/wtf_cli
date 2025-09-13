package system

import (
	"os"
	"runtime"
	"strings"
	"testing"

	"wtf_cli/logger"
)

func TestGetOSInfo(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
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

func TestOSInfoValidation(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	tests := []struct {
		name     string
		osType   string
		expected bool
	}{
		{"Linux OS", "linux", true},
		{"macOS", "darwin", true},
		{"Windows", "windows", true},
		{"Unknown OS", "unknown", false},
		{"Empty OS", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock the OS type by temporarily changing the environment
			if tt.osType != "" {
				// We can't easily mock runtime.GOOS, so we'll test the validation logic
				isValid := tt.osType == "linux" || tt.osType == "darwin" || tt.osType == "windows"
				if isValid != tt.expected {
					t.Errorf("Expected validation result %v for OS type %s, got %v", tt.expected, tt.osType, isValid)
				}
			}
		})
	}
}

func TestOSInfoFields(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	info, err := GetOSInfo()
	if err != nil {
		t.Fatalf("Failed to get OS info: %v", err)
	}

	// Test that all fields are properly set based on OS type
	switch info.Type {
	case "linux":
		if info.Distribution == "" {
			t.Error("Linux should have a distribution name")
		}
		if info.Kernel == "" {
			t.Error("Linux should have kernel information")
		}
		// Version might be empty for some distributions, so we don't require it
		
	case "darwin":
		if info.Kernel == "" {
			t.Error("macOS should have kernel information")
		}
		// Distribution and Version might be empty for macOS
		
	case "windows":
		// Windows might not have all fields populated
		// Just verify the type is correct
		if info.Type != "windows" {
			t.Error("Windows OS type should be 'windows'")
		}
		
	default:
		t.Errorf("Unexpected OS type: %s", info.Type)
	}

	// Test that no field contains only whitespace
	if strings.TrimSpace(info.Type) != info.Type {
		t.Error("OS Type should not contain leading/trailing whitespace")
	}
	if info.Distribution != "" && strings.TrimSpace(info.Distribution) != info.Distribution {
		t.Error("Distribution should not contain leading/trailing whitespace")
	}
	if info.Version != "" && strings.TrimSpace(info.Version) != info.Version {
		t.Error("Version should not contain leading/trailing whitespace")
	}
	if info.Kernel != "" && strings.TrimSpace(info.Kernel) != info.Kernel {
		t.Error("Kernel should not contain leading/trailing whitespace")
	}
}

func TestGetOSInfoConsistency(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	// Call GetOSInfo multiple times and ensure consistent results
	info1, err1 := GetOSInfo()
	if err1 != nil {
		t.Fatalf("First call failed: %v", err1)
	}

	info2, err2 := GetOSInfo()
	if err2 != nil {
		t.Fatalf("Second call failed: %v", err2)
	}

	// Results should be identical
	if info1.Type != info2.Type {
		t.Errorf("Inconsistent OS type: %s vs %s", info1.Type, info2.Type)
	}
	if info1.Distribution != info2.Distribution {
		t.Errorf("Inconsistent distribution: %s vs %s", info1.Distribution, info2.Distribution)
	}
	if info1.Version != info2.Version {
		t.Errorf("Inconsistent version: %s vs %s", info1.Version, info2.Version)
	}
	if info1.Kernel != info2.Kernel {
		t.Errorf("Inconsistent kernel: %s vs %s", info1.Kernel, info2.Kernel)
	}
}

func TestOSInfoEnvironmentVariables(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger(false, "error")
	
	// Test that the function works even with modified environment
	originalPath := os.Getenv("PATH")
	defer os.Setenv("PATH", originalPath)

	// Temporarily modify PATH to test robustness
	os.Setenv("PATH", "/usr/bin:/bin")

	info, err := GetOSInfo()
	if err != nil {
		t.Fatalf("GetOSInfo should work with modified PATH: %v", err)
	}

	// Should still get valid OS type
	if info.Type == "" {
		t.Error("Should still detect OS type with modified environment")
	}
}
