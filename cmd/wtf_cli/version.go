package main

import (
	"fmt"
	"runtime"
)

// These variables are set via ldflags during build
var (
	version   = "dev"
	commit    = "none"
	date      = "unknown"
	goVersion = runtime.Version()
)

// printVersion prints the version information
func printVersion() {
	fmt.Printf("wtf_cli version %s\n", version)
	fmt.Printf("  commit: %s\n", commit)
	fmt.Printf("  built: %s\n", date)
	fmt.Printf("  go: %s\n", goVersion)
	fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
