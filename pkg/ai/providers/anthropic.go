package providers

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/auth"
)

const (
	anthropicDefaultAPIURL  = "https://api.anthropic.com/v1"
	anthropicDefaultModel   = "claude-4-6-sonnet"
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

	authSource := "api_key"
	apiKey := providerCfg.APIKey
	if apiKey == "" && cfg.AuthManager != nil {
		creds, err := cfg.AuthManager.Load(string(ai.ProviderAnthropic))
		if err == nil && creds != nil {
			apiKey = creds.AccessToken
			authSource = "oauth"
		}
	}

	if strings.TrimSpace(apiKey) == "" {
		slog.Debug("anthropic_provider_missing_key")
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

	slog.Debug("anthropic_provider_ready",
		"auth_source", authSource,
		"api_url", apiURL,
		"model", model,
		"timeout_seconds", timeout,
	)
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
	Model       string               `json:"model"`
	Messages    []anthropicMessage   `json:"messages"`
	MaxTokens   int                  `json:"max_tokens"`
	Temperature float64              `json:"temperature,omitempty"`
	System      string               `json:"system,omitempty"`
	Stream      bool                 `json:"stream,omitempty"`
	Tools       []anthropicTool      `json:"tools,omitempty"`
	ToolChoice  *anthropicToolChoice `json:"tool_choice,omitempty"`
}

// anthropicMessage carries either a plain text body (Content.text) or a
// polymorphic block list (Content.blocks). Anthropic's messages API accepts
// "content" as either a string or an array of content blocks; we need both
// shapes to express tool_use (assistant) and tool_result (user) turns.
type anthropicMessage struct {
	Role    string           `json:"role"`
	Content anthropicContent `json:"content"`
}

// anthropicContent is the polymorphic message body. When blocks is non-nil it
// marshals as a JSON array; otherwise it marshals as a JSON string.
type anthropicContent struct {
	text   string
	blocks []anthropicContentBlock
}

func (c anthropicContent) MarshalJSON() ([]byte, error) {
	if c.blocks != nil {
		return json.Marshal(c.blocks)
	}
	return json.Marshal(c.text)
}

func (c *anthropicContent) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		return json.Unmarshal(data, &c.text)
	}
	return json.Unmarshal(data, &c.blocks)
}

// anthropicContentBlock is a discriminated union over the block types we
// emit. Only fields relevant to the block's Type are populated.
type anthropicContentBlock struct {
	Type string `json:"type"`

	// type=text
	Text string `json:"text,omitempty"`

	// type=tool_use
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`

	// type=tool_result
	ToolUseID string `json:"tool_use_id,omitempty"`
	Content   string `json:"content,omitempty"`
	IsError   bool   `json:"is_error,omitempty"`
}

// anthropicTool is a tool definition advertised to the model.
type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"input_schema"`
}

// anthropicToolChoice constrains model tool selection. Type is "auto", "any",
// "tool", or "none". Name is required only for "tool".
type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

