# WTF CLI Interactive Mode Proposal

## Executive Summary

This document proposes the implementation of an interactive mode for the WTF CLI project. This feature will allow users to analyze streaming command output in real-time, providing continuous feedback and suggestions as data flows through pipes.

## Overview

Interactive mode extends the basic pipe integration by providing real-time analysis of streaming data. This is particularly useful for monitoring logs, long-running processes, or any scenario where continuous analysis is beneficial.

## Use Cases

### Real-time Log Monitoring
```bash
tail -f /var/log/syslog | wtf --interactive
```

### Long-running Process Monitoring
```bash
make build 2>&1 | wtf --interactive
```

### Stream Analysis with Triggers
```bash
docker logs -f container_name | wtf --interactive --trigger-on-error
```

## Architecture

### InteractivePipeHandler

```go
package shell

import (
    "bufio"
    "fmt"
    "os"
    "strings"
    "time"
)

// InteractivePipeHandler handles real-time analysis of streaming data
type InteractivePipeHandler struct {
    cfg        config.Config
    chunkSize  int
    isActive   bool
    triggers   []string
}

func NewInteractivePipeHandler(cfg config.Config, chunkSize int) *InteractivePipeHandler {
    return &InteractivePipeHandler{
        cfg:       cfg,
        chunkSize: chunkSize,
        isActive:  true,
        triggers:  []string{"error:", "failed:", "exception:", "fatal:"},
    }
}

func (h *InteractivePipeHandler) ProcessStream() error {
    scanner := bufio.NewScanner(os.Stdin)
    buffer := ""
    
    for scanner.Scan() && h.isActive {
        line := scanner.Text()
        buffer += line + "\n"
        
        // Process when buffer reaches chunk size or on specific triggers
        if len(buffer) >= h.chunkSize || h.shouldProcessChunk(line) {
            if err := h.processChunk(buffer); err != nil {
                return err
            }
            buffer = ""
        }
    }
    
    // Process remaining buffer
    if buffer != "" {
        return h.processChunk(buffer)
    }
    
    return scanner.Err()
}

func (h *InteractivePipeHandler) shouldProcessChunk(line string) bool {
    // Process on error indicators
    for _, trigger := range h.triggers {
        if strings.Contains(strings.ToLower(line), trigger) {
            return true
        }
    }
    return false
}

func (h *InteractivePipeHandler) processChunk(chunk string) error {
    // Create command info for this chunk
    cmdInfo := CommandInfo{
        Command:   "[Interactive Stream]",
        Output:    chunk,
        ExitCode:  0,
        PipeInput: chunk,
        Source:    SourceInteractive,
        Timestamp: time.Now(),
    }
    
    // Get AI suggestion
    osInfo, err := system.GetOSInfo()
    if err != nil {
        return err
    }
    
    suggestion, err := getAISuggestion(h.cfg, cmdInfo, osInfo)
    if err != nil {
        return err
    }
    
    // Display suggestion
    displayInteractiveSuggestion(cmdInfo, suggestion)
    return nil
}

func displayInteractiveSuggestion(cmdInfo CommandInfo, suggestion string) {
    fmt.Printf("\n🔄 Real-time Analysis [%s]\n", time.Now().Format("15:04:05"))
    fmt.Println(strings.Repeat("─", 50))
    fmt.Println(suggestion)
    fmt.Println(strings.Repeat("─", 50))
}
```

## Command Line Interface

```go
var (
    interactive = flag.Bool("interactive", false, "Interactive pipe analysis")
    chunkSize   = flag.Int("chunk-size", 4096, "Chunk size for streaming analysis")
    triggers    = flag.String("triggers", "", "Comma-separated list of trigger words")
)
```

## Configuration

```json
{
  "interactive_mode": {
    "enabled": true,
    "chunk_size": 4096,
    "triggers": ["error:", "failed:", "exception:", "fatal:"],
    "auto_process_on_trigger": true,
    "buffer_timeout": 5000
  }
}
```

## Testing Strategy

### Unit Tests

```go
func TestInteractiveMode(t *testing.T) {
    tests := []struct {
        name     string
        input    []string
        triggers []string
        expected int // Expected number of analysis calls
    }{
        {
            name:     "trigger on error",
            input:    []string{"INFO: Starting", "ERROR: Failed to connect", "INFO: Continuing"},
            triggers: []string{"error:"},
            expected: 1,
        },
        {
            name:     "chunk size trigger",
            input:    make([]string, 100), // Large input
            triggers: []string{},
            expected: 1, // Should trigger on chunk size
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test interactive mode logic
        })
    }
}
```

## Implementation Timeline

### Phase 1: Core Interactive Support (Week 1-2)
- [ ] Implement InteractivePipeHandler
- [ ] Add streaming input processing
- [ ] Basic trigger system

### Phase 2: Enhanced Features (Week 3-4)
- [ ] Configurable triggers
- [ ] Buffer management
- [ ] Error handling

### Phase 3: Testing and Polish (Week 5-6)
- [ ] Comprehensive testing
- [ ] Performance optimization
- [ ] Documentation

## Benefits

- **Real-time Feedback**: Immediate analysis of streaming data
- **Error Detection**: Automatic triggering on error conditions
- **Flexible Configuration**: Customizable triggers and chunk sizes
- **Resource Efficient**: Chunked processing to manage memory

## Future Enhancements

- **Machine Learning Triggers**: Learn from user behavior
- **Multi-stream Support**: Handle multiple input streams
- **Advanced Filtering**: Content-based filtering and analysis
- **Integration with Monitoring**: Connect with system monitoring tools
