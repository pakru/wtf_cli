package main

import (
	"fmt"

	"wtf_cli/pkg/version"
)

// printVersion prints the version information
func printVersion() {
	fmt.Printf("wtf_cli version %s\n", version.Version)
	fmt.Printf("  commit: %s\n", version.Commit)
	fmt.Printf("  built: %s\n", version.Date)
	fmt.Printf("  go: %s\n", version.GoVersion)
	fmt.Printf("  platform: %s\n", version.Platform())
}