// anthropicResponse is the response from Anthropic's messages API.
type anthropicResponse struct {
	ID           string                   `json:"id"`
	Type         string                   `json:"type"`
	Role         string                   `json:"role"`
	Content      []anthropicResponseBlock `json:"content"`
	Model        string                   `json:"model"`
	StopReason   string                   `json:"stop_reason"`
	StopSequence string                   `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

type anthropicResponseBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// anthropicStreamEvent represents a streaming event from Anthropic. The
// content_block_start event carries a fully-typed block (text or tool_use);
// content_block_delta events carry text_delta or input_json_delta increments.
type anthropicStreamEvent struct {
	Type         string                      `json:"type"`
	Index        int                         `json:"index,omitempty"`
	Delta        anthropicStreamEventDelta   `json:"delta,omitempty"`
	ContentBlock anthropicStreamContentBlock `json:"content_block,omitempty"`
}

type anthropicStreamEventDelta struct {
	Type        string `json:"type,omitempty"`
	Text        string `json:"text,omitempty"`
	PartialJSON string `json:"partial_json,omitempty"`
	StopReason  string `json:"stop_reason,omitempty"`
}

type anthropicStreamContentBlock struct {
	Type  string          `json:"type,omitempty"`
	Text  string          `json:"text,omitempty"`
	ID    string          `json:"id,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *AnthropicProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	anthropicReq, err := p.buildRequest(req, false)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	slog.Debug("anthropic_chat_request",
		"model", anthropicReq.Model,
		"message_count", len(anthropicReq.Messages),
		"has_system", anthropicReq.System != "",
		"has_temperature", req.Temperature != nil,
		"has_max_tokens", req.MaxTokens != nil,
	)
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
	var toolCalls []ai.ToolCall
	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			content += block.Text
		case "tool_use":
			args := block.Input
			if len(args) == 0 {
				args = json.RawMessage("{}")
			}
			toolCalls = append(toolCalls, ai.ToolCall{
				ID:        block.ID,
				Name:      block.Name,
				Arguments: args,
			})
		}
	}

	return ai.ChatResponse{
		Content:    content,
		Model:      anthropicResp.Model,
		ToolCalls:  toolCalls,
		StopReason: anthropicResp.StopReason,
	}, nil
}

// CreateChatCompletionStream sends a streaming chat completion request.
func (p *AnthropicProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	anthropicReq, err := p.buildRequest(req, true)
	if err != nil {
		return nil, err
	}

	slog.Debug("anthropic_chat_stream_request",
		"model", anthropicReq.Model,
		"message_count", len(anthropicReq.Messages),
		"has_system", anthropicReq.System != "",
		"has_temperature", req.Temperature != nil,
		"has_max_tokens", req.MaxTokens != nil,
	)
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

	var systemPrompts []string
	messages := make([]anthropicMessage, 0, len(req.Messages))

	for _, msg := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "system", "developer":
			if content := strings.TrimSpace(msg.Content); content != "" {
				systemPrompts = append(systemPrompts, content)
			}
			continue

		case "tool":
			block := anthropicContentBlock{
				Type:      "tool_result",
				ToolUseID: msg.ToolCallID,
				Content:   msg.Content,
			}
			// Anthropic groups tool_result blocks under one user message per
			// turn. Append to the previous message if it is already a user
			// message carrying tool_result blocks; otherwise start a new one.
			if n := len(messages); n > 0 && messages[n-1].Role == "user" && isToolResultMessage(messages[n-1]) {
				messages[n-1].Content.blocks = append(messages[n-1].Content.blocks, block)
			} else {
				messages = append(messages, anthropicMessage{
					Role:    "user",
					Content: anthropicContent{blocks: []anthropicContentBlock{block}},
				})
			}

		case "assistant":
			content := assistantContent(msg)
			messages = append(messages, anthropicMessage{Role: "assistant", Content: content})

		default: // "user" and any unrecognized role mapped to user
			messages = append(messages, anthropicMessage{
				Role:    "user",
				Content: anthropicContent{text: msg.Content},
			})
		}
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

	systemPrompt := strings.Join(systemPrompts, "\n\n")

	out := &anthropicRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
		System:      systemPrompt,
		Stream:      stream,
	}

	if len(req.Tools) > 0 {
		tools := make([]anthropicTool, 0, len(req.Tools))
		for _, t := range req.Tools {
			tools = append(tools, anthropicTool{
				Name:        t.Name,
				Description: t.Description,
				InputSchema: t.JSONSchema,
			})
		}
		out.Tools = tools
		if choice := toAnthropicToolChoice(req.ToolChoice); choice != nil {
			out.ToolChoice = choice
		}
	}

	return out, nil
}

