package providers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"wtf_cli/pkg/ai"

	copilot "github.com/github/copilot-sdk/go"
)

const (
	copilotDefaultModel   = "gpt-4o"
	copilotDefaultTimeout = 30
)

func init() {
	ai.RegisterProvider(ai.ProviderInfo{
		Type:        ai.ProviderCopilot,
		Name:        "GitHub Copilot",
		Description: "Use GitHub Copilot via the official Copilot SDK (requires Copilot CLI authentication)",
		AuthMethod:  "copilot_cli",
		RequiresKey: false,
	}, NewCopilotProvider)
}

type copilotClient interface {
	Start(context.Context) error
	Stop() error
	GetAuthStatus(context.Context) (*copilot.GetAuthStatusResponse, error)
	CreateSession(context.Context, *copilot.SessionConfig) (copilotSession, error)
}

type copilotSession interface {
	Send(context.Context, copilot.MessageOptions) (string, error)
	SendAndWait(context.Context, copilot.MessageOptions) (*copilot.SessionEvent, error)
	On(handler copilot.SessionEventHandler) func()
	Abort(context.Context) error
	Destroy() error
}

type sdkCopilotClient struct {
	client *copilot.Client
}

func (c *sdkCopilotClient) Start(ctx context.Context) error {
	return c.client.Start(ctx)
}

func (c *sdkCopilotClient) Stop() error {
	return c.client.Stop()
}

func (c *sdkCopilotClient) GetAuthStatus(ctx context.Context) (*copilot.GetAuthStatusResponse, error) {
	return c.client.GetAuthStatus(ctx)
}

func (c *sdkCopilotClient) CreateSession(ctx context.Context, config *copilot.SessionConfig) (copilotSession, error) {
	session, err := c.client.CreateSession(ctx, config)
	if err != nil {
		return nil, err
	}
	return &sdkCopilotSession{session: session}, nil
}

type sdkCopilotSession struct {
	session *copilot.Session
}

func (s *sdkCopilotSession) Send(ctx context.Context, options copilot.MessageOptions) (string, error) {
	return s.session.Send(ctx, options)
}

func (s *sdkCopilotSession) SendAndWait(ctx context.Context, options copilot.MessageOptions) (*copilot.SessionEvent, error) {
	return s.session.SendAndWait(ctx, options)
}

func (s *sdkCopilotSession) On(handler copilot.SessionEventHandler) func() {
	return s.session.On(handler)
}

func (s *sdkCopilotSession) Abort(ctx context.Context) error {
	return s.session.Abort(ctx)
}

func (s *sdkCopilotSession) Destroy() error {
	return s.session.Destroy()
}

var newCopilotClient = func() copilotClient {
	return &sdkCopilotClient{client: copilot.NewClient(nil)}
}

// CopilotProvider implements the Provider interface using the Copilot SDK.
type CopilotProvider struct {
	client             copilotClient
	defaultModel       string
	defaultTemperature float64
	defaultMaxTokens   int
	timeout            time.Duration
}

// NewCopilotProvider creates a new GitHub Copilot provider from config.
func NewCopilotProvider(cfg ai.ProviderConfig) (ai.Provider, error) {
	providerCfg := cfg.Config.Providers.Copilot

	model := strings.TrimSpace(providerCfg.Model)
	if model == "" {
		model = copilotDefaultModel
	}

	timeout := providerCfg.APITimeoutSeconds
	if timeout <= 0 {
		timeout = copilotDefaultTimeout
	}

	slog.Debug("copilot_provider_ready",
		"model", model,
		"timeout_seconds", timeout,
	)
	return &CopilotProvider{
		client:             newCopilotClient(),
		defaultModel:       model,
		defaultTemperature: providerCfg.Temperature,
		defaultMaxTokens:   providerCfg.MaxTokens,
		timeout:            time.Duration(timeout) * time.Second,
	}, nil
}

