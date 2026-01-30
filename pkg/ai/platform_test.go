package ai

import (
	"runtime"
	"testing"
)

func TestParseOsRelease(t *testing.T) {
	content := `NAME=Ubuntu
VERSION=22.04.3 LTS
ID=ubuntu
PRETTY_NAME=Ubuntu 22.04.3 LTS
`
	result := ParseOsRelease(content)

	if result["NAME"] != "Ubuntu" {
		t.Errorf("Expected NAME='Ubuntu', got %q", result["NAME"])
	}
	if result["VERSION"] != "22.04.3 LTS" {
		t.Errorf("Expected VERSION='22.04.3 LTS', got %q", result["VERSION"])
	}
	if result["ID"] != "ubuntu" {
		t.Errorf("Expected ID='ubuntu', got %q", result["ID"])
	}
	if result["PRETTY_NAME"] != "Ubuntu 22.04.3 LTS" {
		t.Errorf("Expected PRETTY_NAME='Ubuntu 22.04.3 LTS', got %q", result["PRETTY_NAME"])
	}
}

func TestParseOsRelease_Quoted(t *testing.T) {
	content := `NAME="Fedora Linux"
VERSION="39 (Workstation Edition)"
PRETTY_NAME="Fedora Linux 39 (Workstation Edition)"
HOME_URL="https://fedoraproject.org/"
BUG_REPORT_URL="https://bugzilla.redhat.com/"
`
	result := ParseOsRelease(content)

	if result["NAME"] != "Fedora Linux" {
		t.Errorf("Expected NAME='Fedora Linux', got %q", result["NAME"])
	}
	if result["VERSION"] != "39 (Workstation Edition)" {
		t.Errorf("Expected VERSION='39 (Workstation Edition)', got %q", result["VERSION"])
	}
	if result["PRETTY_NAME"] != "Fedora Linux 39 (Workstation Edition)" {
		t.Errorf("Expected PRETTY_NAME='Fedora Linux 39 (Workstation Edition)', got %q", result["PRETTY_NAME"])
	}
}

func TestParseOsRelease_Escapes(t *testing.T) {
	content := `NAME="Test \"Distro\""
PATH="C:\\Program Files\\test"
`
	result := ParseOsRelease(content)

	if result["NAME"] != `Test "Distro"` {
		t.Errorf("Expected escaped quotes, got %q", result["NAME"])
	}
	if result["PATH"] != `C:\Program Files\test` {
		t.Errorf("Expected escaped backslash, got %q", result["PATH"])
	}
}

func TestParseOsRelease_Comments(t *testing.T) {
	content := `# This is a comment
NAME=Ubuntu
# Another comment
VERSION=22.04
`
	result := ParseOsRelease(content)

	if result["NAME"] != "Ubuntu" {
		t.Errorf("Expected NAME='Ubuntu', got %q", result["NAME"])
	}
	if result["VERSION"] != "22.04" {
		t.Errorf("Expected VERSION='22.04', got %q", result["VERSION"])
	}
	if _, exists := result["#"]; exists {
		t.Error("Comments should not be parsed as keys")
	}
}

func TestParseOsRelease_MissingPrettyName(t *testing.T) {
	content := `NAME=Arch Linux
ID=arch
`
	result := ParseOsRelease(content)

	// PRETTY_NAME is missing
	if _, exists := result["PRETTY_NAME"]; exists {
		t.Error("PRETTY_NAME should not exist")
	}
	// NAME should be available for fallback
	if result["NAME"] != "Arch Linux" {
		t.Errorf("Expected NAME='Arch Linux', got %q", result["NAME"])
	}
}

func TestPlatformInfo_PromptText_Linux(t *testing.T) {
	info := PlatformInfo{
		OS:     "linux",
		Arch:   "amd64",
		Distro: "Ubuntu 22.04.3 LTS",
		Kernel: "6.5.0-44-generic",
	}

	expected := "The user is on Ubuntu 22.04.3 LTS (Linux 6.5.0-44-generic, amd64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestPlatformInfo_PromptText_Linux_DistroOnly(t *testing.T) {
	info := PlatformInfo{
		OS:     "linux",
		Arch:   "arm64",
		Distro: "Debian 12",
	}

	expected := "The user is on Debian 12 (arm64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestPlatformInfo_PromptText_Linux_KernelOnly(t *testing.T) {
	info := PlatformInfo{
		OS:     "linux",
		Arch:   "amd64",
		Kernel: "5.15.0-generic",
	}

	expected := "The user is on Linux 5.15.0-generic (amd64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestPlatformInfo_PromptText_MacOS(t *testing.T) {
	info := PlatformInfo{
		OS:      "darwin",
		Arch:    "arm64",
		Version: "14.2.1",
	}

	expected := "The user is on macOS 14.2.1 (arm64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestPlatformInfo_PromptText_MacOS_NoVersion(t *testing.T) {
	info := PlatformInfo{
		OS:   "darwin",
		Arch: "amd64",
	}

	expected := "The user is on macOS (amd64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestPlatformInfo_PromptText_Fallback(t *testing.T) {
	info := PlatformInfo{
		OS:   "linux",
		Arch: "amd64",
	}

	expected := "The user is on linux (amd64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestPlatformInfo_PromptText_Unknown(t *testing.T) {
	info := PlatformInfo{
		OS:   "freebsd",
		Arch: "amd64",
	}

	expected := "The user is on freebsd (amd64)."
	if info.PromptText() != expected {
		t.Errorf("Expected %q, got %q", expected, info.PromptText())
	}
}

func TestGetPlatformInfo_Basic(t *testing.T) {
	ResetPlatformCache()
	defer ResetPlatformCache()

	info := GetPlatformInfo()

	// OS and Arch should always be populated from runtime
	if info.OS != runtime.GOOS {
		t.Errorf("Expected OS=%q, got %q", runtime.GOOS, info.OS)
	}
	if info.Arch != runtime.GOARCH {
		t.Errorf("Expected Arch=%q, got %q", runtime.GOARCH, info.Arch)
	}
}

func TestGetPlatformInfo_Cached(t *testing.T) {
	ResetPlatformCache()
	defer ResetPlatformCache()

	info1 := GetPlatformInfo()
	info2 := GetPlatformInfo()

	// Should return same cached result
	if info1.OS != info2.OS || info1.Arch != info2.Arch {
		t.Error("Expected cached results to be identical")
	}
}

func TestResetPlatformCache(t *testing.T) {
	// Get initial info
	info1 := GetPlatformInfo()

	// Reset cache
	ResetPlatformCache()

	// Get info again - should still work
	info2 := GetPlatformInfo()

	// Both should have valid OS/Arch
	if info1.OS == "" || info2.OS == "" {
		t.Error("Expected non-empty OS after reset")
	}
}
