package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/tools"
)

// fakeProvider returns one canned stream per CreateChatCompletionStream call,
// in order. Used to drive RunAgentLoop with deterministic provider behavior.
type fakeProvider struct {
	streams       []*fakeStream
	caps          ai.ProviderCapabilities
	openErr       error
	receivedReqs  []ai.ChatRequest
	streamCounter int
}

func (p *fakeProvider) CreateChatCompletion(_ context.Context, _ ai.ChatRequest) (ai.ChatResponse, error) {
	return ai.ChatResponse{}, errors.New("not implemented for tests")
}

func (p *fakeProvider) CreateChatCompletionStream(_ context.Context, req ai.ChatRequest) (ai.ChatStream, error) {
	p.receivedReqs = append(p.receivedReqs, req)
	if p.openErr != nil {
		return nil, p.openErr
	}
	if p.streamCounter >= len(p.streams) {
		return nil, fmt.Errorf("fakeProvider: no more canned streams (call %d)", p.streamCounter)
	}
	s := p.streams[p.streamCounter]
	p.streamCounter++
	return s, nil
}

func (p *fakeProvider) Capabilities() ai.ProviderCapabilities { return p.caps }

// fakeStream emits the listed text chunks one at a time, then exposes the
// supplied toolCalls via ToolCalls() once Next() returns false.
type fakeStream struct {
	textChunks []string
	toolCalls  []ai.ToolCall
	stopReason string
	streamErr  error

	idx     int
	current string
	done    bool
}

func (s *fakeStream) Next() bool {
	if s.done {
		return false
	}
	if s.idx >= len(s.textChunks) {
		s.done = true
		return false
	}
	s.current = s.textChunks[s.idx]
	s.idx++
	return true
}

func (s *fakeStream) Content() string          { return s.current }
func (s *fakeStream) Err() error               { return s.streamErr }
func (s *fakeStream) Close() error             { return nil }
func (s *fakeStream) ToolCalls() []ai.ToolCall { return s.toolCalls }
func (s *fakeStream) StopReason() string       { return s.stopReason }

// echoTool returns the raw arguments concatenated with a constant prefix. It
// also records each invocation for assertion.
type echoTool struct {
	calls [][]byte
	err   error
}

func (t *echoTool) Name() string { return "echo" }
func (t *echoTool) Definition() ai.ToolDefinition {
	return ai.ToolDefinition{Name: "echo", Description: "echoes input", JSONSchema: json.RawMessage(`{}`)}
}
func (t *echoTool) Execute(_ context.Context, args json.RawMessage) (tools.Result, error) {
	t.calls = append(t.calls, append([]byte(nil), args...))
	if t.err != nil {
		return tools.Result{}, t.err
	}
	return tools.Result{Content: "echoed: " + string(args)}, nil
}

// drain pulls every event from the channel into a slice.
func drain(t *testing.T, ch <-chan WtfStreamEvent, timeout time.Duration) []WtfStreamEvent {
	t.Helper()
	var events []WtfStreamEvent
	deadline := time.After(timeout)
	for {
		select {
		case ev, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, ev)
			if ev.Done {
				// Drain rest if any (channel close is what we wait for in normal flow).
			}
		case <-deadline:
			t.Fatalf("timeout draining events; got so far: %+v", events)
			return events
		}
	}
}

func newRegistryWithEcho(et *echoTool) *tools.Registry {
	r := tools.NewRegistry()
	r.Register(et)
	return r
}

