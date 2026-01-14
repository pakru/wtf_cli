package pty

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/creack/pty"
)

// Wrapper manages a pseudo-terminal session
type Wrapper struct {
	ptmx *os.File  // PTY master
	cmd  *exec.Cmd // Child process
}

// SpawnShell creates a new PTY and spawns the user's shell in it
func SpawnShell() (*Wrapper, error) {
	// Get the user's shell from environment, default to /bin/bash
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}

	// Create command to run the shell
	cmd := exec.Command(shell)

	// Inherit environment variables
	cmd.Env = os.Environ()

	// Start the command in a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, fmt.Errorf("failed to start PTY: %w", err)
	}

	return &Wrapper{
		ptmx: ptmx,
		cmd:  cmd,
	}, nil
}

// ProxyIO handles bidirectional I/O between the PTY and stdin/stdout
func (w *Wrapper) ProxyIO() error {
	// Copy stdin to PTY
	go func() {
		io.Copy(w.ptmx, os.Stdin)
	}()

	// Copy PTY to stdout
	io.Copy(os.Stdout, w.ptmx)

	return nil
}

// Wait waits for the shell process to exit
func (w *Wrapper) Wait() error {
	return w.cmd.Wait()
}

// Close cleans up the PTY resources
func (w *Wrapper) Close() error {
	if w.ptmx != nil {
		err := w.ptmx.Close()
		w.ptmx = nil // Prevent double close
		return err
	}
	return nil
}

// GetPTY returns the PTY file for direct access
func (w *Wrapper) GetPTY() *os.File {
	return w.ptmx
}

// GetPID returns the shell process ID
func (w *Wrapper) GetPID() int {
	if w.cmd != nil && w.cmd.Process != nil {
		return w.cmd.Process.Pid
	}
	return 0
}

// GetCwd returns the shell's current working directory
// by reading from /proc/<pid>/cwd (Linux-specific)
func (w *Wrapper) GetCwd() (string, error) {
	pid := w.GetPID()
	if pid == 0 {
		return "", fmt.Errorf("no process running")
	}
	
	// Read the symlink at /proc/<pid>/cwd
	procPath := fmt.Sprintf("/proc/%d/cwd", pid)
	cwd, err := os.Readlink(procPath)
	if err != nil {
		return "", fmt.Errorf("failed to read cwd: %w", err)
	}
	
	return cwd, nil
}
