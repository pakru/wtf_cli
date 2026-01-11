package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"wtf_cli/pkg/capture"
	"wtf_cli/pkg/config"
	"wtf_cli/pkg/pty"
	"wtf_cli/pkg/ui"
)

func main() {
	// Load configuration
	cfg, err := config.Load(config.GetConfigPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Set up terminal raw mode
	terminal, err := pty.MakeRaw()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error setting raw mode: %v\n", err)
		os.Exit(1)
	}
	
	// Ensure terminal is restored on any exit path
	defer terminal.Restore()

	// Spawn the shell in a PTY with buffer
	wrapper, err := pty.SpawnShellWithBuffer(cfg.BufferSize)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error spawning shell: %v\n", err)
		os.Exit(1)
	}
	defer wrapper.Close()

	// Initialize session context
	session := capture.NewSessionContext()
	
	// Initialize status bar
	statusBar := ui.NewStatusBar()

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
		// Clear status bar before exit
		fmt.Print(statusBar.Clear())
		// Cleanup happens via defer
		os.Exit(0)
	}()

	// Render status bar periodically
	statusTicker := time.NewTicker(1 * time.Second)
	defer statusTicker.Stop()
	
	go func() {
		for range statusTicker.C {
			// Update directory from current working directory
			if dir, err := os.Getwd(); err == nil {
				statusBar.SetDirectory(dir)
			}
			// Render status bar
			fmt.Print(statusBar.Render())
		}
	}()

	// Proxy I/O between PTY and stdin/stdout with buffering
	if err := wrapper.ProxyIOWithBuffer(); err != nil {
		fmt.Fprintf(os.Stderr, "Error proxying I/O: %v\n", err)
		os.Exit(1)
	}

	// Wait for shell to exit
	if err := wrapper.Wait(); err != nil {
		// Clear status bar
		fmt.Print(statusBar.Clear())
		
		// Exit with the same code as the shell
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	// Clear status bar on clean exit
	fmt.Print(statusBar.Clear())
	
	// Use session context (prevents unused warning)
	_ = session
}
