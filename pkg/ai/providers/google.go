package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"iter"
	"log/slog"
	"strings"
	"time"

	"wtf_cli/pkg/ai"

	"google.golang.org/genai"
)

const (
	googleDefaultModel   = "gemini-3-flash-preview"
	googleDefaultTimeout = 60
)

func init() {
	ai.RegisterProvider(ai.ProviderInfo{
		Type:        ai.ProviderGoogle,
		Name:        "Google",
		Description: "Direct Google AI (Gemini) API access with free tier",
		AuthMethod:  "api_key",
		RequiresKey: true,
	}, NewGoogleProvider)
}

type googleModelsClient interface {
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error]
}

var newGoogleClient = func(ctx context.Context, cfg *genai.ClientConfig) (*genai.Client, error) {
	return genai.NewClient(ctx, cfg)
}

// GoogleProvider implements the Provider interface using the native Google AI SDK.
type GoogleProvider struct {
	models             googleModelsClient
	defaultModel       string
	defaultTemperature float64
	defaultMaxTokens   int
	defaultTimeout     time.Duration
}

// NewGoogleProvider creates a new Google provider from config.
func NewGoogleProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	providerCfg := cfg.Config.Providers.Google

	apiKey := strings.TrimSpace(providerCfg.APIKey)
	if apiKey == "" {
		slog.Debug("google_provider_missing_key")
		return nil, fmt.Errorf("google api_key is required")
	}

	model := strings.TrimSpace(providerCfg.Model)
	if model == "" {
		model = googleDefaultModel
	}

	timeoutSeconds := providerCfg.APITimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = googleDefaultTimeout
	}

	client, err := newGoogleClient(context.Background(), &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("create google client: %w", err)
	}

	slog.Debug("google_provider_ready",
		"model", model,
		"timeout_seconds", timeoutSeconds,
	)
	return &GoogleProvider{
		models:             client.Models,
		defaultModel:       model,
		defaultTemperature: providerCfg.Temperature,
		defaultMaxTokens:   providerCfg.MaxTokens,
		defaultTimeout:     time.Duration(timeoutSeconds) * time.Second,
	}, nil
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *GoogleProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	model, contents, cfg, err := p.buildRequest(req)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	callCtx, cancel := p.withTimeout(ctx)
	defer cancel()

	resp, err := p.models.GenerateContent(callCtx, model, contents, cfg)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	toolCalls, err := extractFunctionCalls(resp)
	if err != nil {
		return ai.ChatResponse{}, fmt.Errorf("extract function calls: %w", err)
	}

	return ai.ChatResponse{
		Content:    extractVisibleText(resp),
		Model:      model,
		ToolCalls:  toolCalls,
		StopReason: extractFinishReason(resp),
	}, nil
}

// CreateChatCompletionStream sends a streaming chat completion request.
func (p *GoogleProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	model, contents, cfg, err := p.buildRequest(req)
	if err != nil {
		return nil, err
	}

	callCtx, cancel := p.withTimeout(ctx)
	stream := p.models.GenerateContentStream(callCtx, model, contents, cfg)
	return newGoogleStream(stream, cancel), nil
}

func (p *GoogleProvider) buildRequest(req ai.ChatRequest) (string, []*genai.Content, *genai.GenerateContentConfig, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = p.defaultModel
	}
	if model == "" {
		return "", nil, nil, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return "", nil, nil, fmt.Errorf("messages are required")
	}

	contents := make([]*genai.Content, 0, len(req.Messages))
	systemParts := make([]string, 0, 2)
	developerParts := make([]string, 0, 2)

	for _, msg := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		switch role {
		case "system":
			if content := strings.TrimSpace(msg.Content); content != "" {
				systemParts = append(systemParts, content)
			}
		case "developer":
			if content := strings.TrimSpace(msg.Content); content != "" {
				developerParts = append(developerParts, content)
			}
		case "assistant":
			parts := assistantParts(msg)
			contents = append(contents, &genai.Content{Role: genai.RoleModel, Parts: parts})
		case "tool":
			// Gemini surfaces tool results as user-role messages with a
			// FunctionResponse part (mirroring how function calls come back as
			// FunctionCall parts on model-role messages).
			part, err := toolResponsePart(msg)
			if err != nil {
				return "", nil, nil, fmt.Errorf("tool message: %w", err)
			}
			contents = append(contents, &genai.Content{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{part},
			})
		default: // "user" and unrecognized roles
			contents = append(contents, &genai.Content{
				Role:  genai.RoleUser,
				Parts: []*genai.Part{{Text: msg.Content}},
			})
		}
	}
	if len(contents) == 0 {
		return "", nil, nil, fmt.Errorf("at least one user or assistant message is required")
	}

	var systemInstruction *genai.Content
	allSystem := make([]string, 0, len(systemParts)+len(developerParts))
	allSystem = append(allSystem, systemParts...)
	allSystem = append(allSystem, developerParts...)
	if len(allSystem) > 0 {
		systemInstruction = &genai.Content{
			Parts: []*genai.Part{
				{Text: strings.Join(allSystem, "\n\n")},
			},
		}
	}

	temperature := p.defaultTemperature
	if req.Temperature != nil {
		temperature = *req.Temperature
	}

	maxTokens := p.defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}

	config := &genai.GenerateContentConfig{
		SystemInstruction: systemInstruction,
		Temperature:       genai.Ptr(float32(temperature)),
		ThinkingConfig: &genai.ThinkingConfig{
			IncludeThoughts: false,
			// Disable explicit thinking generation to avoid exposing planning-style text
			// from Gemini 2.5/3 preview models in user-facing chat.
			ThinkingBudget: genai.Ptr(int32(0)),
		},
	}
	if maxTokens > 0 {
		config.MaxOutputTokens = int32(maxTokens)
	}

	if len(req.Tools) > 0 {
		tools, err := toGoogleTools(req.Tools)
		if err != nil {
			return "", nil, nil, err
		}
		config.Tools = tools
		if tc := toGoogleToolConfig(req.ToolChoice); tc != nil {
			config.ToolConfig = tc
		}
	}

	return model, contents, config, nil
}

