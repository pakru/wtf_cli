package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"wtf_cli/pkg/ai"
	"wtf_cli/pkg/ai/tools"
)

// scriptedProvider is a stand-in for a real LLM provider — the same seam
// fakeProvider/fakeStream in agent_loop_test.go use — since driving these
// tests through a real model requires network access and API credentials
// the test environment does not have. Everything downstream (RunAgentLoop,
// UIApprover, PathGrants, the real read_file tool, real files on a real
// filesystem) is production code, not a test double.
type scriptedProvider struct {
	streams []*scriptedStream
	i       int
}

func (p *scriptedProvider) CreateChatCompletion(_ context.Context, _ ai.ChatRequest) (ai.ChatResponse, error) {
	return ai.ChatResponse{}, fmt.Errorf("not used")
}
func (p *scriptedProvider) CreateChatCompletionStream(_ context.Context, _ ai.ChatRequest) (ai.ChatStream, error) {
	s := p.streams[p.i]
	p.i++
	return s, nil
}
func (p *scriptedProvider) Capabilities() ai.ProviderCapabilities {
	return ai.ProviderCapabilities{Tools: true}
}

type scriptedStream struct {
	toolCalls []ai.ToolCall
	text      string
	done      bool
}

func (s *scriptedStream) Next() bool {
	if s.done || s.text == "" {
		return false
	}
	s.done = true
	return true
}
func (s *scriptedStream) Content() string          { return s.text }
func (s *scriptedStream) Err() error               { return nil }
func (s *scriptedStream) Close() error             { return nil }
func (s *scriptedStream) ToolCalls() []ai.ToolCall { return s.toolCalls }
func (s *scriptedStream) StopReason() string       { return "stop" }

func oneShotEscapeStream(id, toolName string, argsPath string) []*scriptedStream {
	return []*scriptedStream{
		{toolCalls: []ai.ToolCall{{ID: id, Name: toolName, Arguments: json.RawMessage(fmt.Sprintf(`{"path":%q}`, argsPath))}}},
		{text: "done"},
	}
}

// TestEscapeIntegration_DenyPolicyBehavesLikeTodayEndToEnd drives the real
// production stack (RunAgentLoop -> executeOneTool -> real ReadFile tool)
// with AllowEscapes=false (the "deny" config policy) and confirms: no escape
// classification ever happens (ApprovalRequest.Escape stays nil, so only the
// ordinary popup would ever show — never the escape variant), and the real
// tool still rejects the out-of-workdir path with the standard containment
// error, exactly as it did before this feature existed.
func TestEscapeIntegration_DenyPolicyBehavesLikeTodayEndToEnd(t *testing.T) {
	cwd := t.TempDir()
	outside := t.TempDir()
	target := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(target, []byte("classified\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFile(cwd, 500, 65536, false)) // AllowEscapes=false: "deny"

	approver := &fixedDecisionApprover{decision: ApprovalDecision{Allow: true}}
	provider := &scriptedProvider{streams: oneShotEscapeStream("1", "read_file", target)}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out := make(chan WtfStreamEvent, 16)
	go RunAgentLoop(ctx, provider, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "read it"}},
	}, AgentLoopConfig{Registry: registry, Approver: approver, MaxIterations: 5}, out)

	var finished *ToolCallInfo
	for ev := range out {
		if ev.ToolCallFinished != nil {
			finished = ev.ToolCallFinished
		}
		if ev.Err != nil {
			t.Fatalf("agent loop error: %v", ev.Err)
		}
	}

	if approver.lastReq == nil {
		t.Fatal("expected the approver to be called")
	}
	if approver.lastReq.Escape != nil {
		t.Fatalf("deny policy must never classify an escape; got %+v", approver.lastReq.Escape)
	}
	if finished == nil {
		t.Fatal("expected a ToolCallFinished event")
	}
	if finished.ErrorMessage == "" || !strings.Contains(finished.ErrorMessage, "not approved") {
		t.Fatalf("expected the real tool to reject the outside path with the standard containment error, got: %+v", finished)
	}
}

