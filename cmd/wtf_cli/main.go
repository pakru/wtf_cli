package main

import (
	"fmt"
	"os"
	"os/exec"

	"wtf_cli/pkg/pty"
)

func main() {
	// Spawn the shell in a PTY
	wrapper, err := pty.SpawnShell()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error spawning shell: %v\n", err)
		os.Exit(1)
	}
	defer wrapper.Close()

	// Handle terminal resize signals
	wrapper.HandleResize()

	// Proxy I/O between PTY and stdin/stdout
	if err := wrapper.ProxyIO(); err != nil {
		fmt.Fprintf(os.Stderr, "Error proxying I/O: %v\n", err)
		os.Exit(1)
	}

	// Wait for shell to exit
	if err := wrapper.Wait(); err != nil {
		// Exit with the same code as the shell
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}
}