// assistantParts converts an assistant ai.Message to genai parts. When ToolCalls
// are present they map to FunctionCall parts (one per call), with optional
// leading text part.
func assistantParts(msg ai.Message) []*genai.Part {
	if len(msg.ToolCalls) == 0 {
		return []*genai.Part{{Text: msg.Content}}
	}
	parts := make([]*genai.Part, 0, 1+len(msg.ToolCalls))
	if msg.Content != "" {
		parts = append(parts, &genai.Part{Text: msg.Content})
	}
	for _, tc := range msg.ToolCalls {
		args := map[string]any{}
		if len(tc.Arguments) > 0 {
			if err := json.Unmarshal(tc.Arguments, &args); err != nil {
				// Fall back to wrapping the raw arguments so the model can still
				// match its prior call by name+id.
				args = map[string]any{"_raw": string(tc.Arguments)}
			}
		}
		parts = append(parts, &genai.Part{
			FunctionCall: &genai.FunctionCall{
				ID:   tc.ID,
				Name: tc.Name,
				Args: args,
			},
			ThoughtSignature: tc.ThoughtSignature,
		})
	}
	return parts
}

// toolResponsePart wraps an ai.Message{Role:"tool"} as a FunctionResponse part.
// The Response map uses an "output" key (or "error" when the tool reported a
// soft failure) per Gemini's tool-result convention.
func toolResponsePart(msg ai.Message) (*genai.Part, error) {
	if strings.TrimSpace(msg.Name) == "" {
		return nil, fmt.Errorf("tool message requires Name")
	}
	responseKey := "output"
	if msg.IsError {
		responseKey = "error"
	}
	return &genai.Part{
		FunctionResponse: &genai.FunctionResponse{
			ID:       msg.ToolCallID,
			Name:     msg.Name,
			Response: map[string]any{responseKey: msg.Content},
		},
	}, nil
}

// toGoogleTools maps our tool definitions to a single Tool with all
// FunctionDeclarations attached, which is the standard Gemini shape.
func toGoogleTools(defs []ai.ToolDefinition) ([]*genai.Tool, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	decls := make([]*genai.FunctionDeclaration, 0, len(defs))
	for _, d := range defs {
		var schema any
		if len(d.JSONSchema) > 0 {
			if err := json.Unmarshal(d.JSONSchema, &schema); err != nil {
				return nil, fmt.Errorf("tool %q: invalid JSON schema: %w", d.Name, err)
			}
		}
		decls = append(decls, &genai.FunctionDeclaration{
			Name:                 d.Name,
			Description:          d.Description,
			ParametersJsonSchema: schema,
		})
	}
	return []*genai.Tool{{FunctionDeclarations: decls}}, nil
}

func toGoogleToolConfig(choice string) *genai.ToolConfig {
	mode := strings.ToLower(strings.TrimSpace(choice))
	switch mode {
	case "", "auto":
		return &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAuto},
		}
	case "none":
		return &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeNone},
		}
	case "required", "any":
		return &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{Mode: genai.FunctionCallingConfigModeAny},
		}
	default:
		return &genai.ToolConfig{
			FunctionCallingConfig: &genai.FunctionCallingConfig{
				Mode:                 genai.FunctionCallingConfigModeAny,
				AllowedFunctionNames: []string{choice},
			},
		}
	}
}