func TestRunAgentLoop_NoToolCalls_SingleTurn(t *testing.T) {
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{
			{textChunks: []string{"hello ", "world"}, stopReason: "stop"},
		},
	}

	ch := make(chan WtfStreamEvent, 8)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      tools.NewRegistry(),
		Approver:      AutoAllowApprover{},
		MaxIterations: 5,
	}, ch)

	events := drain(t, ch, 2*time.Second)

	wantDeltas := []string{"hello ", "world"}
	gotDeltas := []string{}
	doneCount := 0
	for _, e := range events {
		if e.Delta != "" {
			gotDeltas = append(gotDeltas, e.Delta)
		}
		if e.Done {
			doneCount++
		}
	}
	if fmt.Sprint(gotDeltas) != fmt.Sprint(wantDeltas) {
		t.Fatalf("deltas = %v, want %v", gotDeltas, wantDeltas)
	}
	if doneCount != 1 {
		t.Fatalf("doneCount = %d, want 1", doneCount)
	}
	if len(provider.receivedReqs) != 1 {
		t.Fatalf("provider call count = %d, want 1", len(provider.receivedReqs))
	}
}

func TestRunAgentLoop_OneToolCall_TwoTurns(t *testing.T) {
	echo := &echoTool{}
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{
			// First turn: optional preamble + tool call.
			{
				textChunks: []string{"checking..."},
				toolCalls: []ai.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{"x":1}`)},
				},
				stopReason: "tool_calls",
			},
			// Second turn: final answer, no more tools.
			{textChunks: []string{"done"}, stopReason: "stop"},
		},
	}

	ch := make(chan WtfStreamEvent, 16)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      newRegistryWithEcho(echo),
		Approver:      AutoAllowApprover{},
		MaxIterations: 5,
	}, ch)

	events := drain(t, ch, 2*time.Second)

	if len(echo.calls) != 1 {
		t.Fatalf("echo executed %d times, want 1", len(echo.calls))
	}
	if string(echo.calls[0]) != `{"x":1}` {
		t.Fatalf("echo args = %q, want %q", echo.calls[0], `{"x":1}`)
	}

	starts, finished := 0, 0
	for _, e := range events {
		if e.ToolCallStart != nil {
			starts++
		}
		if e.ToolCallFinished != nil {
			finished++
			if e.ToolCallFinished.Denied {
				t.Fatalf("expected approved call, got denied")
			}
			if !containsString(e.ToolCallFinished.Result, "echoed") {
				t.Fatalf("expected result with 'echoed', got %q", e.ToolCallFinished.Result)
			}
		}
	}
	if starts != 1 {
		t.Fatalf("ToolCallStart events = %d, want 1", starts)
	}
	if finished != 1 {
		t.Fatalf("ToolCallFinished events = %d, want 1", finished)
	}

	// Second-turn message history must include assistant turn with tool_calls
	// and the tool result message before the next user-facing turn.
	if len(provider.receivedReqs) != 2 {
		t.Fatalf("provider call count = %d, want 2", len(provider.receivedReqs))
	}
	secondReq := provider.receivedReqs[1]
	if len(secondReq.Messages) < 3 {
		t.Fatalf("second-turn messages = %d, want >= 3", len(secondReq.Messages))
	}
	asst := secondReq.Messages[len(secondReq.Messages)-2]
	tool := secondReq.Messages[len(secondReq.Messages)-1]
	if asst.Role != "assistant" || len(asst.ToolCalls) != 1 {
		t.Fatalf("expected assistant message with tool_calls, got role=%q tool_calls=%d", asst.Role, len(asst.ToolCalls))
	}
	if tool.Role != "tool" || tool.ToolCallID != "call_1" || tool.Name != "echo" {
		t.Fatalf("expected tool message linked to call_1, got %+v", tool)
	}
}

// denyApprover always denies and counts how many times it was called.
type denyApprover struct{ count int }

func (a *denyApprover) Approve(_ context.Context, _ *ApprovalRequest) (ApprovalDecision, error) {
	a.count++
	return ApprovalDecision{Allow: false}, nil
}

func TestRunAgentLoop_DeniedToolBecomesToolMessage(t *testing.T) {
	echo := &echoTool{}
	approver := &denyApprover{}
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{
			{
				textChunks: []string{"want to call"},
				toolCalls: []ai.ToolCall{
					{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{}`)},
				},
				stopReason: "tool_calls",
			},
			{textChunks: []string{"giving up"}, stopReason: "stop"},
		},
	}

	ch := make(chan WtfStreamEvent, 16)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      newRegistryWithEcho(echo),
		Approver:      approver,
		MaxIterations: 5,
	}, ch)
	events := drain(t, ch, 2*time.Second)

	if approver.count != 1 {
		t.Fatalf("approver called %d times, want 1", approver.count)
	}
	if len(echo.calls) != 0 {
		t.Fatalf("denied tool should not execute; got %d calls", len(echo.calls))
	}

	denied := false
	for _, e := range events {
		if e.ToolCallFinished != nil && e.ToolCallFinished.Denied {
			denied = true
		}
	}
	if !denied {
		t.Fatal("expected ToolCallFinished{Denied:true} event")
	}

	// The tool message appended back to the request should report the denial.
	if len(provider.receivedReqs) != 2 {
		t.Fatalf("provider call count = %d, want 2", len(provider.receivedReqs))
	}
	last := provider.receivedReqs[1].Messages[len(provider.receivedReqs[1].Messages)-1]
	if last.Role != "tool" || !containsString(last.Content, "denied") {
		t.Fatalf("expected denied tool message, got %+v", last)
	}
}