// CreateChatCompletion sends a non-streaming chat completion request.
func (p *CopilotProvider) CreateChatCompletion(ctx context.Context, req ai.ChatRequest) (ai.ChatResponse, error) {
	ctx = normalizeCopilotContext(ctx)
	if err := ctx.Err(); err != nil {
		return ai.ChatResponse{}, err
	}

	systemMsg, prompt, err := buildCopilotPrompt(req)
	if err != nil {
		return ai.ChatResponse{}, err
	}

	model := pickCopilotModel(req.Model, p.defaultModel)
	requestTimeout := selectCopilotTimeout(ctx, p.timeout)

	slog.Debug("copilot_chat_request",
		"model", model,
		"message_count", len(req.Messages),
		"has_temperature", req.Temperature != nil,
		"has_max_tokens", req.MaxTokens != nil,
	)
	logCopilotUnsupportedOptions(req, p.defaultTemperature, p.defaultMaxTokens)

	if err := p.client.Start(ctx); err != nil {
		return ai.ChatResponse{}, fmt.Errorf("copilot client start: %w", err)
	}
	defer stopCopilotClient(p.client)

	if err := ensureCopilotAuthenticated(ctx, p.client); err != nil {
		return ai.ChatResponse{}, err
	}

	slog.Debug("copilot_session_create_start", "model", model, "streaming", false)
	session, err := p.client.CreateSession(ctx, &copilot.SessionConfig{
		Model:         model,
		Streaming:     false,
		SystemMessage: copilotSystemMessage(systemMsg),
	})
	if err != nil {
		return ai.ChatResponse{}, fmt.Errorf("copilot session create: %w", err)
	}
	slog.Debug("copilot_session_create_done", "model", model)
	defer session.Destroy()

	sendCtx, cancel := context.WithTimeout(ctx, requestTimeout)
	defer cancel()
	abortDone := watchCopilotContext(sendCtx, session)
	defer close(abortDone)

	slog.Debug("copilot_session_send_start", "prompt_chars", len(prompt))
	resp, err := session.SendAndWait(sendCtx, copilot.MessageOptions{Prompt: prompt})
	if err != nil {
		return ai.ChatResponse{}, err
	}
	slog.Debug("copilot_session_send_done")

	content := ""
	if resp != nil {
		if data, ok := resp.Data.(*copilot.AssistantMessageData); ok {
			content = data.Content
		}
	}

	return ai.ChatResponse{
		Content: content,
		Model:   model,
	}, nil
}

// CreateChatCompletionStream sends a streaming chat completion request.
func (p *CopilotProvider) CreateChatCompletionStream(ctx context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	ctx = normalizeCopilotContext(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	systemMsg, prompt, err := buildCopilotPrompt(req)
	if err != nil {
		return nil, err
	}

	model := pickCopilotModel(req.Model, p.defaultModel)

	slog.Debug("copilot_chat_stream_request",
		"model", model,
		"message_count", len(req.Messages),
		"has_temperature", req.Temperature != nil,
		"has_max_tokens", req.MaxTokens != nil,
	)
	logCopilotUnsupportedOptions(req, p.defaultTemperature, p.defaultMaxTokens)

	if err := p.client.Start(ctx); err != nil {
		return nil, fmt.Errorf("copilot client start: %w", err)
	}

	if err := ensureCopilotAuthenticated(ctx, p.client); err != nil {
		stopCopilotClient(p.client)
		return nil, err
	}

	slog.Debug("copilot_session_create_start", "model", model, "streaming", true)
	session, err := p.client.CreateSession(ctx, &copilot.SessionConfig{
		Model:         model,
		Streaming:     true,
		SystemMessage: copilotSystemMessage(systemMsg),
	})
	if err != nil {
		stopCopilotClient(p.client)
		return nil, fmt.Errorf("copilot session create: %w", err)
	}
	slog.Debug("copilot_session_create_done", "model", model)

	stream := newCopilotStream(ctx, p.client, session)
	stream.start(prompt)
	return stream, nil
}

func pickCopilotModel(requested, fallback string) string {
	model := strings.TrimSpace(requested)
	if model == "" {
		model = strings.TrimSpace(fallback)
	}
	if model == "" {
		return copilotDefaultModel
	}
	return model
}

func selectCopilotTimeout(ctx context.Context, fallback time.Duration) time.Duration {
	if ctx == nil {
		return fallback
	}
	if deadline, ok := ctx.Deadline(); ok {
		remaining := time.Until(deadline)
		if remaining > 0 && remaining < fallback {
			return remaining
		}
	}
	return fallback
}

func normalizeCopilotContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func stopCopilotClient(client copilotClient) {
	if client == nil {
		return
	}
	if err := client.Stop(); err != nil {
		slog.Debug("copilot_client_stop_error", "error", err)
	}
}

func ensureCopilotAuthenticated(ctx context.Context, client copilotClient) error {
	status, err := client.GetAuthStatus(ctx)
	if err != nil {
		return fmt.Errorf("copilot auth status: %w", err)
	}
	if status != nil && status.IsAuthenticated {
		slog.Debug("copilot_auth_status_ok")
		return nil
	}
	msg := "Copilot CLI is not authenticated"
	if status != nil && status.StatusMessage != nil && strings.TrimSpace(*status.StatusMessage) != "" {
		msg = strings.TrimSpace(*status.StatusMessage)
	}
	slog.Debug("copilot_auth_status_not_authenticated", "message", msg)
	return errors.New(msg)
}

func copilotSystemMessage(content string) *copilot.SystemMessageConfig {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return nil
	}
	return &copilot.SystemMessageConfig{Mode: "append", Content: trimmed}
}

