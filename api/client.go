package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"wtf_cli/logger"
)

const (
	DefaultBaseURL     = "https://openrouter.ai/api/v1"
	DefaultModel       = "google/gemma-3-27b-it:free"
	DefaultTemperature = 0.7
	DefaultMaxTokens   = 1000
	DefaultTimeout     = 30 * time.Second
)

// Client handles API interactions
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	UserAgent  string
	Referer    string
}

// NewClient creates a new API client
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

// ChatCompletion sends a chat completion request to the API
func (c *Client) ChatCompletion(req Request) (*Response, error) {
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

	logger.Debug("Preparing API request",
		"model", req.Model,
		"temperature", req.Temperature,
		"max_tokens", req.MaxTokens,
		"messages_count", len(req.Messages))

	// Marshal request to JSON
	jsonData, err := json.Marshal(req)
	if err != nil {
		logger.Error("Failed to marshal API request", "error", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Log request body for debugging (with pretty formatting)
	if logger.DebugEnabled() {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, jsonData, "", "  "); err == nil {
			logger.Debug("API request body", "json", prettyJSON.String())
		} else {
			logger.Debug("API request body (raw)", "json", string(jsonData))
		}
	}

	// Mask API key for logging
	maskedKey := ""
	if len(c.APIKey) > 8 {
		maskedKey = c.APIKey[:4] + "..." + c.APIKey[len(c.APIKey)-4:]
	}

	logger.Debug("Sending API request",
		"url", c.BaseURL+"/chat/completions",
		"api_key", maskedKey,
		"request_size", len(jsonData))

	// Create HTTP request
	httpReq, err := http.NewRequest("POST", c.BaseURL+"/chat/completions", bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Error("Failed to create HTTP request", "error", err)
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
		logger.Error("Failed to send API request", "error", err)
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	logger.Debug("Received API response",
		"status_code", resp.StatusCode,
		"content_type", resp.Header.Get("Content-Type"))

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Error("Failed to read response body", "error", err)
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	logger.Debug("API response details",
		"response_size", len(body),
		"status_code", resp.StatusCode)

	// Log response body for debugging (with pretty formatting)
	if logger.DebugEnabled() {
		var prettyJSON bytes.Buffer
		if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
			logger.Debug("API response body", "json", prettyJSON.String())
		} else {
			logger.Debug("API response body (raw)", "json", string(body))
		}
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		// Log error response (truncated for security)
		errorPreview := string(body)
		if len(errorPreview) > 200 {
			errorPreview = errorPreview[:200] + "..."
		}
		logger.Error("API returned error status",
			"status_code", resp.StatusCode,
			"response_preview", errorPreview)

		var apiErr APIError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}
		return nil, fmt.Errorf("API error: %s", apiErr.Error.Message)
	}

	// Parse successful response
	var apiResp Response
	if err := json.Unmarshal(body, &apiResp); err != nil {
		logger.Error("Failed to unmarshal API response", "error", err)
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Log successful response details
	responseContent := ""
	if len(apiResp.Choices) > 0 {
		content := apiResp.Choices[0].Message.Content
		if len(content) > 100 {
			responseContent = content[:100] + "..."
		} else {
			responseContent = content
		}
	}

	logger.Debug("API request completed successfully",
		"response_id", apiResp.ID,
		"model", apiResp.Model,
		"choices_count", len(apiResp.Choices),
		"total_tokens", apiResp.Usage.TotalTokens,
		"prompt_tokens", apiResp.Usage.PromptTokens,
		"completion_tokens", apiResp.Usage.CompletionTokens,
		"response_preview", responseContent)

	return &apiResp, nil
}

// SetTimeout configures the HTTP client timeout
func (c *Client) SetTimeout(timeout time.Duration) {
	c.HTTPClient.Timeout = timeout
}

// SetReferer configures the HTTP-Referer header
func (c *Client) SetReferer(referer string) {
	c.Referer = referer
}

// SetUserAgent configures the User-Agent header
func (c *Client) SetUserAgent(userAgent string) {
	c.UserAgent = userAgent
}
