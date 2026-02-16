package providers

import (
	"context"
	"errors"
	"iter"
	"math"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/config"

	"google.golang.org/genai"
)

type stubGoogleModelsClient struct {
	generateResp *genai.GenerateContentResponse
	generateErr  error
	streamSeq    iter.Seq2[*genai.GenerateContentResponse, error]

	gotModel    string
	gotContents []*genai.Content
	gotConfig   *genai.GenerateContentConfig
}

func (s *stubGoogleModelsClient) GenerateContent(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	s.gotModel = model
	s.gotContents = contents
	s.gotConfig = cfg
	return s.generateResp, s.generateErr
}

func (s *stubGoogleModelsClient) GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) iter.Seq2[*genai.GenerateContentResponse, error] {
	s.gotModel = model
	s.gotContents = contents
	s.gotConfig = cfg
	if s.streamSeq != nil {
		return s.streamSeq
	}
	return func(yield func(*genai.GenerateContentResponse, error) bool) {}
}

func googleTextResponse(text string) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{
			{
				Content: &genai.Content{
					Role: genai.RoleModel,
					Parts: []*genai.Part{
						{Text: text},
					},
				},
			},
		},
	}
}

func TestNewGoogleProvider_RequiresAPIKey(t *testing.T) {
	cfg := config.Default()
	cfg.LLMProvider = "google"
	cfg.Providers.Google.APIKey = ""

	_, err := NewGoogleProvider(ai.ProviderConfig{
		Type:   ai.ProviderGoogle,
		Config: cfg,
	})
	if err == nil {
		t.Fatal("Expected error when Google API key is missing")
	}
}

func TestNewGoogleProvider_DefaultFallbacks(t *testing.T) {
	origNewClient := newGoogleClient
	defer func() {
		newGoogleClient = origNewClient
	}()

	var gotClientCfg *genai.ClientConfig
	newGoogleClient = func(ctx context.Context, cfg *genai.ClientConfig) (*genai.Client, error) {
		gotClientCfg = cfg
		return &genai.Client{}, nil
	}

	cfg := config.Default()
	cfg.LLMProvider = "google"
	cfg.Providers.Google.APIKey = "test-google-key"
	cfg.Providers.Google.Model = ""
	cfg.Providers.Google.Temperature = 0.55
	cfg.Providers.Google.MaxTokens = 2048
	cfg.Providers.Google.APITimeoutSeconds = 0

	provider, err := NewGoogleProvider(ai.ProviderConfig{
		Type:   ai.ProviderGoogle,
		Config: cfg,
	})
	if err != nil {
		t.Fatalf("NewGoogleProvider() error: %v", err)
	}

	googleProvider, ok := provider.(*GoogleProvider)
	if !ok {
		t.Fatalf("Expected *GoogleProvider, got %T", provider)
	}
	if gotClientCfg == nil {
		t.Fatal("Expected Google client config to be captured")
	}
	if gotClientCfg.APIKey != "test-google-key" {
		t.Fatalf("Expected API key to be forwarded, got %q", gotClientCfg.APIKey)
	}
	if gotClientCfg.Backend != genai.BackendGeminiAPI {
		t.Fatalf("Expected BackendGeminiAPI, got %q", gotClientCfg.Backend)
	}
	if googleProvider.defaultModel != googleDefaultModel {
		t.Fatalf("Expected default model %q, got %q", googleDefaultModel, googleProvider.defaultModel)
	}
	if googleProvider.defaultTimeout != 60*time.Second {
		t.Fatalf("Expected default timeout 60s, got %s", googleProvider.defaultTimeout)
	}
	if googleProvider.defaultTemperature != 0.55 {
		t.Fatalf("Expected default temperature 0.55, got %f", googleProvider.defaultTemperature)
	}
	if googleProvider.defaultMaxTokens != 2048 {
		t.Fatalf("Expected default max tokens 2048, got %d", googleProvider.defaultMaxTokens)
	}
}

