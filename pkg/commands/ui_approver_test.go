package commands

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"wtf_cli/pkg/ai/tools"
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
	approver := NewUIApprover(out, policy, NewPathGrants())

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
	approver := NewUIApprover(out, policy, NewPathGrants())

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
	approver := NewUIApprover(out, policy, NewPathGrants())

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
	approver := NewUIApprover(out, NewSessionApprovals(), NewPathGrants())

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
	approver := NewUIApprover(out, NewSessionApprovals(), NewPathGrants())

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
			approver := NewUIApprover(out, policy, NewPathGrants())
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

func mkEscapeRequest(tool, resolved, grantDir string) *ApprovalRequest {
	return &ApprovalRequest{
		Name: tool,
		Escape: &tools.EscapeRequest{
			RequestedPath: resolved,
			ResolvedPath:  resolved,
			GrantDir:      grantDir,
			Target:        tools.FileID{Dev: 1, Ino: 1, Valid: true},
		},
	}
}

// TestUIApprover_EscapeRequest_NotAutoAllowedByToolNameSessionGrant is the
// central regression the whole feature exists to fix: session-allowing a
// tool by name must never silently unlock out-of-workdir access.
func TestUIApprover_EscapeRequest_NotAutoAllowedByToolNameSessionGrant(t *testing.T) {
	policy := NewSessionApprovals()
	policy.Allow("read_file")

	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, policy, NewPathGrants())

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/etc/hosts", "/etc"))
		resultCh <- d
	}()

	select {
	case ev := <-out:
		if ev.ToolApproval == nil {
			t.Fatalf("expected a popup event for the escape request, got %+v", ev)
		}
		ev.ToolApproval.Reply <- ApprovalDecision{Allow: true}
	case <-time.After(time.Second):
		t.Fatal("tool-name session grant incorrectly skipped the popup for an escape request")
	}
	<-resultCh
}

func TestUIApprover_EscapeRequest_AutoAllowedByCoveringPathGrant(t *testing.T) {
	grants := NewPathGrants()
	grants.Allow("read_file", "/etc")
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, NewSessionApprovals(), grants)

	d, err := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/etc/hosts", "/etc"))
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}
	if !d.Allow || !d.AllowOutsideWorkdir || !d.Persistent {
		t.Fatalf("expected Allow+AllowOutsideWorkdir+Persistent from a covering grant, got %+v", d)
	}

	select {
	case ev := <-out:
		t.Fatalf("expected no UI event when a path grant already covers the request; got %+v", ev)
	case <-time.After(20 * time.Millisecond):
	}
}

func TestUIApprover_EscapeRequest_PopupAllowOnceDoesNotPersist(t *testing.T) {
	grants := NewPathGrants()
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, NewSessionApprovals(), grants)

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/etc/hosts", "/etc"))
		resultCh <- d
	}()

	ev := <-out
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: false}
	d := <-resultCh

	if !d.Allow || !d.AllowOutsideWorkdir {
		t.Fatalf("expected Allow+AllowOutsideWorkdir for an allow-once escape decision, got %+v", d)
	}
	if grants.IsAllowed("read_file", "/etc/hosts") {
		t.Fatal("allow-once must not record a path grant")
	}
}

// TestUIApprover_EscapeRequest_PersistentDecisionRecordsInPathGrantsNotSessionApprovals
// verifies the decision lands in the *directory* grant store, never the
// tool-name store — the popup said "directory", so that is what must be
// remembered.
func TestUIApprover_EscapeRequest_PersistentDecisionRecordsInPathGrantsNotSessionApprovals(t *testing.T) {
	policy := NewSessionApprovals()
	grants := NewPathGrants()
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, policy, grants)

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/etc/hosts", "/etc"))
		resultCh <- d
	}()

	ev := <-out
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: true}
	d := <-resultCh

	if !d.AllowOutsideWorkdir {
		t.Fatal("expected AllowOutsideWorkdir=true")
	}
	if !grants.IsAllowed("read_file", "/etc/hosts") {
		t.Fatal("expected the persistent decision to record a path grant for read_file under /etc")
	}
	if policy.IsAllowed("read_file") {
		t.Fatal("a persistent escape decision must never grant blanket tool-name approval")
	}
}

// TestUIApprover_EscapeRequest_PersistentRootGrantIsRecorded is the specific
// regression for a file directly under "/": GrantDir == "/" is the one case
// a naive `dir + separator` containment check gets wrong (it builds "//",
// which no real resolved path starts with), so a persistent grant for a
// root-level file used to silently fail to save even though the ordinary
// (non-root) case worked fine.
func TestUIApprover_EscapeRequest_PersistentRootGrantIsRecorded(t *testing.T) {
	policy := NewSessionApprovals()
	grants := NewPathGrants()
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, policy, grants)

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/secret.txt", "/"))
		resultCh <- d
	}()

	ev := <-out
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: true}
	d := <-resultCh

	if !d.AllowOutsideWorkdir {
		t.Fatal("expected AllowOutsideWorkdir=true")
	}
	if !grants.IsAllowed("read_file", "/secret.txt") {
		t.Fatal("expected a persistent root-level grant (GrantDir \"/\") to be recorded and to cover a file directly under root")
	}
}

// TestUIApprover_EscapeRequest_GrantDirNotContainingResolvedPathIsNotStored
// covers a malformed EscapeRequest (a tool implementation bug): the approver
// must not blindly trust GrantDir and store an over-broad grant.
func TestUIApprover_EscapeRequest_GrantDirNotContainingResolvedPathIsNotStored(t *testing.T) {
	grants := NewPathGrants()
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, NewSessionApprovals(), grants)

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		// GrantDir ("/var/log") does not contain ResolvedPath ("/etc/hosts").
		d, _ := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/etc/hosts", "/var/log"))
		resultCh <- d
	}()

	ev := <-out
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: true, Persistent: true}
	d := <-resultCh

	if !d.Allow {
		t.Fatal("the in-flight decision should still be an allow")
	}
	if grants.IsAllowed("read_file", "/etc/hosts") {
		t.Fatal("a GrantDir that does not contain ResolvedPath must not be stored")
	}
}

func TestUIApprover_EscapeRequest_DenyDoesNotSetAllowOutsideWorkdir(t *testing.T) {
	out := make(chan WtfStreamEvent, 4)
	approver := NewUIApprover(out, NewSessionApprovals(), NewPathGrants())

	resultCh := make(chan ApprovalDecision, 1)
	go func() {
		d, _ := approver.Approve(context.Background(), mkEscapeRequest("read_file", "/etc/hosts", "/etc"))
		resultCh <- d
	}()

	ev := <-out
	ev.ToolApproval.Reply <- ApprovalDecision{Allow: false}
	d := <-resultCh

	if d.Allow || d.AllowOutsideWorkdir {
		t.Fatalf("a denial must not carry Allow or AllowOutsideWorkdir, got %+v", d)
	}
}
