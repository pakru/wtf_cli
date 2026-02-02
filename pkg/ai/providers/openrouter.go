package providers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/auth"
	"wtf_cli/pkg/config"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

func init() {
	ai.RegisterProvider(ai.ProviderInfo{
		Type:        ai.ProviderOpenRouter,
		Name:        "OpenRouter",
		Description: "Access 400+ LLM models through OpenRouter API",
		AuthMethod:  "api_key",
		RequiresKey: true,
	}, NewOpenRouterProvider)
}

// OpenRouterProvider implements the Provider interface using the OpenRouter API.
type OpenRouterProvider struct {
	client             openai.Client
	defaultModel       string
	defaultTemperature float64
	defaultMaxTokens   int
}

// NewOpenRouterProvider creates a new OpenRouter provider from config.
func NewOpenRouterProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	orCfg := cfg.Config.OpenRouter
	httpClient := &http.Client{Timeout: time.Duration(orCfg.APITimeoutSeconds) * time.Second}
	return newOpenRouterProviderWithHTTPClient(orCfg, httpClient)
}

// NewOpenRouterProviderFromConfig creates a provider directly from OpenRouterConfig.
func NewOpenRouterProviderFromConfig(cfg config.OpenRouterConfig) (*OpenRouterProvider, error) {
	httpClient := &http.Client{Timeout: time.Duration(cfg.APITimeoutSeconds) * time.Second}
	return newOpenRouterProviderWithHTTPClient(cfg, httpClient)
}

func newOpenRouterProviderWithHTTPClient(cfg config.OpenRouterConfig, httpClient *http.Client) (*OpenRouterProvider, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		slog.Debug("openrouter_provider_missing_key")
		return nil, fmt.Errorf("openrouter api_key is required")
	}
	if strings.TrimSpace(cfg.APIURL) == "" {
		return nil, fmt.Errorf("openrouter api_url is required")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, fmt.Errorf("openrouter model is required")
	}
	if cfg.APITimeoutSeconds <= 0 {
		return nil, fmt.Errorf("openrouter api_timeout_seconds must be positive")
	}

	opts := []option.RequestOption{
		option.WithAPIKey(cfg.APIKey),
		option.WithBaseURL(cfg.APIURL),
	}

	if strings.TrimSpace(cfg.HTTPReferer) != "" {
		opts = append(opts, option.WithHeader("HTTP-Referer", cfg.HTTPReferer))
	}
	if strings.TrimSpace(cfg.XTitle) != "" {
		opts = append(opts, option.WithHeader("X-Title", cfg.XTitle))
	}

	if httpClient == nil {
		httpClient = &http.Client{Timeout: time.Duration(cfg.APITimeoutSeconds) * time.Second}
	}
	opts = append(opts, option.WithHTTPClient(httpClient))

	client := openai.NewClient(opts...)

	slog.Debug("openrouter_provider_ready",
		"api_url", cfg.APIURL,
		"model", cfg.Model,
		"timeout_seconds", cfg.APITimeoutSeconds,
	)
	return &OpenRouterProvider{
		client:             client,
		defaultModel:       cfg.Model,
		defaultTemperature: cfg.Temperature,
		defaultMaxTokens:   cfg.MaxTokens,
	}, nil
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *OpenRouterProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	params, err := p.buildChatParams(req)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	slog.Debug("openrouter_chat_request",
		"model", string(params.Model),
		"message_count", len(req.Messages),
		"has_temperature", req.Temperature != nil,
		"has_max_tokens", req.MaxTokens != nil,
	)
	resp, err := p.client.Chat.Completions.New(ctx, params)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	content := ""
	if len(resp.Choices) > 0 {
		content = resp.Choices[0].Message.Content
	}

	return ai.ChatResponse{
		Content: content,
		Model:   resp.Model,
	}, nil
}

// CreateChatCompletionStream sends a streaming chat completion request.
func (p *OpenRouterProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	params, err := p.buildChatParams(req)
	if err != nil {
		return nil, err
	}

	slog.Debug("openrouter_chat_stream_request",
		"model", string(params.Model),
		"message_count", len(req.Messages),
		"has_temperature", req.Temperature != nil,
		"has_max_tokens", req.MaxTokens != nil,
	)
	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	if err := stream.Err(); err != nil {
		return nil, err
	}

	return &openRouterStream{stream: stream}, nil
}

func (p *OpenRouterProvider) buildChatParams(req ai.ChatRequest) (openai.ChatCompletionNewParams, error) {
	model := strings.TrimSpace(req.Model)
	if model == "" {
		model = p.defaultModel
	}
	if strings.TrimSpace(model) == "" {
		return openai.ChatCompletionNewParams{}, fmt.Errorf("model is required")
	}
	if len(req.Messages) == 0 {
		return openai.ChatCompletionNewParams{}, fmt.Errorf("messages are required")
	}

	messages := make([]openai.ChatCompletionMessageParamUnion, 0, len(req.Messages))
	for _, msg := range req.Messages {
		param, err := toChatMessageParam(msg)
		if err != nil {
			return openai.ChatCompletionNewParams{}, err
		}
		messages = append(messages, param)
	}

	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(model),
		Messages: messages,
	}

	temperature := p.defaultTemperature
	if req.Temperature != nil {
		temperature = *req.Temperature
	}
	params.Temperature = openai.Float(temperature)

	maxTokens := p.defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if maxTokens > 0 {
		params.MaxTokens = openai.Int(int64(maxTokens))
	}

	return params, nil
}

func toChatMessageParam(msg ai.Message) (openai.ChatCompletionMessageParamUnion, error) {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	switch role {
	case "system":
		return openai.SystemMessage(msg.Content), nil
	case "user":
		return openai.UserMessage(msg.Content), nil
	case "assistant":
		return openai.AssistantMessage(msg.Content), nil
	case "developer":
		return openai.DeveloperMessage(msg.Content), nil
	default:
		return openai.ChatCompletionMessageParamUnion{}, fmt.Errorf("unsupported role: %s", msg.Role)
	}
}

type openRouterStream struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
}

func (s *openRouterStream) Next() bool {
	return s.stream.Next()
}

func (s *openRouterStream) Content() string {
	chunk := s.stream.Current()
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

func (s *openRouterStream) Err() error {
	return s.stream.Err()
}

func (s *openRouterStream) Close() error {
	return s.stream.Close()
}

// Ensure interface compliance
var _ ai.Provider = (*OpenRouterProvider)(nil)

// Blank import to ensure auth package is available
var _ = auth.DefaultAuthPath
