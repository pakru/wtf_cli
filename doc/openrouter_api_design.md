# OpenRouter API Integration Design

## Overview

This document outlines the design and implementation plan for integrating OpenRouter API with the WTF CLI tool. The integration will enable the CLI to send failed command information to the `google/gemma-3-27b` model and receive troubleshooting suggestions.

## OpenRouter API Basics

**Base URL:** `https://openrouter.ai/api/v1`  
**Endpoint:** `/chat/completions`  
**Model:** `google/gemma-3-27b`  
**Authentication:** Bearer token via `Authorization` header

## API Request Structure

### HTTP Request Format
```
POST https://openrouter.ai/api/v1/chat/completions
Authorization: Bearer $OPENROUTER_API_KEY
HTTP-Referer: https://github.com/your-username/wtf-cli
X-Title: WTF CLI
Content-Type: application/json
```

### JSON Request Body
```json
{
  "model": "google/gemma-3-27b",
  "messages": [
    {
      "role": "system",
      "content": "System prompt with troubleshooting instructions"
    },
    {
      "role": "user", 
      "content": "Failed command analysis with system environment"
    }
  ],
  "temperature": 0.7,
  "max_tokens": 1000,
  "stream": false
}
```

## Implementation Plan

### 1. Go Package Structure

Create new package `api/` with the following files:
- `api/client.go` - Main API client implementation
- `api/types.go` - Request/response type definitions
- `api/prompt.go` - Prompt building logic

### 2. Type Definitions (`api/types.go`)

```go
package api

import "time"

// OpenRouterRequest represents the API request structure
type OpenRouterRequest struct {
    Model       string    `json:"model"`
    Messages    []Message `json:"messages"`
    Temperature float64   `json:"temperature,omitempty"`
    MaxTokens   int       `json:"max_tokens,omitempty"`
    Stream      bool      `json:"stream,omitempty"`
}

// Message represents a single message in the conversation
type Message struct {
    Role    string `json:"role"`    // "system", "user", "assistant"
    Content string `json:"content"`
}

// OpenRouterResponse represents the API response structure
type OpenRouterResponse struct {
    ID      string   `json:"id"`
    Object  string   `json:"object"`
    Created int64    `json:"created"`
    Model   string   `json:"model"`
    Choices []Choice `json:"choices"`
    Usage   Usage    `json:"usage"`
}

// Choice represents a response choice
type Choice struct {
    Index        int     `json:"index"`
    Message      Message `json:"message"`
    FinishReason string  `json:"finish_reason"`
}

// Usage represents token usage information
type Usage struct {
    PromptTokens     int `json:"prompt_tokens"`
    CompletionTokens int `json:"completion_tokens"`
    TotalTokens      int `json:"total_tokens"`
}

// APIError represents an error response from the API
type APIError struct {
    Error struct {
        Message string `json:"message"`
        Type    string `json:"type"`
        Code    string `json:"code"`
    } `json:"error"`
}
```

### 3. API Client Implementation (`api/client.go`)

```go
package api

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

const (
    DefaultBaseURL     = "https://openrouter.ai/api/v1"
    DefaultModel       = "google/gemma-3-27b"
    DefaultTemperature = 0.7
    DefaultMaxTokens   = 1000
    DefaultTimeout     = 30 * time.Second
)

// Client handles OpenRouter API interactions
type Client struct {
    BaseURL    string
    APIKey     string
    HTTPClient *http.Client
    UserAgent  string
    Referer    string
}

// NewClient creates a new OpenRouter API client
func NewClient(apiKey string) *Client {
    return &Client{
        BaseURL: DefaultBaseURL,
        APIKey:  apiKey,
        HTTPClient: &http.Client{
            Timeout: DefaultTimeout,
        },
        UserAgent: "WTF-CLI/1.0",
        Referer:   "https://github.com/your-username/wtf-cli",
    }
}

// ChatCompletion sends a chat completion request to OpenRouter
func (c *Client) ChatCompletion(req OpenRouterRequest) (*OpenRouterResponse, error) {
    // Set default values
    if req.Model == "" {
        req.Model = DefaultModel
    }
    if req.Temperature == 0 {
        req.Temperature = DefaultTemperature
    }
    if req.MaxTokens == 0 {
        req.MaxTokens = DefaultMaxTokens
    }

    // Marshal request to JSON
    jsonData, err := json.Marshal(req)
    if err != nil {
        return nil, fmt.Errorf("failed to marshal request: %w", err)
    }

    // Create HTTP request
    httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
    if err != nil {
        return nil, fmt.Errorf("failed to create request: %w", err)
    }

    // Set headers
    httpReq.Header.Set("Authorization", "Bearer "+c.APIKey)
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("HTTP-Referer", c.Referer)
    httpReq.Header.Set("X-Title", "WTF CLI")
    httpReq.Header.Set("User-Agent", c.UserAgent)

    // Send request
    resp, err := c.HTTPClient.Do(httpReq)
    if err != nil {
        return nil, fmt.Errorf("failed to send request: %w", err)
    }
    defer resp.Body.Close()

    // Read response body
    body, err := io.ReadAll(resp.Body)
    if err != nil {
        return nil, fmt.Errorf("failed to read response: %w", err)
    }

    // Handle error responses
    if resp.StatusCode != http.StatusOK {
        var apiErr APIError
        if err := json.Unmarshal(body, &apiErr); err != nil {
            return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
        }
        return nil, fmt.Errorf("API error: %s", apiErr.Error.Message)
    }

    // Parse successful response
    var openRouterResp OpenRouterResponse
    if err := json.Unmarshal(body, &openRouterResp); err != nil {
        return nil, fmt.Errorf("failed to unmarshal response: %w", err)
    }

    return &openRouterResp, nil
}
```

