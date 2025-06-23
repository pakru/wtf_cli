package system

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

// OSInfo contains information about the operating system
type OSInfo struct {
	Type         string // OS type (e.g., "linux", "darwin", "windows")
	Distribution string // Linux distribution name (e.g., "Ubuntu", "Fedora")
	Version      string // OS version or distribution version
	Kernel       string // Kernel version
}

// GetOSInfo retrieves information about the current operating system
func GetOSInfo() (OSInfo, error) {
	info := OSInfo{
		Type: runtime.GOOS,
	}

	// Get more detailed information based on OS type
	switch info.Type {
	case "linux":
		// Try to get Linux distribution information
		distInfo, err := getLinuxDistribution()
		if err != nil {
			return info, fmt.Errorf("failed to get Linux distribution: %w", err)
		}
		info.Distribution = distInfo.name
		info.Version = distInfo.version

		// Get kernel version
		kernel, err := getKernelVersion()
		if err != nil {
			return info, fmt.Errorf("failed to get kernel version: %w", err)
		}
		info.Kernel = kernel

	case "darwin":
		// For macOS, get the OS version
		version, err := exec.Command("sw_vers", "-productVersion").Output()
		if err == nil {
			info.Version = strings.TrimSpace(string(version))
		}

		// Get kernel version
		kernel, err := getKernelVersion()
		if err == nil {
			info.Kernel = kernel
		}

	case "windows":
		// For Windows, get the OS version
		version, err := exec.Command("cmd", "/c", "ver").Output()
		if err == nil {
			info.Version = strings.TrimSpace(string(version))
		}
	}

	return info, nil
}

// linuxDistInfo holds information about a Linux distribution
type linuxDistInfo struct {
	name    string
	version string
}

// getLinuxDistribution tries to determine the Linux distribution name and version
func getLinuxDistribution() (linuxDistInfo, error) {
	var distInfo linuxDistInfo

	// Try to use lsb_release command
	lsbRelease, err := exec.Command("lsb_release", "-a").Output()
	if err == nil {
		output := string(lsbRelease)

		// Extract distribution name
		if idx := strings.Index(output, "Distributor ID:"); idx != -1 {
			line := output[idx:]
			if endIdx := strings.Index(line, "\n"); endIdx != -1 {
				line = line[:endIdx]
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				distInfo.name = strings.TrimSpace(parts[1])
			}
		}

		// Extract version
		if idx := strings.Index(output, "Release:"); idx != -1 {
			line := output[idx:]
			if endIdx := strings.Index(line, "\n"); endIdx != -1 {
				line = line[:endIdx]
			}
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				distInfo.version = strings.TrimSpace(parts[1])
			}
		}

		return distInfo, nil
	}

	// If lsb_release fails, try to read /etc/os-release
	osRelease, err := exec.Command("cat", "/etc/os-release").Output()
	if err == nil {
		output := string(osRelease)

		// Extract distribution name
		if idx := strings.Index(output, "NAME="); idx != -1 {
			line := output[idx:]
			if endIdx := strings.Index(line, "\n"); endIdx != -1 {
				line = line[:endIdx]
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				name := strings.TrimSpace(parts[1])
				// Remove quotes if present
				name = strings.Trim(name, "\"'")
				distInfo.name = name
			}
		}

		// Extract version
		if idx := strings.Index(output, "VERSION_ID="); idx != -1 {
			line := output[idx:]
			if endIdx := strings.Index(line, "\n"); endIdx != -1 {
				line = line[:endIdx]
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) == 2 {
				version := strings.TrimSpace(parts[1])
				// Remove quotes if present
				version = strings.Trim(version, "\"'")
				distInfo.version = version
			}
		}

		return distInfo, nil
	}

	// If all else fails, return generic information
	return linuxDistInfo{name: "Linux", version: "Unknown"}, nil
}

// getKernelVersion retrieves the kernel version
func getKernelVersion() (string, error) {
	output, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