func TestGoogleProvider_CreateChatCompletion_MapsMessages(t *testing.T) {
	stub := &stubGoogleModelsClient{
		generateResp: googleTextResponse("ok"),
	}
	provider := &GoogleProvider{
		models:             stub,
		defaultModel:       "google-default",
		defaultTemperature: 0.7,
		defaultMaxTokens:   1024,
	}

	temp := 0.2
	maxTokens := 42
	resp, err := provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{
			{Role: "system", Content: "system prompt"},
			{Role: "developer", Content: "developer prompt"},
			{Role: "user", Content: "user prompt"},
			{Role: "assistant", Content: "assistant prompt"},
			{Role: "tool", Content: "unknown role maps to user"},
		},
		Temperature: &temp,
		MaxTokens:   &maxTokens,
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}

	if resp.Content != "ok" {
		t.Fatalf("Expected response content %q, got %q", "ok", resp.Content)
	}
	if resp.Model != "google-default" {
		t.Fatalf("Expected response model %q, got %q", "google-default", resp.Model)
	}
	if stub.gotModel != "google-default" {
		t.Fatalf("Expected default model to be used, got %q", stub.gotModel)
	}
	if len(stub.gotContents) != 3 {
		t.Fatalf("Expected 3 non-system messages, got %d", len(stub.gotContents))
	}

	if stub.gotContents[0].Role != genai.RoleUser {
		t.Fatalf("Expected first content role user, got %q", stub.gotContents[0].Role)
	}
	if stub.gotContents[1].Role != genai.RoleModel {
		t.Fatalf("Expected second content role model, got %q", stub.gotContents[1].Role)
	}
	if stub.gotContents[2].Role != genai.RoleUser {
		t.Fatalf("Expected unknown role to map to user, got %q", stub.gotContents[2].Role)
	}
	if stub.gotConfig == nil || stub.gotConfig.SystemInstruction == nil {
		t.Fatal("Expected system instruction to be set")
	}
	if len(stub.gotConfig.SystemInstruction.Parts) != 1 {
		t.Fatalf("Expected one system instruction part, got %d", len(stub.gotConfig.SystemInstruction.Parts))
	}
	if got := stub.gotConfig.SystemInstruction.Parts[0].Text; got != "system prompt\n\ndeveloper prompt" {
		t.Fatalf("Expected merged system+developer prompt, got %q", got)
	}
	if stub.gotConfig.Temperature == nil {
		t.Fatal("Expected temperature to be set")
	}
	if stub.gotConfig.ThinkingConfig == nil {
		t.Fatal("Expected thinking config to be set")
	}
	if stub.gotConfig.ThinkingConfig.IncludeThoughts {
		t.Fatal("Expected IncludeThoughts=false")
	}
	if stub.gotConfig.ThinkingConfig.ThinkingBudget == nil {
		t.Fatal("Expected ThinkingBudget to be set")
	}
	if *stub.gotConfig.ThinkingConfig.ThinkingBudget != 0 {
		t.Fatalf("Expected ThinkingBudget=0, got %d", *stub.gotConfig.ThinkingConfig.ThinkingBudget)
	}
	if math.Abs(float64(*stub.gotConfig.Temperature)-0.2) > 0.0001 {
		t.Fatalf("Expected temperature override 0.2, got %f", *stub.gotConfig.Temperature)
	}
	if stub.gotConfig.MaxOutputTokens != 42 {
		t.Fatalf("Expected max output tokens 42, got %d", stub.gotConfig.MaxOutputTokens)
	}
}

func TestGoogleProvider_CreateChatCompletion_FiltersThoughtParts(t *testing.T) {
	stub := &stubGoogleModelsClient{
		generateResp: &genai.GenerateContentResponse{
			Candidates: []*genai.Candidate{
				{
					Content: &genai.Content{
						Role: genai.RoleModel,
						Parts: []*genai.Part{
							{Text: "internal", Thought: true},
							{Text: "visible answer"},
						},
					},
				},
			},
		},
	}
	provider := &GoogleProvider{
		models:       stub,
		defaultModel: "google-default",
	}

	resp, err := provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if resp.Content != "visible answer" {
		t.Fatalf("Expected thought parts to be filtered, got %q", resp.Content)
	}
}