// TestEscapeIntegration_SymlinkRepointedAfterGrantReprompts is the "symlink
// under an existing grant" case: classification runs fresh on every call
// (ReadFile.ClassifyCall resolves symlinks each time), so once the in-tree
// symlink the model references is repointed to a directory outside any
// existing grant, the next call's fresh resolution no longer matches what
// was granted and must prompt again rather than silently auto-allow through
// stale path-string matching.
func TestEscapeIntegration_SymlinkRepointedAfterGrantReprompts(t *testing.T) {
	if testing.Short() {
		t.Skip("uses real symlinks")
	}
	cwd := t.TempDir()
	dirA := t.TempDir()
	dirB := t.TempDir()
	fileA := filepath.Join(dirA, "a.txt")
	fileB := filepath.Join(dirB, "b.txt")
	if err := os.WriteFile(fileA, []byte("content A\n"), 0o644); err != nil {
		t.Fatalf("write A: %v", err)
	}
	if err := os.WriteFile(fileB, []byte("content B\n"), 0o644); err != nil {
		t.Fatalf("write B: %v", err)
	}

	link := filepath.Join(cwd, "link")
	if err := os.Symlink(dirA, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	registry := tools.NewRegistry()
	registry.Register(tools.NewReadFile(cwd, 500, 65536, true))

	policy := NewSessionApprovals()
	grants := NewPathGrants()

	// First call: through the symlink pointing at dirA. Approve "for session".
	out1 := make(chan WtfStreamEvent, 16)
	approver1 := NewUIApprover(out1, policy, grants)
	provider1 := &scriptedProvider{streams: oneShotEscapeStream("1", "read_file", filepath.Join(link, "a.txt"))}
	ctx1, cancel1 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel1()
	go RunAgentLoop(ctx1, provider1, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "read via link"}},
	}, AgentLoopConfig{Registry: registry, Approver: approver1, MaxIterations: 5}, out1)

	var firstResult string
	for ev := range out1 {
		if ev.ToolApproval != nil {
			ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: true, AllowOutsideWorkdir: true}
		}
		if ev.ToolCallFinished != nil {
			firstResult = ev.ToolCallFinished.Result
		}
		if ev.Err != nil {
			t.Fatalf("agent loop error: %v", ev.Err)
		}
	}
	if !strings.Contains(firstResult, "content A") {
		t.Fatalf("expected the first call to read dirA's file, got: %q", firstResult)
	}
	if !grants.IsAllowed("read_file", fileA) {
		t.Fatal("expected the session grant to cover dirA")
	}

	// Repoint the symlink to dirB — a directory never covered by any grant.
	if err := os.Remove(link); err != nil {
		t.Fatalf("remove symlink: %v", err)
	}
	if err := os.Symlink(dirB, link); err != nil {
		t.Fatalf("re-symlink: %v", err)
	}

	out2 := make(chan WtfStreamEvent, 16)
	approver2 := NewUIApprover(out2, policy, grants) // same grants store
	provider2 := &scriptedProvider{streams: oneShotEscapeStream("2", "read_file", filepath.Join(link, "b.txt"))}
	ctx2, cancel2 := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel2()
	go RunAgentLoop(ctx2, provider2, ai.ChatRequest{
		Messages: []ai.Message{{Role: "user", Content: "read via link again"}},
	}, AgentLoopConfig{Registry: registry, Approver: approver2, MaxIterations: 5}, out2)

	sawPopup := false
	var secondResult string
	for ev := range out2 {
		if ev.ToolApproval != nil {
			sawPopup = true
			ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, AllowOutsideWorkdir: true}
		}
		if ev.ToolCallFinished != nil {
			secondResult = ev.ToolCallFinished.Result
		}
		if ev.Err != nil {
			t.Fatalf("agent loop error: %v", ev.Err)
		}
	}
	if !sawPopup {
		t.Fatal("expected a fresh popup after the symlink was repointed to an ungranted directory — the stale grant for dirA must not auto-allow dirB")
	}
	if !strings.Contains(secondResult, "content B") {
		t.Fatalf("expected the second call to read dirB's file after approval, got: %q", secondResult)
	}
}