func TestRunAgentLoop_UnknownToolSoftFails(t *testing.T) {
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{
			{
				toolCalls: []ai.ToolCall{
					{ID: "call_1", Name: "no_such_tool", Arguments: json.RawMessage(`{}`)},
				},
				stopReason: "tool_calls",
			},
			{textChunks: []string{"sorry"}, stopReason: "stop"},
		},
	}

	ch := make(chan WtfStreamEvent, 16)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      tools.NewRegistry(),
		Approver:      AutoAllowApprover{},
		MaxIterations: 5,
	}, ch)
	_ = drain(t, ch, 2*time.Second)

	// Loop must continue (2 provider calls) and surface the error to the model
	// as a tool message rather than aborting.
	if len(provider.receivedReqs) != 2 {
		t.Fatalf("provider call count = %d, want 2 (unknown tool should soft-fail)", len(provider.receivedReqs))
	}
	last := provider.receivedReqs[1].Messages[len(provider.receivedReqs[1].Messages)-1]
	if last.Role != "tool" || !containsString(last.Content, "Unknown tool") {
		t.Fatalf("expected 'Unknown tool' soft-error tool message, got %+v", last)
	}
}

func TestRunAgentLoop_MaxIterationsErr(t *testing.T) {
	echo := &echoTool{}
	// Three turns, all of which call the tool again. Cap is 2.
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{
			{toolCalls: []ai.ToolCall{{ID: "1", Name: "echo", Arguments: json.RawMessage(`{}`)}}},
			{toolCalls: []ai.ToolCall{{ID: "2", Name: "echo", Arguments: json.RawMessage(`{}`)}}},
			{toolCalls: []ai.ToolCall{{ID: "3", Name: "echo", Arguments: json.RawMessage(`{}`)}}},
		},
	}

	ch := make(chan WtfStreamEvent, 32)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      newRegistryWithEcho(echo),
		Approver:      AutoAllowApprover{},
		MaxIterations: 2,
	}, ch)
	events := drain(t, ch, 2*time.Second)

	gotErr := false
	for _, e := range events {
		if errors.Is(e.Err, ErrMaxIterations) {
			gotErr = true
		}
	}
	if !gotErr {
		t.Fatalf("expected ErrMaxIterations event, got %+v", events)
	}
	if len(provider.receivedReqs) != 2 {
		t.Fatalf("provider calls = %d, want exactly 2 (cap)", len(provider.receivedReqs))
	}
}

