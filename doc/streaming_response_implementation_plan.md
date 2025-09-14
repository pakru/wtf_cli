# Streaming Response Implementation Plan for WTF CLI

## Overview

This document outlines the implementation plan for adding streaming response support to the WTF CLI tool. Streaming responses will provide real-time output from the LLM, improving user experience with immediate feedback and the ability to cancel long-running requests.

## Complexity Assessment: Medium 🟡

**Implementation Complexity Breakdown:**
- **Low complexity**: OpenRouter already supports streaming with a simple `stream: true` parameter
- **Medium complexity**: Server-Sent Events (SSE) parsing and real-time display management
- **Medium complexity**: Graceful error handling and cancellation support

**Estimated Implementation Time:** 4-7 days for complete implementation

## Current Architecture Analysis

The existing codebase has a clean separation that facilitates streaming integration:

### Current Flow
```
main() → getAISuggestion() → client.ChatCompletion() → displayAISuggestion()
```

### Key Components
1. **`api/client.go`**: Contains `ChatCompletion()` method returning complete response
2. **`api/types.go`**: Already has `Stream` field in `Request` struct (currently unused)
3. **`main.go`**: Orchestrates API call and display logic
4. **Display logic**: Simple formatting in `displayAISuggestion()`

## Implementation Plan

### Phase 1: Core Streaming Infrastructure (High Priority)

#### 1.1 New Streaming Types (`api/types.go`)

Add streaming-specific types to handle SSE responses:

```go
// StreamResponse represents a single SSE chunk from OpenRouter
type StreamResponse struct {
    ID      string        `json:"id"`
    Object  string        `json:"object"`
    Created int64         `json:"created"`
    Model   string        `json:"model"`
    Choices []StreamChoice `json:"choices"`
}

// StreamChoice represents a streaming response choice
type StreamChoice struct {
    Index        int          `json:"index"`
    Delta        StreamDelta  `json:"delta"`
    FinishReason *string      `json:"finish_reason"`
}

// StreamDelta represents the incremental content
type StreamDelta struct {
    Role    string `json:"role,omitempty"`
    Content string `json:"content,omitempty"`
}

// StreamCallback defines the function signature for handling stream chunks
type StreamCallback func(StreamResponse) error
```

#### 1.2 SSE Parser Implementation (`api/stream.go` - new file)

Implement robust Server-Sent Events parsing:

```go
package api

import (
    "bufio"
    "encoding/json"
    "fmt"
    "io"
    "strings"
)

// SSEParser handles Server-Sent Events parsing for OpenRouter streaming
type SSEParser struct {
    buffer string
}

// NewSSEParser creates a new SSE parser instance
func NewSSEParser() *SSEParser {
    return &SSEParser{}
}

// ParseStream processes an SSE stream and calls callback for each valid response
func (p *SSEParser) ParseStream(reader io.Reader, callback StreamCallback) error

// ParseChunk processes a chunk of SSE data and returns parsed responses
func (p *SSEParser) ParseChunk(chunk string) ([]StreamResponse, error)

// parseSSELine parses a single SSE line and returns a StreamResponse if valid
func parseSSELine(line string) (*StreamResponse, error)
```

#### 1.3 Streaming Client Method (`api/client.go`)

Add streaming support to the API client:

```go
// ChatCompletionStream sends a streaming chat completion request
func (c *Client) ChatCompletionStream(req Request, callback StreamCallback) error {
    // Set streaming flag
    req.Stream = true
    
    // Create HTTP request with streaming
    // Handle SSE parsing
    // Call callback for each chunk
    // Handle errors and cleanup
}

// Helper method to create streaming HTTP request
func (c *Client) createStreamingRequest(req Request) (*http.Request, error)
```

### Phase 2: User Experience Enhancements (Medium Priority)

#### 2.1 Real-time Display Enhancement (`main.go`)

Update display logic to support streaming:

```go
// displayStreamingResponse handles real-time streaming output
func displayStreamingResponse(client *api.Client, request api.Request) error {
    fmt.Println("══════════════════════════════")
    
    return client.ChatCompletionStream(request, func(response api.StreamResponse) error {
        if len(response.Choices) > 0 && response.Choices[0].Delta.Content != "" {
            fmt.Print(response.Choices[0].Delta.Content)
        }
        return nil
    })
}

// getAISuggestionStreaming - streaming version of getAISuggestion
func getAISuggestionStreaming(cfg config.Config, cmdInfo shell.CommandInfo, osInfo system.OSInfo) error
```

#### 2.2 Configuration Support (`config/config.go`)

Add streaming configuration options:

```go
type OpenRouterConfig struct {
    APIKey              string  `json:"api_key"`
    Model               string  `json:"model"`
    Temperature         float64 `json:"temperature"`
    MaxTokens           int     `json:"max_tokens"`
    APITimeoutSeconds   int     `json:"api_timeout_seconds"`
    
    // New streaming options
    EnableStreaming     bool    `json:"enable_streaming"`
    StreamTimeout       int     `json:"stream_timeout_seconds"`
    StreamBufferSize    int     `json:"stream_buffer_size"`
}
```

Environment variable support:
- `WTF_ENABLE_STREAMING=true`
- `WTF_STREAM_TIMEOUT=30`

#### 2.3 Graceful Cancellation Support (`main.go`)

Implement Ctrl+C handling for stream cancellation:

