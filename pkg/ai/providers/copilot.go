package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/auth"

	openai "github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"github.com/openai/openai-go/v3/packages/ssestream"
)

const (
	copilotDefaultModel   = "gpt-4o"
	copilotDefaultTimeout = 30
	copilotTokenURL       = "https://api.github.com/copilot_internal/v2/token"
)

func init() {
	ai.RegisterProvider(ai.ProviderInfo{
		Type:        ai.ProviderCopilot,
		Name:        "GitHub Copilot",
		Description: "Use your GitHub Copilot subscription (requires Copilot Pro/Business/Enterprise)",
		AuthMethod:  "oauth_device",
		RequiresKey: false,
	}, NewCopilotProvider)
}

// CopilotProvider implements the Provider interface using GitHub Copilot's API.
type CopilotProvider struct {
	client             openai.Client
	authManager        *auth.AuthManager
	defaultModel       string
	defaultTemperature float64
	defaultMaxTokens   int
}

// CopilotTokenResponse is the response from GitHub's Copilot token endpoint.
type CopilotTokenResponse struct {
	Token     string `json:"token"`
	ExpiresAt int64  `json:"expires_at"`
	Endpoints struct {
		API string `json:"api"`
	} `json:"endpoints"`
}

// NewCopilotProvider creates a new GitHub Copilot provider from config.
func NewCopilotProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	if cfg.AuthManager == nil {
		return nil, fmt.Errorf("copilot provider requires AuthManager for OAuth credentials")
	}

	creds, err := cfg.AuthManager.Load(string(ai.ProviderCopilot))
	if err != nil {
		return nil, fmt.Errorf("copilot credentials not found - please authenticate first: %w", err)
	}

	if creds.IsExpired() {
		return nil, fmt.Errorf("copilot credentials have expired - please re-authenticate")
	}

	copilotToken, apiEndpoint, err := getCopilotAPIToken(creds.AccessToken)
	if err != nil {
		return nil, fmt.Errorf("failed to get Copilot API token: %w", err)
	}

	providerCfg := cfg.Config.Providers.Copilot

	model := providerCfg.Model
	if model == "" {
		model = copilotDefaultModel
	}

	timeout := providerCfg.APITimeoutSeconds
	if timeout <= 0 {
		timeout = copilotDefaultTimeout
	}

	httpClient := &http.Client{Timeout: time.Duration(timeout) * time.Second}

	opts := []option.RequestOption{
		option.WithAPIKey(copilotToken),
		option.WithBaseURL(apiEndpoint),
		option.WithHTTPClient(httpClient),
		option.WithHeader("Editor-Version", "wtf_cli/1.0"),
		option.WithHeader("Copilot-Integration-Id", "vscode-chat"),
	}

	client := openai.NewClient(opts...)

	return &CopilotProvider{
		client:             client,
		authManager:        cfg.AuthManager,
		defaultModel:       model,
		defaultTemperature: providerCfg.Temperature,
		defaultMaxTokens:   providerCfg.MaxTokens,
	}, nil
}

func getCopilotAPIToken(githubToken string) (string, string, error) {
	req, err := http.NewRequest("GET", copilotTokenURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("failed to create token request: %w", err)
	}

	req.Header.Set("Authorization", "token "+githubToken)
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", "", fmt.Errorf("failed to request Copilot token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", fmt.Errorf("failed to read token response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("Copilot token request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var tokenResp CopilotTokenResponse
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", "", fmt.Errorf("failed to parse token response: %w", err)
	}

	apiEndpoint := tokenResp.Endpoints.API
	if apiEndpoint == "" {
		apiEndpoint = "https://api.githubcopilot.com"
	}

	return tokenResp.Token, apiEndpoint, nil
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *CopilotProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
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
func (p *CopilotProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	params, err := p.buildChatParams(req)
	if err != nil {
		return nil, err
	}

	stream := p.client.Chat.Completions.NewStreaming(ctx, params)
	if err := stream.Err(); err != nil {
		return nil, err
	}

	return &copilotStream{stream: stream}, nil
}

func (p *CopilotProvider) buildChatParams(req ai.ChatRequest) (openai.ChatCompletionNewParams, error) {
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

type copilotStream struct {
	stream *ssestream.Stream[openai.ChatCompletionChunk]
}

func (s *copilotStream) Next() bool {
	return s.stream.Next()
}

func (s *copilotStream) Content() string {
	chunk := s.stream.Current()
	if len(chunk.Choices) == 0 {
		return ""
	}
	return chunk.Choices[0].Delta.Content
}

func (s *copilotStream) Err() error {
	return s.stream.Err()
}

func (s *copilotStream) Close() error {
	return s.stream.Close()
}

// Ensure interface compliance
var _ ai.Provider = (*CopilotProvider)(nil)
