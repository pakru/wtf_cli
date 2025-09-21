package command

import (
	"testing"

	"wtf_cli/config"
	"wtf_cli/logger"
	"wtf_cli/shell"
	"wtf_cli/system"
)

func TestNewCommandHandler(t *testing.T) {
	cfg := config.Config{
		DryRun: true,
	}
	handler := NewCommandHandler(cfg)

	if handler == nil {
		t.Error("NewCommandHandler returned nil")
	}

	if handler.config.DryRun != cfg.DryRun {
		t.Error("Handler config not set correctly")
	}
}

func TestCommandHandler_ProcessCommandMode(t *testing.T) {
	// Initialize logger for tests
	logger.InitLogger("info")

	tests := []struct {
		name        string
		dryRun      bool
		expectError bool
	}{
		{
			name:        "dry_run_mode",
			dryRun:      true,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := config.Config{
				DryRun: tt.dryRun,
			}
			handler := NewCommandHandler(cfg)

			err := handler.ProcessCommandMode()
			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestCommandHandler_HelperFunctions(t *testing.T) {
	cfg := config.Config{
		DryRun: true,
	}
	handler := NewCommandHandler(cfg)

	// Test helper functions don't panic
	_ = handler.getCurrentWorkingDir()
	_ = handler.getShellInfo()
	_ = handler.getUserInfo()
	_ = handler.getHomeDir()
}

// TestCommandIntegrationEndToEnd tests the complete command flow
func TestCommandIntegrationEndToEnd(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		exitCode    int
		output      string
		description string
	}{
		{
			name:        "failed_command_analysis",
			command:     "ls /nonexistent",
			exitCode:    2,
			output:      "ls: cannot access '/nonexistent': No such file or directory",
			description: "Should analyze failed command",
		},
		{
			name:        "successful_command_analysis",
			command:     "ls",
			exitCode:    0,
			output:      "file1.txt\nfile2.txt\nfile3.txt",
			description: "Should analyze successful command",
		},
		{
			name:        "command_with_no_output",
			command:     "touch newfile.txt",
			exitCode:    0,
			output:      "",
			description: "Should handle command with no output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This test would simulate the complete command flow:
			// 1. Create CommandInfo
			// 2. Process command mode with specific info
			// 3. Display results

			t.Logf("Test case: %s", tt.description)
			t.Logf("Command: %q", tt.command)
			t.Logf("Exit code: %d", tt.exitCode)

			// Initialize logger and create handler
			logger.InitLogger("info")
			cfg := config.Config{DryRun: true}
			handler := NewCommandHandler(cfg)

			cmdInfo := shell.CommandInfo{
				Command:  tt.command,
				Output:   tt.output,
				ExitCode: tt.exitCode,
				Source:   shell.SourceEnvironment,
			}

			osInfo := system.OSInfo{
				Type:         "linux",
				Distribution: "ubuntu",
				Version:      "22.04",
				Kernel:       "5.15.0",
			}

			// Test command mode processing with specific info
			err := handler.processCommandWithInfo(cmdInfo, osInfo)
			if err != nil {
				t.Errorf("processCommandWithInfo failed: %v", err)
			}
		})
	}
}