### 4. Prompt Building (`api/prompt.go`)

```go
package api

import (
    "fmt"
    "strings"
)

const SystemPrompt = `You are a command-line troubleshooting expert. Your job is to analyze failed shell commands and provide clear, actionable solutions.

RESPONSE GUIDELINES:
- Start with suggestion for next command to run
- Next include brief explanation of what likely went wrong
- Provide specific, copy-pasteable commands to fix the issue
- Include relevant context about why the error occurred
- Keep explanations concise but thorough
- Use code blocks for commands
- If multiple solutions exist, prioritize the most common/likely fix first
- Keep in mind that you are running in cli, so output should be copy-pasteable, and not much text should be included

FORMAT YOUR RESPONSE:
1. Suggest next command to run
2. Brief problem summary
3. Root cause explanation
4. Optional: Prevention tips for the future`

// CommandInfo represents information about a failed command
type CommandInfo struct {
    Command    string
    ExitCode   string
    Output     string
    WorkingDir string
    Duration   string
}

// SystemInfo represents system environment information
type SystemInfo struct {
    OS           string
    Distribution string
    Kernel       string
    Shell        string
    User         string
    Home         string
    GitRepo      string
    SSHAgent     string
}

// BuildPrompt creates the user prompt from command and system information
func BuildPrompt(cmdInfo CommandInfo, sysInfo SystemInfo) string {
    var builder strings.Builder
    
    builder.WriteString("FAILED COMMAND ANALYSIS:\n\n")
    
    // Command details
    builder.WriteString("COMMAND DETAILS:\n")
    builder.WriteString(fmt.Sprintf("- Command: %s\n", cmdInfo.Command))
    builder.WriteString(fmt.Sprintf("- Exit Code: %s\n", cmdInfo.ExitCode))
    if cmdInfo.Output != "" {
        builder.WriteString(fmt.Sprintf("- Output: %s\n", cmdInfo.Output))
    }
    if cmdInfo.WorkingDir != "" {
        builder.WriteString(fmt.Sprintf("- Working Directory: %s\n", cmdInfo.WorkingDir))
    }
    if cmdInfo.Duration != "" {
        builder.WriteString(fmt.Sprintf("- Duration: %s\n", cmdInfo.Duration))
    }
    
    builder.WriteString("\nSYSTEM ENVIRONMENT:\n")
    if sysInfo.OS != "" {
        builder.WriteString(fmt.Sprintf("- OS: %s\n", sysInfo.OS))
    }
    if sysInfo.Distribution != "" {
        builder.WriteString(fmt.Sprintf("- Distribution: %s\n", sysInfo.Distribution))
    }
    if sysInfo.Kernel != "" {
        builder.WriteString(fmt.Sprintf("- Kernel: %s\n", sysInfo.Kernel))
    }
    if sysInfo.Shell != "" {
        builder.WriteString(fmt.Sprintf("- Shell: %s\n", sysInfo.Shell))
    }
    if sysInfo.User != "" {
        builder.WriteString(fmt.Sprintf("- User: %s\n", sysInfo.User))
    }
    if sysInfo.GitRepo != "" {
        builder.WriteString(fmt.Sprintf("- Git Repository: %s\n", sysInfo.GitRepo))
    }
    if sysInfo.SSHAgent != "" {
        builder.WriteString(fmt.Sprintf("- SSH Agent Status: %s\n", sysInfo.SSHAgent))
    }
    
    builder.WriteString("\nPlease analyze what went wrong and provide a solution.")
    
    return builder.String()
}

// CreateChatRequest builds a complete OpenRouter request
func CreateChatRequest(cmdInfo CommandInfo, sysInfo SystemInfo) OpenRouterRequest {
    userPrompt := BuildPrompt(cmdInfo, sysInfo)
    
    return OpenRouterRequest{
        Model: DefaultModel,
        Messages: []Message{
            {
                Role:    "system",
                Content: SystemPrompt,
            },
            {
                Role:    "user",
                Content: userPrompt,
            },
        },
        Temperature: DefaultTemperature,
        MaxTokens:   DefaultMaxTokens,
        Stream:      false,
    }
}
```