func TestGoogleProvider_CreateChatCompletion_NormalizesNonPositiveMaxTokens(t *testing.T) {
	stub := &stubGoogleModelsClient{
		generateResp: googleTextResponse("ok"),
	}
	provider := &GoogleProvider{
		models:             stub,
		defaultModel:       "google-default",
		defaultTemperature: 0.7,
		defaultMaxTokens:   1024,
	}

	zero := 0
	_, err := provider.CreateChatCompletion(context.Background(), ai.ChatRequest{
		Messages:  []ai.Message{{Role: "user", Content: "hello"}},
		MaxTokens: &zero,
	})
	if err != nil {
		t.Fatalf("CreateChatCompletion() error: %v", err)
	}
	if stub.gotConfig == nil {
		t.Fatal("Expected request config to be captured")
	}
	if stub.gotConfig.MaxOutputTokens != 0 {
		t.Fatalf("Expected max output tokens to be unset (0), got %d", stub.gotConfig.MaxOutputTokens)
	}
}

func TestGoogleProvider_CreateChatCompletionStream(t *testing.T) {
	stub := &stubGoogleModelsClient{
		streamSeq: func(yield func(*genai.GenerateContentResponse, error) bool) {
			if !yield(googleTextResponse("Hello"), nil) {
				return
			}
			yield(googleTextResponse(" world"), nil)
		},
	}
	provider := &GoogleProvider{
		models:       stub,
		defaultModel: "google-default",
	}

	stream, err := provider.CreateChatCompletionStream(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "stream"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error: %v", err)
	}
	defer stream.Close()

	var output strings.Builder
	for stream.Next() {
		output.WriteString(stream.Content())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Unexpected stream error: %v", err)
	}
	if output.String() != "Hello world" {
		t.Fatalf("Expected stream output %q, got %q", "Hello world", output.String())
	}
}

func TestGoogleProvider_CreateChatCompletionStream_CumulativeChunks(t *testing.T) {
	stub := &stubGoogleModelsClient{
		streamSeq: func(yield func(*genai.GenerateContentResponse, error) bool) {
			if !yield(googleTextResponse("Hello"), nil) {
				return
			}
			yield(googleTextResponse("Hello world"), nil)
		},
	}
	provider := &GoogleProvider{
		models:       stub,
		defaultModel: "google-default",
	}

	stream, err := provider.CreateChatCompletionStream(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "stream"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error: %v", err)
	}
	defer stream.Close()

	var output strings.Builder
	for stream.Next() {
		output.WriteString(stream.Content())
	}
	if err := stream.Err(); err != nil {
		t.Fatalf("Unexpected stream error: %v", err)
	}
	if output.String() != "Hello world" {
		t.Fatalf("Expected cumulative stream output %q, got %q", "Hello world", output.String())
	}
}

func TestGoogleProvider_CreateChatCompletionStream_Error(t *testing.T) {
	streamErr := errors.New("stream failed")
	stub := &stubGoogleModelsClient{
		streamSeq: func(yield func(*genai.GenerateContentResponse, error) bool) {
			yield(nil, streamErr)
		},
	}
	provider := &GoogleProvider{
		models:       stub,
		defaultModel: "google-default",
	}

	stream, err := provider.CreateChatCompletionStream(context.Background(), ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "stream"}},
	})
	if err != nil {
		t.Fatalf("CreateChatCompletionStream() error: %v", err)
	}
	defer stream.Close()

	if stream.Next() {
		t.Fatal("Expected Next() to return false when stream emits an error")
	}
	if err := stream.Err(); err == nil {
		t.Fatal("Expected stream error to be reported")
	}
}
