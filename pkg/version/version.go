package version

import (
	"fmt"
	"runtime"
)

// These variables are set via ldflags during build.
var (
	Version   = "dev"
	Commit    = "none"
	Date      = "unknown"
	GoVersion = runtime.Version()
)

func Platform() string {
	return runtime.GOOS + "/" + runtime.GOARCH
}

func Summary() string {
	v := Version
	if v == "" {
		v = "dev"
	}
	if Commit != "" && Commit != "none" {
		short := Commit
		if len(short) > 7 {
			short = short[:7]
		}
		return fmt.Sprintf("%s (%s)", v, short)
	}
	return v
}