// assistantContent converts an assistant ai.Message to Anthropic content. When
// ToolCalls are present, content is a block list (text + tool_use blocks);
// otherwise the simpler string form is used.
func assistantContent(msg ai.Message) anthropicContent {
	if len(msg.ToolCalls) == 0 {
		return anthropicContent{text: msg.Content}
	}
	blocks := make([]anthropicContentBlock, 0, 1+len(msg.ToolCalls))
	if msg.Content != "" {
		blocks = append(blocks, anthropicContentBlock{Type: "text", Text: msg.Content})
	}
	for _, tc := range msg.ToolCalls {
		input := tc.Arguments
		if len(input) == 0 {
			input = json.RawMessage("{}")
		}
		blocks = append(blocks, anthropicContentBlock{
			Type:  "tool_use",
			ID:    tc.ID,
			Name:  tc.Name,
			Input: input,
		})
	}
	return anthropicContent{blocks: blocks}
}

func isToolResultMessage(m anthropicMessage) bool {
	for _, b := range m.Content.blocks {
		if b.Type == "tool_result" {
			return true
		}
	}
	return false
}

func toAnthropicToolChoice(choice string) *anthropicToolChoice {
	switch strings.ToLower(strings.TrimSpace(choice)) {
	case "", "auto":
		return &anthropicToolChoice{Type: "auto"}
	case "none":
		return &anthropicToolChoice{Type: "none"}
	case "required", "any":
		return &anthropicToolChoice{Type: "any"}
	default:
		return &anthropicToolChoice{Type: "tool", Name: choice}
	}
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

	// Tool-call accumulation. Anthropic emits tool_use as a content_block_start
	// (with id+name) followed by input_json_delta events carrying partial JSON
	// fragments, and finalized at content_block_stop. We accumulate by block
	// index and expose the assembled list via ToolCalls() at end-of-stream.
	pendingByIdx map[int]*pendingAnthropicToolCall
	blockOrder   []int
	stopReason   string
	finalized    bool
	toolCalls    []ai.ToolCall
}

type pendingAnthropicToolCall struct {
	ID        string
	Name      string
	Arguments strings.Builder
	isToolUse bool
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
		case "content_block_start":
			if event.ContentBlock.Type == "tool_use" {
				if s.pendingByIdx == nil {
					s.pendingByIdx = make(map[int]*pendingAnthropicToolCall)
				}
				s.pendingByIdx[event.Index] = &pendingAnthropicToolCall{
					ID:        event.ContentBlock.ID,
					Name:      event.ContentBlock.Name,
					isToolUse: true,
				}
				s.blockOrder = append(s.blockOrder, event.Index)
			}
		case "content_block_delta":
			switch event.Delta.Type {
			case "text_delta":
				if event.Delta.Text != "" {
					s.current = event.Delta.Text
					return true
				}
			case "input_json_delta":
				if pc, ok := s.pendingByIdx[event.Index]; ok {
					pc.Arguments.WriteString(event.Delta.PartialJSON)
				}
			}
		case "message_delta":
			if event.Delta.StopReason != "" {
				s.stopReason = event.Delta.StopReason
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

func (s *anthropicStream) ToolCalls() []ai.ToolCall {
	if !s.finalized {
		s.finalize()
	}
	return s.toolCalls
}

func (s *anthropicStream) StopReason() string { return s.stopReason }

func (s *anthropicStream) finalize() {
	s.finalized = true
	if len(s.blockOrder) == 0 {
		return
	}
	out := make([]ai.ToolCall, 0, len(s.blockOrder))
	for _, idx := range s.blockOrder {
		pc := s.pendingByIdx[idx]
		if pc == nil || !pc.isToolUse {
			continue
		}
		args := strings.TrimSpace(pc.Arguments.String())
		if args == "" {
			args = "{}"
		}
		out = append(out, ai.ToolCall{
			ID:        pc.ID,
			Name:      pc.Name,
			Arguments: json.RawMessage(args),
		})
	}
	s.toolCalls = out
}

// Capabilities reports what the Anthropic provider supports.
func (p *AnthropicProvider) Capabilities() ai.ProviderCapabilities {
	return ai.ProviderCapabilities{Streaming: true, Tools: true}
}

// Ensure interface compliance
var _ ai.Provider = (*AnthropicProvider)(nil)

// Blank import to ensure auth package is available
var _ = auth.DefaultAuthPath