func buildCopilotPrompt(req ai.ChatRequest) (string, string, error) {
	if len(req.Messages) == 0 {
		return "", "", fmt.Errorf("messages are required")
	}

	var systemParts []string
	var promptParts []string

	for _, msg := range req.Messages {
		role := strings.ToLower(strings.TrimSpace(msg.Role))
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		switch role {
		case "system", "developer":
			label := strings.ToUpper(role)
			systemParts = append(systemParts, fmt.Sprintf("%s:\n%s", label, content))
		default:
			label := roleLabel(role)
			promptParts = append(promptParts, fmt.Sprintf("%s: %s", label, content))
		}
	}

	if len(promptParts) == 0 {
		return "", "", fmt.Errorf("messages are required")
	}

	systemMsg := strings.Join(systemParts, "\n\n")
	prompt := strings.Join(promptParts, "\n\n")
	return systemMsg, prompt, nil
}

func roleLabel(role string) string {
	if role == "" {
		return "User"
	}
	return strings.ToUpper(role[:1]) + role[1:]
}

func logCopilotUnsupportedOptions(req ai.ChatRequest, defaultTemp float64, defaultMaxTokens int) {
	if req.Temperature != nil {
		slog.Debug("copilot_option_ignored", "option", "temperature", "value", *req.Temperature)
	} else if defaultTemp > 0 {
		slog.Debug("copilot_option_ignored", "option", "temperature", "value", defaultTemp)
	}
	if req.MaxTokens != nil {
		slog.Debug("copilot_option_ignored", "option", "max_tokens", "value", *req.MaxTokens)
	} else if defaultMaxTokens > 0 {
		slog.Debug("copilot_option_ignored", "option", "max_tokens", "value", defaultMaxTokens)
	}
}

type copilotStreamEvent struct {
	delta string
	err   error
	done  bool
}

type copilotStream struct {
	ctx          context.Context
	events       chan copilotStreamEvent
	current      string
	err          error
	cleanupOnce  sync.Once
	cleanup      func()
	unsubscribe  func()
	session      copilotSession
	sawDelta     bool
	eventCount   int
	deltaCount   int
	closeEvents  func()
	closeEventMu sync.Once
}

