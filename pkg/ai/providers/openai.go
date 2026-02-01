package providers

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"wtf_cli/pkg/ai"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

const (
	openAIDefaultAPIURL  = "https://api.openai.com/v1"
	openAIDefaultModel   = "gpt-4o"
	openAIDefaultTimeout = 30
)

func init() {
	ai.RegisterProvider(ai.ProviderInfo{
		Type:        ai.ProviderOpenAI,
		Name:        "OpenAI",
		Description: "Direct OpenAI API access (API key or ChatGPT Plus/Pro OAuth)",
		AuthMethod:  "api_key", // Can also use oauth_pkce
		RequiresKey: true,
	}, NewOpenAIProvider)
}

// OpenAIProvider implements the Provider interface using the OpenAI API directly.
type OpenAIProvider struct {
	client             openai.Client
	defaultModel       string
	defaultTemperature float64
	defaultMaxTokens   int
}

// NewOpenAIProvider creates a new OpenAI provider from config.
func NewOpenAIProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	providerCfg := cfg.Config.Providers.OpenAI

	apiKey := providerCfg.APIKey
	if apiKey == "" && cfg.AuthManager != nil {
		creds, err := cfg.AuthManager.Load(string(ai.ProviderOpenAI))
		if err == nil && creds != nil {
			apiKey = creds.AccessToken
		}
	}

	if strings.TrimSpace(apiKey) == "" {
		return nil, fmt.Errorf("openai api_key is required (set in config or authenticate via OAuth)")
	}

	apiURL := providerCfg.APIURL
	if apiURL == "" {
		apiURL = openAIDefaultAPIURL
	}

	model := providerCfg.Model
	if model == "" {
		model = openAIDefaultModel
	}

	timeout := providerCfg.APITimeoutSeconds
	if timeout <= 0 {
		timeout = openAIDefaultTimeout
	}

	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	opts := []option.RequestOption{
		option.WithAPIKey(apiKey),
		option.WithBaseURL(apiURL),
		option.WithHTTPClient(httpClient),
	}

	client := openai.NewClient(opts...)

	return &OpenAIProvider{
		client:             client,
		defaultModel:       model,
		defaultTemperature: providerCfg.Temperature,
		defaultMaxTokens:   providerCfg.MaxTokens,
	}, nil
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *OpenAIProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	params, err := p.buildChatParams(req)
	if err != nil {
		return ai.ChatResponse{}, err
	}

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
func (p *OpenAIProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	params, err := p.buildChatParams(req)
	if err != nil {
		return nil, err
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	if err := stream.Err(); err != nil {
		return nil, err
	}

	return &openAIStream{stream: stream}, nil
}

func (p *OpenAIProvider) buildChatParams(req ai.ChatRequest) (openai.ChatCompletionNewParams, error) {
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
	if temperature > 0 {
		params.Temperature = openai.Float(temperature)
	}

	maxTokens := p.defaultMaxTokens
	if req.MaxTokens != nil {
		maxTokens = *req.MaxTokens
	}
	if maxTokens > 0 {
		params.MaxTokens = openai.Int(int64(maxTokens))
	}

	return params, nil
}

type openAIStream struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
}

func (s *openAIStream) Next() bool {
	return s.stream.Next()
}

func (s *openAIStream) Content() string {
	chunk := s.stream.Current()
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

func (s *openAIStream) Err() error {
	return s.stream.Err()
}

func (s *openAIStream) Close() error {
	return s.stream.Close()
}

// Ensure interface compliance
var _ ai.Provider = (*OpenAIProvider)(nil)
