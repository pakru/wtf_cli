# WTF CLI Pipe Integration Proposal - MVP

## Executive Summary

This document proposes the implementation of bash output reading through pipes for the WTF CLI project. This MVP feature will allow users to pipe command output directly to `wtf` for analysis, focusing on the core functionality without additional complexity.

## Current Architecture Analysis

### Existing Command Capture System

The WTF CLI currently employs a sophisticated multi-layered command capture system:

1. **Shell Integration**: Uses bash hooks (`DEBUG` trap and `PROMPT_COMMAND`) to capture command execution
2. **Command Storage**: Stores command metadata in JSON format at `~/.wtf/last_command.json`
3. **Fallback Mechanisms**: Falls back to bash history, environment variables, and heuristic inference

### Current Limitations

- **No Real-time Analysis**: Users must wait for command completion before analysis
- **Limited Output Context**: Output capture is not comprehensive
- **Single Analysis Mode**: Only supports post-execution analysis
- **No Streaming Support**: Cannot handle continuous data streams

## Proposed Pipe Integration

### Use Cases

The MVP pipe integration will support the basic scenario:

```bash
# Basic error analysis
ls /nonexistent 2>&1 | wtf
```

### Architecture Overview

The proposed architecture extends the existing system with minimal disruption:

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   Command       │───▶│   WTF CLI        │───▶│   AI Analysis   │
│   Output        │    │   Pipe Handler   │    │   & Response    │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                              │
                              ▼
                       ┌──────────────────┐
                       │   Shell          │
                       │   Integration    │
                       │   (Fallback)     │
                       └──────────────────┘
```

## Implementation Details

### 1. Pipe Detection and Input Handling

**File**: `main.go`

```go
// Detect if WTF is receiving input via stdin
func isReadingFromPipe() bool {
    stat, _ := os.Stdin.Stat()
    return (stat.Mode() & os.ModeCharDevice) == 0
}

// Handle pipe input with size limits and error handling
func handlePipeInput() (string, error) {
    if !isReadingFromPipe() && !*pipeMode {
        return "", nil
    }
    
    // Implement size limits to prevent memory exhaustion
    var data []byte
    if *maxPipeSize > 0 {
        data = make([]byte, *maxPipeSize)
        n, err := os.Stdin.Read(data)
        if err != nil && err != io.EOF {
            return "", fmt.Errorf("failed to read pipe input: %w", err)
        }
        data = data[:n]
    } else {
        var err error
        data, err = io.ReadAll(os.Stdin)
        if err != nil {
            return "", fmt.Errorf("failed to read pipe input: %w", err)
        }
    }
    
    return string(data), nil
}
```

### 2. Enhanced CommandInfo Structure

**File**: `shell/history.go`

```go
// Extended CommandInfo to support pipe scenarios
type CommandInfo struct {
    Command     string      // The command that was executed
    Output      string      // Combined stdout and stderr
    ExitCode    int         // Exit code of the command
    PipeInput   string      // NEW: Input received via pipe
    Source      InputSource // NEW: How the data was obtained
}

type InputSource int

const (
    SourceShellIntegration InputSource = iota
    SourceHistory
    SourceEnvironment
    SourcePipe
)
```

### 3. Pipe Mode Processing

**File**: `main.go`

```go
// Process pipe input with specialized handling
func processPipeMode(input string, cfg config.Config) error {
    logger.Debug("Processing pipe mode", "input_length", len(input))
    
    // Create command info from pipe input
    cmdInfo := shell.CommandInfo{
        Command:   "[Pipe Input]",
        Output:    input,
        ExitCode:  0, // Unknown for pipe input
        PipeInput: input,
        Source:    shell.SourcePipe,
    }
    
    // Get system information
    osInfo, err := system.GetOSInfo()
    if err != nil {
        logger.Warn("Failed to get OS info", "error", err)
    }
    
    if cfg.DryRun {
        displayPipeDryRun(cmdInfo)
        return nil
    }
    
    // Get AI suggestion for pipe input
    suggestion, err := getAISuggestion(cfg, cmdInfo, osInfo)
    if err != nil {
        return fmt.Errorf("failed to get AI suggestion for pipe input: %w", err)
    }
    
    displayPipeSuggestion(cmdInfo, suggestion)
    return nil
}
```

### 4. Modified Main Function

**File**: `main.go`

```go
func main() {
    // Initialize logger with default settings first
    logger.InitLogger("info")
    
    logger.Debug("wtf CLI utility - Go implementation started")

    // Load configuration from ~/.wtf/config.json
    configPath := config.GetConfigPath()
    cfg, err := config.LoadConfig(configPath)
    if err != nil {
        logger.Error("Failed to load configuration", "error", err, "config_path", configPath)
        os.Exit(1)
    }

    // Re-initialize logger with configuration settings
    logger.InitLogger(cfg.LogLevel)

    logger.Info("Configuration loaded", "config_path", configPath, "dry_run", cfg.DryRun, "log_level", cfg.LogLevel)

    // Check if we're reading from a pipe first
    if pipeInput, err := handlePipeInput(); err == nil && pipeInput != "" {
        logger.Info("Pipe mode detected", "input_length", len(pipeInput))
        if err := processPipeMode(pipeInput, cfg); err != nil {
            logger.Error("Failed to process pipe input", "error", err)
            os.Exit(1)
        }
        return
    }

    // ... existing shell integration code ...
}
```

### 6. Enhanced Display Functions

**File**: `main.go`

```go
// Display pipe-specific suggestions
func displayPipeSuggestion(cmdInfo shell.CommandInfo, suggestion string) {
    headerText := fmt.Sprintf(" < Analysis of piped input (%d bytes) >", len(cmdInfo.PipeInput))
    fmt.Println(headerText)
    fmt.Println(strings.Repeat("═", len(headerText)))
    fmt.Println(suggestion)
    fmt.Println(strings.Repeat("═", len(headerText)))
}