func newCopilotStream(ctx context.Context, client copilotClient, session copilotSession) *copilotStream {
	events := make(chan copilotStreamEvent, 32)
	stream := &copilotStream{
		ctx:    normalizeCopilotContext(ctx),
		events: events,
		cleanup: func() {
			if session != nil {
				_ = session.Destroy()
			}
			stopCopilotClient(client)
		},
		session: session,
	}

	stream.closeEvents = func() {
		stream.closeEventMu.Do(func() {
			close(events)
		})
	}

	stream.unsubscribe = session.On(func(event copilot.SessionEvent) {
		stream.handleEvent(event)
	})

	if done := stream.ctx.Done(); done != nil {
		go func() {
			<-done
			slog.Debug("copilot_session_abort", "reason", stream.ctx.Err())
			abortCopilotSession(session)
			stream.sendEvent(copilotStreamEvent{err: stream.ctx.Err(), done: true})
			stream.closeEvents()
		}()
	}

	return stream
}

func (s *copilotStream) start(prompt string) {
	go func() {
		slog.Debug("copilot_session_send_start", "prompt_chars", len(prompt))
		_, err := s.sessionSend(prompt)
		if err != nil {
			s.sendEvent(copilotStreamEvent{err: err, done: true})
			s.closeEvents()
			return
		}
		slog.Debug("copilot_session_send_done")
	}()
}

func (s *copilotStream) sessionSend(prompt string) (string, error) {
	if s == nil {
		return "", fmt.Errorf("stream not initialized")
	}
	return s.session.Send(s.ctx, copilot.MessageOptions{Prompt: prompt})
}

func (s *copilotStream) handleEvent(event copilot.SessionEvent) {
	s.eventCount++
	switch event.Type {
	case copilot.SessionEventTypeAssistantMessageDelta:
		if data, ok := event.Data.(*copilot.AssistantMessageDeltaData); ok && data.DeltaContent != "" {
			s.sawDelta = true
			s.deltaCount++
			s.sendEvent(copilotStreamEvent{delta: data.DeltaContent})
		}
	case copilot.SessionEventTypeAssistantMessage:
		if data, ok := event.Data.(*copilot.AssistantMessageData); ok && !s.sawDelta && data.Content != "" {
			s.sendEvent(copilotStreamEvent{delta: data.Content})
		}
	case copilot.SessionEventTypeSessionError:
		errMsg := "copilot session error"
		if data, ok := event.Data.(*copilot.SessionErrorData); ok && strings.TrimSpace(data.Message) != "" {
			errMsg = strings.TrimSpace(data.Message)
		}
		slog.Debug("copilot_session_error", "message", errMsg)
		s.sendEvent(copilotStreamEvent{err: errors.New(errMsg), done: true})
		s.closeEvents()
	case copilot.SessionEventTypeSessionIdle:
		slog.Debug("copilot_session_idle", "events", s.eventCount, "deltas", s.deltaCount)
		s.sendEvent(copilotStreamEvent{done: true})
		s.closeEvents()
	}
}

func (s *copilotStream) sendEvent(evt copilotStreamEvent) {
	defer func() {
		if recover() != nil {
			return
		}
	}()
	s.events <- evt
}

func (s *copilotStream) Next() bool {
	evt, ok := <-s.events
	if !ok {
		return false
	}
	if evt.err != nil {
		s.err = evt.err
		return false
	}
	if evt.done {
		return false
	}
	s.current = evt.delta
	return true
}

func (s *copilotStream) Content() string {
	return s.current
}

func (s *copilotStream) Err() error {
	return s.err
}

func (s *copilotStream) Close() error {
	s.cleanupOnce.Do(func() {
		slog.Debug("copilot_session_close", "events", s.eventCount, "deltas", s.deltaCount)
		if s.unsubscribe != nil {
			s.unsubscribe()
		}
		if s.cleanup != nil {
			s.cleanup()
		}
		s.closeEvents()
	})
	return nil
}

func watchCopilotContext(ctx context.Context, session copilotSession) chan struct{} {
	done := make(chan struct{})
	if ctx == nil || session == nil {
		return done
	}
	go func() {
		select {
		case <-ctx.Done():
			abortCopilotSession(session)
		case <-done:
		}
	}()
	return done
}

func abortCopilotSession(session copilotSession) {
	if session == nil {
		return
	}
	abortCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = session.Abort(abortCtx)
}

// Ensure interface compliance
var _ ai.Provider = (*CopilotProvider)(nil)