```go
import (
    "context"
    "os"
    "os/signal"
    "syscall"
)

// setupCancellation sets up graceful cancellation for streaming
func setupCancellation() (context.Context, context.CancelFunc) {
    ctx, cancel := context.WithCancel(context.Background())
    
    c := make(chan os.Signal, 1)
    signal.Notify(c, os.Interrupt, syscall.SIGTERM)
    
    go func() {
        <-c
        logger.Info("Cancellation requested, stopping stream...")
        cancel()
    }()
    
    return ctx, cancel
}
```

### Phase 3: Robustness & Testing (Medium Priority)

#### 3.1 Comprehensive Testing

**Unit Tests (`api/stream_test.go`):**
```go
func TestSSEParser_ParseChunk(t *testing.T)
func TestSSEParser_ParseStream(t *testing.T)
func TestClient_ChatCompletionStream(t *testing.T)
func TestStreamingWithMockServer(t *testing.T)
```

**Integration Tests:**
- Mock SSE server for realistic testing
- Stream parsing validation with malformed data
- Cancellation scenarios
- Error handling edge cases
- Network interruption recovery

#### 3.2 Fallback Logic (`api/client.go`)

Implement automatic fallback to non-streaming:

```go
// ChatCompletionWithFallback attempts streaming first, falls back to regular completion
func (c *Client) ChatCompletionWithFallback(req Request, streamCallback StreamCallback) (*Response, error) {
    if req.Stream {
        err := c.ChatCompletionStream(req, streamCallback)
        if err != nil {
            logger.Warn("Streaming failed, falling back to non-streaming", "error", err)
            req.Stream = false
            return c.ChatCompletion(req)
        }
        return nil, nil // Success via streaming
    }
    return c.ChatCompletion(req)
}
```

## Key Benefits

### User Experience
1. **Real-time Feedback**: Users see responses as they're generated
2. **Perceived Performance**: Feels faster even if total time is similar
3. **Cancellation**: Users can interrupt long responses with Ctrl+C
4. **Modern CLI Feel**: Similar to ChatGPT CLI and other modern AI tools

### Technical Benefits
1. **Reduced Memory Usage**: Process responses incrementally
2. **Better Error Handling**: Can detect and handle errors mid-stream
3. **Improved Responsiveness**: No waiting for complete response

## Implementation Challenges & Solutions

### Challenge 1: SSE Parsing Complexity
**Solution**: Implement robust buffering and line-by-line parsing with proper error handling

### Challenge 2: Terminal Output Management
**Solution**: Use atomic writes and proper formatting to maintain clean output

### Challenge 3: Error Handling
**Solution**: Distinguish between stream errors, HTTP errors, and JSON parsing errors with appropriate fallbacks

### Challenge 4: Testing Complexity
**Solution**: Create mock SSE server and comprehensive test scenarios

## Configuration Examples

### Default Configuration (`~/.wtf/config.json`)
```json
{
  "debug": false,
  "dry_run": false,
  "log_level": "info",
  "openrouter": {
    "api_key": "your-openrouter-api-key",
    "model": "google/gemma-3-27b-it:free",
    "temperature": 0.7,
    "max_tokens": 1000,
    "api_timeout_seconds": 30,
    "enable_streaming": true,
    "stream_timeout_seconds": 45,
    "stream_buffer_size": 1024
  }
}
```

### Environment Variables
```bash
export WTF_ENABLE_STREAMING=true
export WTF_STREAM_TIMEOUT=30
export WTF_DEBUG=true
```

## Usage Examples

### Streaming Enabled
```bash
$ wtf
══════════════════════════════
It looks like you're trying to run `git push` but encountering authentication issues...

Try running:
```bash
git config --global user.email "your-email@example.com"
git config --global user.name "Your Name"
```

The error suggests that Git doesn't know who you are...
══════════════════════════════
```

### Streaming with Cancellation
```bash
$ wtf
══════════════════════════════
It looks like you're trying to run `git push` but encoun^C
Cancellation requested, stopping stream...
Stream cancelled by user.
```

## Migration Strategy

### Backward Compatibility
- Streaming is opt-in via configuration
- Existing non-streaming behavior remains default
- All existing APIs remain unchanged

### Gradual Rollout
1. **Phase 1**: Implement core streaming with manual configuration
2. **Phase 2**: Add UX enhancements and auto-detection
3. **Phase 3**: Make streaming default for supported models

## Success Metrics

### Performance Metrics
- Time to first token (TTFT) < 2 seconds
- Stream processing latency < 100ms per chunk
- Memory usage remains stable during streaming

### User Experience Metrics
- Successful stream completion rate > 95%
- Cancellation response time < 500ms
- Fallback success rate > 99%

## Future Enhancements

### Advanced Features
1. **Progress Indicators**: Show streaming progress with dynamic indicators
2. **Partial Response Caching**: Cache partial responses for retry scenarios
3. **Multiple Model Streaming**: Support streaming from multiple models simultaneously
4. **Stream Quality Metrics**: Monitor and report streaming performance

### Integration Opportunities
1. **Shell Integration**: Real-time suggestions during command typing
2. **IDE Integration**: Streaming responses in editor environments
3. **Web Interface**: Browser-based streaming for web deployments

## Conclusion

Implementing streaming responses in WTF CLI is a valuable enhancement that will significantly improve user experience. The current architecture supports this addition well, and the implementation can be done incrementally with proper fallback mechanisms to ensure reliability.

The estimated 4-7 day implementation timeline provides a robust, well-tested streaming solution that maintains backward compatibility while offering modern, responsive AI interactions.