// extractFunctionCalls walks the candidate parts and returns assembled tool
// calls. Args are JSON-marshaled back to a RawMessage to fit our generic
// ToolCall shape.
func extractFunctionCalls(resp *genai.GenerateContentResponse) ([]ai.ToolCall, error) {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0] == nil || resp.Candidates[0].Content == nil {
		return nil, nil
	}
	var out []ai.ToolCall
	for _, part := range resp.Candidates[0].Content.Parts {
		if part == nil || part.FunctionCall == nil {
			continue
		}
		fc := part.FunctionCall
		args := json.RawMessage("{}")
		if len(fc.Args) > 0 {
			b, err := json.Marshal(fc.Args)
			if err != nil {
				return nil, err
			}
			args = b
		}
		out = append(out, ai.ToolCall{
			ID:               fc.ID,
			Name:             fc.Name,
			Arguments:        args,
			ThoughtSignature: part.ThoughtSignature,
		})
	}
	return out, nil
}

func extractFinishReason(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0] == nil {
		return ""
	}
	return string(resp.Candidates[0].FinishReason)
}

func (p *GoogleProvider) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	if _, hasDeadline := ctx.Deadline(); hasDeadline || p.defaultTimeout <= 0 {
		return ctx, func() {}
	}

	return context.WithTimeout(ctx, p.defaultTimeout)
}

type googleStreamEvent struct {
	delta string
	err   error
	done  bool
}

type googleStream struct {
	events     chan googleStreamEvent
	current    string
	output     string
	err        error
	done       bool
	cancel     context.CancelFunc
	toolCallMu chan struct{} // 1-buffered "mutex" set by producer goroutine
	toolCalls  []ai.ToolCall
	stopReason string
}

func newGoogleStream(stream iter.Seq2[*genai.GenerateContentResponse, error], cancel context.CancelFunc) *googleStream {
	s := &googleStream{
		events:     make(chan googleStreamEvent, 32),
		cancel:     cancel,
		toolCallMu: make(chan struct{}, 1),
	}
	go func() {
		defer close(s.events)
		var collected []ai.ToolCall
		var lastStop string
		for resp, err := range stream {
			if err != nil {
				s.events <- googleStreamEvent{err: err}
				s.publishFinalState(collected, lastStop)
				return
			}

			if calls, cerr := extractFunctionCalls(resp); cerr == nil && len(calls) > 0 {
				collected = append(collected, calls...)
			}
			if reason := extractFinishReason(resp); reason != "" {
				lastStop = reason
			}

			fullText := extractVisibleText(resp)
			if fullText == "" {
				continue
			}

			delta := fullText
			if strings.HasPrefix(fullText, s.output) {
				delta = fullText[len(s.output):]
				s.output = fullText
			} else {
				s.output += delta
			}

			if delta != "" {
				s.events <- googleStreamEvent{delta: delta}
			}
		}
		s.publishFinalState(collected, lastStop)
		s.events <- googleStreamEvent{done: true}
	}()
	return s
}

// publishFinalState records tool calls and stop reason so they're visible to
// the consumer once Next() returns false. The toolCallMu channel acts as a
// 1-slot lock to synchronize the writes with the consumer's read.
func (s *googleStream) publishFinalState(calls []ai.ToolCall, stopReason string) {
	s.toolCallMu <- struct{}{}
	s.toolCalls = calls
	s.stopReason = stopReason
	<-s.toolCallMu
}

func (s *googleStream) Next() bool {
	if s.done || s.err != nil {
		return false
	}

	for ev := range s.events {
		if ev.err != nil {
			s.err = ev.err
			s.done = true
			return false
		}
		if ev.done {
			s.done = true
			return false
		}
		if ev.delta == "" {
			continue
		}
		s.current = ev.delta
		return true
	}

	s.done = true
	return false
}

func (s *googleStream) Content() string {
	return s.current
}

func (s *googleStream) Err() error {
	return s.err
}

func (s *googleStream) Close() error {
	if s.cancel != nil {
		s.cancel()
		s.cancel = nil
	}
	if s.done {
		return nil
	}
	// Drain remaining events to allow producer goroutine to finish.
	for range s.events {
	}
	s.done = true
	return nil
}

func (s *googleStream) ToolCalls() []ai.ToolCall {
	s.toolCallMu <- struct{}{}
	calls := s.toolCalls
	<-s.toolCallMu
	return calls
}

func (s *googleStream) StopReason() string {
	s.toolCallMu <- struct{}{}
	reason := s.stopReason
	<-s.toolCallMu
	return reason
}

// Capabilities reports what the Google provider supports.
func (p *GoogleProvider) Capabilities() ai.ProviderCapabilities {
	return ai.ProviderCapabilities{Streaming: true, Tools: true}
}

// Ensure interface compliance
var _ ai.Provider = (*GoogleProvider)(nil)

func extractVisibleText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0] == nil || resp.Candidates[0].Content == nil {
		return ""
	}

	var sb strings.Builder
	for _, part := range resp.Candidates[0].Content.Parts {
		if part == nil || part.Thought || part.Text == "" {
			continue
		}
		sb.WriteString(part.Text)
	}
	return sb.String()
}