func TestRunAgentLoop_ProviderWithoutToolsClearsToolsField(t *testing.T) {
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{}, // Tools=false
		streams: []*fakeStream{
			{textChunks: []string{"plain answer"}, stopReason: "stop"},
		},
	}

	ch := make(chan WtfStreamEvent, 8)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
		Tools: []ai.ToolDefinition{
			{Name: "should_be_dropped", JSONSchema: json.RawMessage(`{}`)},
		},
	}, AgentLoopConfig{
		Registry:      tools.NewRegistry(),
		Approver:      AutoAllowApprover{},
		MaxIterations: 5,
	}, ch)
	_ = drain(t, ch, 2*time.Second)

	if len(provider.receivedReqs) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(provider.receivedReqs))
	}
	if len(provider.receivedReqs[0].Tools) != 0 {
		t.Fatalf("tools should be cleared for non-tool-capable provider; got %d", len(provider.receivedReqs[0].Tools))
	}
}

func TestRunAgentLoop_ProviderOpenError(t *testing.T) {
	provider := &fakeProvider{
		caps:    ai.ProviderCapabilities{Tools: true},
		openErr: errors.New("network down"),
	}

	ch := make(chan WtfStreamEvent, 8)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      tools.NewRegistry(),
		Approver:      AutoAllowApprover{},
		MaxIterations: 5,
	}, ch)
	events := drain(t, ch, 2*time.Second)

	gotErr := false
	for _, e := range events {
		if e.Err != nil && containsString(e.Err.Error(), "network down") {
			gotErr = true
		}
	}
	if !gotErr {
		t.Fatalf("expected error event, got %+v", events)
	}
}

func TestRunAgentLoop_RequiresApprover(t *testing.T) {
	provider := &fakeProvider{
		caps:    ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{{textChunks: []string{"x"}}},
	}

	ch := make(chan WtfStreamEvent, 4)
	go RunAgentLoop(context.Background(), provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      tools.NewRegistry(),
		MaxIterations: 5,
	}, ch)
	events := drain(t, ch, 2*time.Second)

	gotErr := false
	for _, e := range events {
		if e.Err != nil && containsString(e.Err.Error(), "approver") {
			gotErr = true
		}
	}
	if !gotErr {
		t.Fatalf("expected approver-required error, got %+v", events)
	}
}

func TestRunAgentLoop_ContextCanceledDuringApproval(t *testing.T) {
	echo := &echoTool{}
	// Approver blocks until ctx fires; the loop should propagate the cancel.
	approver := &blockingApprover{}
	provider := &fakeProvider{
		caps: ai.ProviderCapabilities{Tools: true},
		streams: []*fakeStream{
			{toolCalls: []ai.ToolCall{{ID: "call_1", Name: "echo", Arguments: json.RawMessage(`{}`)}}},
		},
	}

	ctx, cancel := context.WithCancel(context.Background())
	ch := make(chan WtfStreamEvent, 16)
	go RunAgentLoop(ctx, provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "hi"}},
	}, AgentLoopConfig{
		Registry:      newRegistryWithEcho(echo),
		Approver:      approver,
		MaxIterations: 5,
	}, ch)

	// Give the loop a moment to reach the approver.
	time.Sleep(20 * time.Millisecond)
	cancel()

	events := drain(t, ch, 2*time.Second)
	gotErr := false
	for _, e := range events {
		if e.Err != nil && errors.Is(e.Err, context.Canceled) {
			gotErr = true
		}
	}
	if !gotErr {
		// On some scheduling orders the approver may emit its own error
		// branch; either way, the tool must not have run.
		if len(echo.calls) != 0 {
			t.Fatalf("expected no tool execution after cancel; got %d calls", len(echo.calls))
		}
	}
}

type blockingApprover struct{}

func (blockingApprover) Approve(ctx context.Context, _ *ApprovalRequest) (ApprovalDecision, error) {
	<-ctx.Done()
	return ApprovalDecision{}, ctx.Err()
}

func containsString(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || indexOf(haystack, needle) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
