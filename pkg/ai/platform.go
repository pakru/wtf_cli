package ai

import (
	"bufio"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"

	"os"
)

// PlatformInfo contains host platform information for the AI assistant.
type PlatformInfo struct {
	OS      string // "linux", "darwin", "windows"
	Arch    string // "amd64", "arm64"
	Distro  string // Linux only: "Ubuntu 22.04.3 LTS" (from PRETTY_NAME)
	Kernel  string // Linux only: "6.5.0-44-generic"
	Version string // macOS only: "14.2.1"
}

var (
	platformCache *PlatformInfo
	platformOnce  sync.Once
)

// GetPlatformInfo returns cached platform information.
// The result is cached since platform info doesn't change during runtime.
func GetPlatformInfo() PlatformInfo {
	platformOnce.Do(func() {
		platformCache = detectPlatform()
	})
	return *platformCache
}

// ResetPlatformCache clears the cached platform info (for testing).
func ResetPlatformCache() {
	platformOnce = sync.Once{}
	platformCache = nil
}

// PromptText returns a formatted string for inclusion in the system prompt.
// Never returns empty; falls back to basic OS/Arch if details unavailable.
func (p PlatformInfo) PromptText() string {
	switch p.OS {
	case "linux":
		if p.Distro != "" && p.Kernel != "" {
			return fmt.Sprintf("The user is on %s (Linux %s, %s).", p.Distro, p.Kernel, p.Arch)
		}
		if p.Distro != "" {
			return fmt.Sprintf("The user is on %s (%s).", p.Distro, p.Arch)
		}
		if p.Kernel != "" {
			return fmt.Sprintf("The user is on Linux %s (%s).", p.Kernel, p.Arch)
		}
		return fmt.Sprintf("The user is on linux (%s).", p.Arch)

	case "darwin":
		if p.Version != "" {
			return fmt.Sprintf("The user is on macOS %s (%s).", p.Version, p.Arch)
		}
		return fmt.Sprintf("The user is on macOS (%s).", p.Arch)

	default:
		return fmt.Sprintf("The user is on %s (%s).", p.OS, p.Arch)
	}
}

func detectPlatform() *PlatformInfo {
	info := &PlatformInfo{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	switch runtime.GOOS {
	case "linux":
		info.Distro = readOsRelease()
		info.Kernel = readKernelVersion()
	case "darwin":
		info.Version = readMacOSVersion()
	}

	return info
}

// readOsRelease reads and parses /etc/os-release or /usr/lib/os-release.
func readOsRelease() string {
	paths := []string{"/etc/os-release", "/usr/lib/os-release"}

	for _, path := range paths {
		content, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		parsed := ParseOsRelease(string(content))

		// Prefer PRETTY_NAME as recommended by freedesktop.org spec
		if prettyName, ok := parsed["PRETTY_NAME"]; ok && prettyName != "" {
			return prettyName
		}

		// Fallback to NAME + VERSION
		name := parsed["NAME"]
		version := parsed["VERSION"]
		if name != "" {
			if version != "" {
				return name + " " + version
			}
			return name
		}
	}

	return ""
}

// ParseOsRelease parses os-release file content into key-value pairs.
// Handles shell-style syntax: KEY=value, KEY="quoted value", and backslash escapes.
func ParseOsRelease(content string) map[string]string {
	result := make(map[string]string)

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Find the first '=' separator
		idx := strings.Index(line, "=")
		if idx <= 0 {
			continue
		}

		key := line[:idx]
		value := line[idx+1:]

		// Handle quoted values
		value = unquoteValue(value)

		result[key] = value
	}

	return result
}

// unquoteValue removes surrounding quotes and processes escape sequences.
func unquoteValue(s string) string {
	if len(s) < 2 {
		return s
	}

	// Check for double quotes
	if s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
		return processEscapes(s)
	}

	// Check for single quotes (no escape processing)
	if s[0] == '\'' && s[len(s)-1] == '\'' {
		return s[1 : len(s)-1]
	}

	return s
}

// processEscapes handles backslash escape sequences in double-quoted strings.
func processEscapes(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			next := s[i+1]
			switch next {
			case '"', '\\', '$', '`':
				sb.WriteByte(next)
				i++
			default:
				sb.WriteByte(s[i])
			}
		} else {
			sb.WriteByte(s[i])
		}
	}

	return sb.String()
}

// readKernelVersion reads the kernel version from /proc/sys/kernel/osrelease.
func readKernelVersion() string {
	// Fast path: read from proc filesystem (no process spawn)
	content, err := os.ReadFile("/proc/sys/kernel/osrelease")
	if err == nil {
		return strings.TrimSpace(string(content))
	}

	// Fallback: use uname -r
	cmd := exec.Command("uname", "-r")
	output, err := cmd.Output()
	if err == nil {
		return strings.TrimSpace(string(output))
	}

	return ""
}

// readMacOSVersion reads the macOS version using sw_vers.
func readMacOSVersion() string {
	cmd := exec.Command("sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}