// Display dry run information for pipe mode
func displayPipeDryRun(cmdInfo shell.CommandInfo) {
    fmt.Println("🧪 Pipe Mode - Dry Run")
    fmt.Println("═══════════════════════")
    fmt.Printf("Input size: %d bytes\n", len(cmdInfo.PipeInput))
    fmt.Printf("Input preview: %s\n", truncateString(cmdInfo.PipeInput, 100))
    fmt.Println()
    fmt.Println("💡 Mock Response:")
    fmt.Println("   • Analyzing piped input")
    fmt.Println("   • Providing contextual suggestions")
    fmt.Println("   • No API calls made in dry-run mode")
    fmt.Println()
    fmt.Println("🔧 To use real AI suggestions, set your API key and remove WTF_DRY_RUN")
}

// Utility function for string truncation
func truncateString(s string, maxLen int) string {
    if len(s) <= maxLen {
        return s
    }
    return s[:maxLen] + "..."
}
```

### 8. Shell Integration Enhancement

**File**: `scripts/integration.sh`

```bash
# Enhanced prompt command to handle pipe scenarios
wtf_prompt_command() {
    local exit_code=$?
    
    # Check if the last command involved piping to wtf
    if [[ -n "$WTF_LAST_COMMAND" && "$WTF_LAST_COMMAND" =~ \|.*wtf ]]; then
        # Skip capturing - wtf will handle this via pipe
        WTF_LAST_COMMAND=""
        WTF_COMMAND_START=""
        return
    fi
    
    # Only process if we have a captured command and it's not a wtf internal command
    if [[ -n "$WTF_LAST_COMMAND" && "$WTF_LAST_COMMAND" != wtf_* && "$WTF_LAST_COMMAND" != *wtf_* ]]; then
        wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START" "$exit_code"
        # Clear the variables to prevent duplicate captures
        WTF_LAST_COMMAND=""
        WTF_COMMAND_START=""
    fi
}
```

### 5. Shell Integration Enhancement

**File**: `scripts/integration.sh`

```bash
# Enhanced prompt command to handle pipe scenarios
wtf_prompt_command() {
    local exit_code=$?
    
    # Check if the last command involved piping to wtf
    if [[ -n "$WTF_LAST_COMMAND" && "$WTF_LAST_COMMAND" =~ \|.*wtf ]]; then
        # Skip capturing - wtf will handle this via pipe
        WTF_LAST_COMMAND=""
        WTF_COMMAND_START=""
        return
    fi
    
    # Only process if we have a captured command and it's not a wtf internal command
    if [[ -n "$WTF_LAST_COMMAND" && "$WTF_LAST_COMMAND" != wtf_* && "$WTF_LAST_COMMAND" != *wtf_* ]]; then
        wtf_capture_command "$WTF_LAST_COMMAND" "$WTF_COMMAND_START" "$exit_code"
        # Clear the variables to prevent duplicate captures
        WTF_LAST_COMMAND=""
        WTF_COMMAND_START=""
    fi
}
```

## Testing Strategy

### Unit Tests

**File**: `shell/pipe_test.go`

```go
package shell

import (
    "io"
    "os"
    "strings"
    "testing"
)

func TestPipeInputDetection(t *testing.T) {
    tests := []struct {
        name     string
        setup    func() (*os.File, func())
        expected bool
    }{
        {
            name: "detect pipe input",
            setup: func() (*os.File, func()) {
                r, w := io.Pipe()
                go func() {
                    defer w.Close()
                    w.Write([]byte("test input"))
                }()
                return r, func() { r.Close() }
            },
            expected: true,
        },
        {
            name: "detect non-pipe input",
            setup: func() (*os.File, func()) {
                // Simulate terminal input
                return os.Stdin, func() {}
            },
            expected: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            stdin, cleanup := tt.setup()
            defer cleanup()
            
            // Test pipe detection logic
            // Implementation would go here
        })
    }
}

func TestPipeModeProcessing(t *testing.T) {
    tests := []struct {
        name        string
        input       string
        expectError bool
    }{
        {
            name:        "error output analysis",
            input:       "ls: cannot access '/nonexistent': No such file or directory",
            expectError: false,
        },
        {
            name:        "successful command output",
            input:       "file1.txt\nfile2.txt\nfile3.txt",
            expectError: false,
        },
        {
            name:        "empty input",
            input:       "",
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test pipe mode processing logic
            // Implementation would go here
        })
    }
}
```

### Integration Tests

**File**: `testing/pipe_integration_test.go`

```go
package testing

