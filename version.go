package main

import (
	"fmt"
	"runtime"
)

// Version information. These variables are set via -ldflags during build.
var (
	version   = "dev"     // Semantic version (e.g., "1.0.0")
	commit    = "none"    // Git commit hash
	date      = "unknown" // Build date
	goVersion = runtime.Version()
)

// VersionInfo returns formatted version information
func VersionInfo() string {
	return fmt.Sprintf("wtf version %s\n  commit: %s\n  built: %s\n  go: %s",
		version, commit, date, goVersion)
}

// Version returns just the version string
func Version() string {
	return version
}
