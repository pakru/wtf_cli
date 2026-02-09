//go:build linux

package pty

import (
	"fmt"
	"os"
)

// GetCwd returns the shell's current working directory
// by reading /proc/<pid>/cwd.
func (w *Wrapper) GetCwd() (string, error) {
	pid := w.GetPID()
	if pid == 0 {
		return "", fmt.Errorf("no process running")
	}

	procPath := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(procPath)
	if err != nil {
		return "", fmt.Errorf("failed to read cwd: %w", err)
	}

	return cwd, nil
}
