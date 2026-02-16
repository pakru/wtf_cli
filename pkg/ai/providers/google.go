package providers

import (
	"context"
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

	return ai.ChatResponse{
		Content: extractVisibleText(resp),
		Model:   model,
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
			contents = append(contents, &genai.Content{
				Role: genai.RoleModel,
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
			})
		case "user":
			contents = append(contents, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
			})
		default:
			contents = append(contents, &genai.Content{
				Role: genai.RoleUser,
				Parts: []*genai.Part{
					{Text: msg.Content},
				},
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

	return model, contents, config, nil
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
	events  chan googleStreamEvent
	current string
	output  string
	err     error
	done    bool
	cancel  context.CancelFunc
}

func newGoogleStream(stream iter.Seq2[*genai.GenerateContentResponse, error], cancel context.CancelFunc) *googleStream {
	s := &googleStream{
		events: make(chan googleStreamEvent, 32),
		cancel: cancel,
	}
	go func() {
		defer close(s.events)
		for resp, err := range stream {
			if err != nil {
				s.events <- googleStreamEvent{err: err}
				return
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
		s.events <- googleStreamEvent{done: true}
	}()
	return s
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
