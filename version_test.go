package main

import (
	"strings"
	"testing"
)

func TestVersionInfo(t *testing.T) {
	info := VersionInfo()

	// Check that version info contains expected components
	if !strings.Contains(info, "wtf version") {
		t.Errorf("VersionInfo should contain 'wtf version', got: %s", info)
	}

	if !strings.Contains(info, "commit:") {
		t.Errorf("VersionInfo should contain 'commit:', got: %s", info)
	}

	if !strings.Contains(info, "built:") {
		t.Errorf("VersionInfo should contain 'built:', got: %s", info)
	}

	if !strings.Contains(info, "go:") {
		t.Errorf("VersionInfo should contain 'go:', got: %s", info)
	}
}

func TestVersion(t *testing.T) {
	ver := Version()

	// Version should not be empty
	if ver == "" {
		t.Error("Version should not be empty")
	}

	// Default version should be "dev" when not built with ldflags
	if ver != "dev" {
		t.Logf("Version is set to: %s (expected 'dev' for test builds)", ver)
	}
}