## Configuration Integration

### Update `config/config.go`

Add OpenRouter-specific configuration fields:

```go
type Config struct {
    // ... existing fields
    
    // OpenRouter API settings
    OpenRouterAPIKey string `json:"openrouter_api_key"`
    Model           string `json:"model"`
    Temperature     float64 `json:"temperature"`
    MaxTokens       int    `json:"max_tokens"`
    APITimeout      int    `json:"api_timeout_seconds"`
}
```

### Configuration Support

Support these configuration variables:
- `openrouter_api_key` - API key override
- `model` - Model override (default: google/gemma-3-27b)
- `temperature` - Temperature override
- `max_tokens` - Max tokens override
- `api_timeout_seconds` - API timeout override

## Integration with Main Application

### Update `main.go`

```go
func main() {
    // ... existing initialization code
    
    if !cfg.DryRun {
        // Get command and system information
        cmdInfo := getCommandInfo()
        sysInfo := getSystemInfo()
        
        // Create API client and get suggestion
        suggestion, err := getSuggestion(cfg, cmdInfo, sysInfo)
        if err != nil {
            logger.Error("Failed to get suggestion", "error", err)
            os.Exit(1)
        }
        
        fmt.Println(suggestion)
    }
}

func getSuggestion(cfg *config.Config, cmdInfo api.CommandInfo, sysInfo api.SystemInfo) (string, error) {
    client := api.NewClient(cfg.OpenRouterAPIKey)
    
    request := api.CreateChatRequest(cmdInfo, sysInfo)
    
    response, err := client.ChatCompletion(request)
    if err != nil {
        return "", err
    }
    
    if len(response.Choices) == 0 {
        return "", fmt.Errorf("no response choices received")
    }
    
    return response.Choices[0].Message.Content, nil
}
```

## Error Handling Strategy

1. **Network Errors**: Retry with exponential backoff (max 3 attempts)
2. **API Errors**: Log error details and show user-friendly message
3. **Rate Limiting**: Respect rate limits and retry after delay
4. **Invalid API Key**: Clear error message with setup instructions
5. **Timeout**: Configurable timeout with fallback message

## Testing Strategy

1. **Unit Tests**: Test prompt building and request construction
2. **Integration Tests**: Test API client with mock server
3. **Dry Run Mode**: Test without making actual API calls
4. **Error Scenarios**: Test various error conditions

## Security Considerations

1. **API Key Storage**: Store in config file with appropriate permissions (600)
2. **Environment Variables**: Support env var override for CI/CD
3. **Request Logging**: Never log API keys or sensitive data
4. **Error Messages**: Don't expose internal details in user-facing errors

## Future Enhancements (Post-MVP)

1. **Streaming Responses**: Implement real-time response streaming
2. **Response Caching**: Cache responses for identical command failures
3. **Multiple Models**: Support switching between different models
4. **Custom Prompts**: Allow users to customize system prompts
5. **Usage Analytics**: Track API usage and costs

## Implementation Checklist

- [x] Create `api/` package with type definitions
- [x] Implement OpenRouter API client
- [x] Add prompt building logic
- [x] Update configuration structure
- [x] Integrate with main application flow
- [x] Add comprehensive error handling
- [ ] Write unit tests for API package
- [ ] Test with real OpenRouter API
- [ ] Update documentation and README

## Example Usage

```bash
# Set API key
export WTF_OPENROUTER_API_KEY="sk-or-v1-..."

# Run with failed command
git push origin main  # This fails
wtf                   # Get AI-powered suggestion

# Dry run mode
WTF_DRY_RUN=true wtf

# Custom model
WTF_MODEL="anthropic/claude-3-sonnet" wtf
```

This design provides a solid foundation for integrating OpenRouter API into the WTF CLI while maintaining flexibility for future enhancements.
