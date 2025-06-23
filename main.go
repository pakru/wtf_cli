package main

import (
	"fmt"
	"log"
	"os"

	"wtf_cli/config"
	"wtf_cli/shell"
	"wtf_cli/system"
)

func main() {
	fmt.Println("wtf CLI utility - Go implementation started.")

	// Load configuration from ~/.wtf/config.json
	configPath := config.GetConfigPath()
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Check if API key is configured
	if err := cfg.Validate(); err != nil {
		fmt.Printf("Configuration error: %v\n", err)
		fmt.Println("Please set your OpenRouter API key in the configuration file:")
		fmt.Printf("  %s\n", configPath)
		os.Exit(1)
	}

	fmt.Println("Configuration loaded successfully.")

	// Get last command, output, and exit code
	cmdInfo, err := shell.GetLastCommand()
	if err != nil {
		log.Printf("Warning: Failed to get last command: %v", err)
	}

	// Get exit code separately if not available from GetLastCommand
	if cmdInfo.ExitCode == 0 {
		exitCode, err := shell.GetLastExitCode()
		if err == nil {
			cmdInfo.ExitCode = exitCode
		}
	}

	// Get current OS information
	osInfo, err := system.GetOSInfo()
	if err != nil {
		log.Printf("Warning: Failed to get OS info: %v", err)
	}

	// Display the collected information
	fmt.Println("\nLast Command Information:")
	fmt.Printf("  Command: %s\n", cmdInfo.Command)
	fmt.Printf("  Exit Code: %d\n", cmdInfo.ExitCode)

	fmt.Println("\nSystem Information:")
	fmt.Printf("  OS Type: %s\n", osInfo.Type)
	if osInfo.Distribution != "" {
		fmt.Printf("  Distribution: %s\n", osInfo.Distribution)
	}
	fmt.Printf("  Version: %s\n", osInfo.Version)
	fmt.Printf("  Kernel: %s\n", osInfo.Kernel)

	// TODO: Prepare payload for LLM
	// TODO: Call OpenRouter.ai API
	// TODO: Display suggestions
}
