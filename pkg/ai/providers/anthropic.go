package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/auth"
)

const (
	anthropicDefaultAPIURL  = "https://api.anthropic.com/v1"
	anthropicDefaultModel   = "claude-3-5-sonnet-20241022"
	anthropicDefaultTimeout = 60
	anthropicAPIVersion     = "2023-06-01"
)

func init() {
	ai.RegisterProvider(ai.ProviderInfo{
		Type:        ai.ProviderAnthropic,
		Name:        "Anthropic",
		Description: "Direct Anthropic Claude API access",
		AuthMethod:  "api_key",
		RequiresKey: true,
	}, NewAnthropicProvider)
}

// AnthropicProvider implements the Provider interface using the Anthropic API.
type AnthropicProvider struct {
	apiKey             string
	apiURL             string
	httpClient         *http.Client
	defaultModel       string
	defaultTemperature float64
	defaultMaxTokens   int
}

// NewAnthropicProvider creates a new Anthropic provider from config.
func NewAnthropicProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	providerCfg := cfg.Config.Providers.Anthropic

	apiKey := providerCfg.APIKey
	if apiKey == "" && cfg.AuthManager != nil {
		creds, err := cfg.AuthManager.Load(string(ai.ProviderAnthropic))
		if err == nil && creds != nil {
			apiKey = creds.AccessToken
		}
	}

	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("anthropic api_key is required")
	}

	apiURL := providerCfg.APIURL
	if apiURL == "" {
		apiURL = anthropicDefaultAPIURL
	}

	model := providerCfg.Model
	if model == "" {
		model = anthropicDefaultModel
	}

	timeout := providerCfg.APITimeoutSeconds
	if timeout <= 0 {
		timeout = anthropicDefaultTimeout
	}

	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	return &AnthropicProvider{
		apiKey:             apiKey,
		apiURL:             apiURL,
		httpClient:         httpClient,
		defaultModel:       model,
		defaultTemperature: providerCfg.Temperature,
		defaultMaxTokens:   providerCfg.MaxTokens,
	}, nil
}

// anthropicRequest is the request body for Anthropic's messages API.
type anthropicRequest struct {
	Model       string             `json:"model"`
	Messages    []anthropicMessage `json:"messages"`
	MaxTokens   int                `json:"max_tokens"`
	Temperature float64            `json:"temperature,omitempty"`
	System      string             `json:"system,omitempty"`
	Stream      bool               `json:"stream,omitempty"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// anthropicResponse is the response from Anthropic's messages API.
type anthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// anthropicStreamEvent represents a streaming event from Anthropic.
type anthropicStreamEvent struct {
	Type  string `json:"type"`
	Index int    `json:"index,omitempty"`
	Delta struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"delta,omitempty"`
	ContentBlock struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content_block,omitempty"`
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *AnthropicProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	anthropicReq, err := p.buildRequest(req, false)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return ai.ChatResponse{}, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.apiURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return ai.ChatResponse{}, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return ai.ChatResponse{}, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ai.ChatResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return ai.ChatResponse{}, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	var anthropicResp anthropicResponse
	if err := json.Unmarshal(respBody, &anthropicResp); err != nil {
		return ai.ChatResponse{}, fmt.Errorf("failed to parse response: %w", err)
	}

	content := ""
	for _, block := range anthropicResp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	return ai.ChatResponse{
		Content: content,
		Model:   anthropicResp.Model,
	}, nil
}

// CreateChatCompletionStream sends a streaming chat completion request.
func (p *AnthropicProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	anthropicReq, err := p.buildRequest(req, true)
	if err != nil {
		return nil, err
	}

	body, err := json.Marshal(anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", p.apiURL+"/messages", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	p.setHeaders(httpReq)

	resp, err := p.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
	}

	return &anthropicStream{
		reader: bufio.NewReader(resp.Body),
		body:   resp.Body,
	}, nil
}

func (p *AnthropicProvider) buildRequest(req ai.ChatRequest, stream bool) (*anthropicRequest, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = p.defaultModel
	}
	if strings.TrimSpace(model) == "" {
		return nil, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return nil, fmt.Errorf("messages are required")
	}

	var systemPrompt string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		if role == "system" || role == "developer" {
			systemPrompt = msg.Content
			continue
		}

		anthropicRole := role
		if anthropicRole != "user" && anthropicRole != "assistant" {
			anthropicRole = "user"
		}

		messages = append(messages, anthropicMessage{
			Role:    anthropicRole,
			Content: msg.Content,
		})
	}

	if len(messages) == 0 {
		return nil, fmt.Errorf("at least one user or assistant message is required")
	}

	temperature := p.defaultTemperature
	if req.Temperature != nil {
		temperature = *req.Temperature
	}

	maxTokens := p.defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if maxTokens <= 0 {
		maxTokens = 4096
	}

	return &anthropicRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		System:      systemPrompt,
		Stream:      stream,
	}, nil
}

func (p *AnthropicProvider) setHeaders(req *http.Request) {
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicAPIVersion)
}

type anthropicStream struct {
	reader  *bufio.Reader
	body    io.ReadCloser
	current string
	err     error
	done    bool
}

func (s *anthropicStream) Next() bool {
	if s.done || s.err != nil {
		return false
	}

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				s.done = true
			} else {
				s.err = err
			}
			return false
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			s.done = true
			return false
		}

		var event anthropicStreamEvent
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			continue
		}

		switch event.Type {
		case "content_block_delta":
			if event.Delta.Type == "text_delta" {
				s.current = event.Delta.Text
				return true
			}
		case "message_stop":
			s.done = true
			return false
		}
	}
}

func (s *anthropicStream) Content() string {
	return s.current
}

func (s *anthropicStream) Err() error {
	return s.err
}

func (s *anthropicStream) Close() error {
	return s.body.Close()
}

// Ensure interface compliance
var _ ai.Provider = (*AnthropicProvider)(nil)

// Blank import to ensure auth package is available
var _ = auth.DefaultAuthPath