import (
    "os"
    "os/exec"
    "strings"
    "testing"
)

func TestPipeIntegration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping pipe integration test in short mode")
    }
    
    tests := []struct {
        name        string
        command     string
        pipeInput   string
        expectError bool
    }{
        {
            name:        "error output analysis",
            command:     "wtf --dry-run",
            pipeInput:   "ls: cannot access '/nonexistent': No such file or directory",
            expectError: false,
        },
        {
            name:        "successful command output",
            command:     "wtf --dry-run",
            pipeInput:   "file1.txt\nfile2.txt\nfile3.txt",
            expectError: false,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := exec.Command("bash", "-c", fmt.Sprintf("echo '%s' | %s", tt.pipeInput, tt.command))
            
            output, err := cmd.CombinedOutput()
            if tt.expectError && err == nil {
                t.Errorf("Expected error but got none")
            }
            if !tt.expectError && err != nil {
                t.Errorf("Unexpected error: %v", err)
            }
            
            // Verify output contains expected elements
            outputStr := string(output)
            if !strings.Contains(outputStr, "Pipe Mode") {
                t.Errorf("Expected pipe mode indication in output")
            }
        })
    }
}
```

## Edge Cases and Considerations

### Memory Management

- **Large Input Handling**: Implement streaming for inputs larger than configured limits
- **Memory Limits**: Configurable maximum input size to prevent memory exhaustion
- **Chunked Processing**: Process large inputs in manageable chunks
- **Buffer Management**: Efficient buffer allocation and cleanup

### Error Handling

- **Broken Pipes**: Handle SIGPIPE gracefully when upstream commands terminate
- **Partial Input**: Handle cases where pipe input is incomplete or corrupted
- **Encoding Issues**: Handle different text encodings (UTF-8, ASCII, etc.)
- **Timeout Handling**: Implement timeouts for long-running operations

### Performance Considerations

- **Buffer Size Optimization**: Tune buffer sizes for different input types
- **Concurrent Processing**: Consider concurrent processing for large inputs
- **Caching**: Cache frequently analyzed patterns and results
- **Resource Limits**: Implement CPU and memory usage limits

### User Experience

- **Progress Indicators**: Show progress for large input processing
- **Interactive Mode**: Real-time analysis of streaming data
- **Format Detection**: Automatic detection and handling of different input formats
- **Error Messages**: Clear, actionable error messages for common issues

### Security Considerations

- **Input Validation**: Sanitize and validate all pipe input
- **Size Limits**: Prevent DoS attacks via extremely large inputs
- **Content Filtering**: Option to filter sensitive information before analysis
- **Access Control**: Respect file permissions and access controls

## Implementation Timeline

### Phase 1: Core Pipe Support (Week 1-2)
- [ ] Implement basic pipe detection
- [ ] Add pipe input handling
- [ ] Create enhanced CommandInfo structure
- [ ] Basic pipe mode processing
- [ ] Enhanced display functions

### Phase 2: Testing and Polish (Week 3-4)
- [ ] Unit tests for pipe functionality
- [ ] Integration tests
- [ ] Shell integration updates
- [ ] Documentation updates

### Phase 3: Release Preparation (Week 5-6)
- [ ] User acceptance testing
- [ ] Performance optimization
- [ ] Security review
- [ ] Release documentation

## Benefits

### User Benefits
- **Basic Pipe Analysis**: Analyze command output via pipes
- **Simple Integration**: Minimal changes to existing workflows
- **Better Context**: More comprehensive output analysis
- **Improved Productivity**: Faster feedback and troubleshooting

### Technical Benefits
- **Extensible Architecture**: Easy to add new input sources
- **Backward Compatibility**: Existing functionality remains unchanged
- **Performance**: Efficient handling of various input sizes
- **Maintainability**: Clean separation of concerns

## Risks and Mitigation

### Technical Risks
- **Memory Usage**: Large inputs could cause memory issues
  - *Mitigation*: Implement size limits and streaming
- **Performance Impact**: Processing large inputs could be slow
  - *Mitigation*: Optimize algorithms and add progress indicators
- **Complexity**: Additional code paths increase complexity
  - *Mitigation*: Comprehensive testing and clear documentation

### User Experience Risks
- **Confusion**: Users might not understand pipe mode behavior
  - *Mitigation*: Clear documentation and helpful error messages
- **Breaking Changes**: Existing workflows might be affected
  - *Mitigation*: Maintain backward compatibility and provide migration guides

## Conclusion

The MVP pipe integration feature will provide essential pipe functionality while maintaining simplicity and reliability. This focused approach ensures quick delivery of core value while establishing a foundation for future enhancements.

The simplified implementation reduces complexity and risk while still delivering significant user value. The comprehensive testing strategy ensures robust functionality.

This proposal provides a clear roadmap for implementing the essential pipe integration feature, with interactive mode and advanced features planned for future releases as separate proposals.
