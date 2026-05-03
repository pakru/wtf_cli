package commands

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestSessionApprovals_AllowAndIsAllowed(t *testing.T) {
	s := NewSessionApprovals()
	if s.IsAllowed("read_file") {
		t.Fatal("freshly created store should not have any tool allowed")
	}
	s.Allow("read_file")
	if !s.IsAllowed("read_file") {
		t.Fatal("expected read_file to be allowed after Allow")
	}
	if s.IsAllowed("other_tool") {
		t.Fatal("only the named tool should be allowed")
	}
}

func TestSessionApprovals_Reset(t *testing.T) {
	s := NewSessionApprovals()
	s.Allow("a")
	s.Allow("b")
	s.Reset()
	if s.IsAllowed("a") || s.IsAllowed("b") {
		t.Fatal("Reset() should clear all entries")
	}
}

func TestSessionApprovals_NilSafe(t *testing.T) {
	var s *SessionApprovals
	// All methods must be no-ops on nil receiver.
	if s.IsAllowed("x") {
		t.Fatal("nil store should report not-allowed")
	}
	s.Allow("x")
	s.Reset()
}

// TestUIApprover_SessionAllowSkipsPopup verifies that when the session policy
// already permits a tool, the approver returns immediately *without* emitting
// any UI event — that's the whole point of the "always allow" flow.
func TestUIApprover_SessionAllowSkipsPopup(t *testing.T) {
	policy := NewSessionApprovals()
	policy.Allow("read_file")

	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, policy)

	d, err := approver.Approve(context.Background(), &ApprovalRequest{Name: "read_file"})
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !d.Allow || !d.Persistent {
		t.Fatalf("session-allowed call should report Allow+Persistent, got %+v", d)
	}

	// No UI event should have been emitted.
	select {
	case ev := <-out:
		t.Fatalf("expected no UI event for session-allowed call; got %+v", ev)
	case <-time.After(20 * time.Millisecond):
	}
}

// TestUIApprover_HappyPath drives the full event-and-reply round trip.
func TestUIApprover_HappyPath(t *testing.T) {
	policy := NewSessionApprovals()
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, policy)

	// Run Approve in a goroutine; pretend the UI receives the event and
	// dispatches a decision.
	type result struct {
		decision ApprovalDecision
		err      error
	}
	resultCh := make(chan result, 1)
	go func() {
		d, err := approver.Approve(context.Background(), &ApprovalRequest{Name: "echo"})
		resultCh <- result{d, err}
	}()

	// Read the popup-open event.
	var ev WtfStreamEvent
	select {
	case ev = <-out:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ToolApproval event")
	}
	if ev.ToolApproval == nil {
		t.Fatalf("expected ToolApproval event, got %+v", ev)
	}
	if ev.ToolApproval.Reply == nil {
		t.Fatalf("approver should have allocated Reply channel before sending event")
	}

	// Dispatch user's "allow always this session" decision.
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: true}

	got := <-resultCh
	if got.err != nil {
		t.Fatalf("Approve err: %v", got.err)
	}
	if !got.decision.Allow || !got.decision.Persistent {
		t.Fatalf("decision = %+v, want Allow+Persistent", got.decision)
	}
	// Persistent decisions must update the session policy.
	if !policy.IsAllowed("echo") {
		t.Fatal("Persistent=true decision should mark tool allowed in session policy")
	}
}

func TestUIApprover_NonPersistentAllowDoesNotMutateSession(t *testing.T) {
	policy := NewSessionApprovals()
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, policy)

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), &ApprovalRequest{Name: "echo"})
		resultCh <- d
	}()
	ev := <-out
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: false}
	<-resultCh

	if policy.IsAllowed("echo") {
		t.Fatal("non-persistent allow should not update session policy")
	}
}

// TestUIApprover_CtxCancelDuringWait covers the case where the loop's context
// is canceled while the popup is up. The approver must return promptly with
// the context's error rather than blocking forever on Reply.
func TestUIApprover_CtxCancelDuringWait(t *testing.T) {
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, NewSessionApprovals())

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := approver.Approve(ctx, &ApprovalRequest{Name: "echo"})
		resultCh <- err
	}()

	// Drain the event so the approver progresses to the Reply wait.
	select {
	case <-out:
	case <-time.After(time.Second):
		t.Fatal("did not receive ToolApproval event")
	}

	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("approver did not unblock after ctx cancel")
	}
}

// TestUIApprover_ChannelBufferAvoidsDeadlock verifies the documented
// concurrency contract: with a buffered out channel, the approver's send
// returns even when no listener is currently reading. This is the
// "deadlock-avoidance" invariant called out in the plan.
func TestUIApprover_ChannelBufferAvoidsDeadlock(t *testing.T) {
	out := make(chan WtfStreamEvent, 16)
	approver := NewUIApprover(out, NewSessionApprovals())

	// Start the approver but never read the channel until after a delay.
	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), &ApprovalRequest{Name: "echo"})
		resultCh <- d
	}()

	// Wait long enough that, if the approver were not buffered, it would
	// hang. Then drain.
	time.Sleep(50 * time.Millisecond)
	ev := <-out
	if ev.ToolApproval == nil {
		t.Fatalf("expected ToolApproval, got %+v", ev)
	}
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true}

	select {
	case <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("approver did not complete after dispatch")
	}
}

// TestUIApprover_ConcurrentDecisionsDistinctTools ensures the session policy
// is concurrency-safe and that tool-by-tool tracking does not bleed between
// goroutines.
func TestUIApprover_ConcurrentDecisionsDistinctTools(t *testing.T) {
	policy := NewSessionApprovals()
	var wg sync.WaitGroup
	tools := []string{"a", "b", "c", "d"}
	for _, name := range tools {
		wg.Add(1)
		go func(name string) {
			defer wg.Done()
			out := make(chan WtfStreamEvent, 4)
			approver := NewUIApprover(out, policy)
			done := make(chan struct{})
			go func() {
				_, _ = approver.Approve(context.Background(), &ApprovalRequest{Name: name})
				close(done)
			}()
			ev := <-out
			ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: true}
			<-done
		}(name)
	}
	wg.Wait()

	for _, name := range tools {
		if !policy.IsAllowed(name) {
			t.Fatalf("expected %q to be allowed", name)
		}
	}
}
