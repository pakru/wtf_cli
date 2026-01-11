package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"wtf_cli/pkg/pty"
)

func main() {
	// Set up terminal raw mode
	terminal, err := pty.MakeRaw()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw mode: %v\n", err)
		os.Exit(1)
	}
	
	// Ensure terminal is restored on any exit path
	defer terminal.Restore()

	// Spawn the shell in a PTY
	wrapper, err := pty.SpawnShell()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error spawning shell: %v\n", err)
		os.Exit(1)
	}
	defer wrapper.Close()

	// Display welcome message
	fmt.Print("\r\n\033[1;36m╔══════════════════════════════════════════════════════╗\033[0m\r\n")
	fmt.Print("\033[1;36m║\033[0m  \033[1;37mWTF CLI\033[0m - AI-Powered Terminal Assistant       \033[1;36m║\033[0m\r\n")
	fmt.Print("\033[1;36m║\033[0m  Type \033[1;33m/wtf\033[0m for AI help • Press \033[1;33mCtrl+D\033[0m to exit   \033[1;36m║\033[0m\r\n")
	fmt.Print("\033[1;36m╚══════════════════════════════════════════════════════╝\033[0m\r\n\r\n")

	// Handle terminal resize signals
	wrapper.HandleResize()

	// Set up signal handlers for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	go func() {
		<-sigChan
		// Cleanup happens via defer
		os.Exit(0)
	}()

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
