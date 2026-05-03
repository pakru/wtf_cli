package commands

import (
	"context"
	"sync"
)

// SessionApprovals is a per-process set of "allow always this session"
// approvals keyed by tool name. Safe for concurrent use.
//
// The UI's Model holds a single instance for the lifetime of the wtf_cli
// process so that approving "always" for a tool persists across multiple
// /explain or /chat invocations.
type SessionApprovals struct {
	allowed sync.Map // map[toolName]struct{}
}

// NewSessionApprovals returns a fresh, empty session-policy store.
func NewSessionApprovals() *SessionApprovals {
	return &SessionApprovals{}
}

// IsAllowed reports whether the named tool was previously approved with
// "always allow this session".
func (s *SessionApprovals) IsAllowed(toolName string) bool {
	if s == nil {
		return false
	}
	_, ok := s.allowed.Load(toolName)
	return ok
}

// Allow marks the named tool as allowed for the rest of the session.
func (s *SessionApprovals) Allow(toolName string) {
	if s == nil {
		return
	}
	s.allowed.Store(toolName, struct{}{})
}

// Reset clears every per-session "always allow" entry. Useful when the user
// chooses a session-reset action.
func (s *SessionApprovals) Reset() {
	if s == nil {
		return
	}
	s.allowed.Range(func(k, _ any) bool {
		s.allowed.Delete(k)
		return true
	})
}

// UIApprover bridges the agent goroutine and the Bubble Tea main loop. It
// emits a WtfStreamEvent{ToolApproval:...} on the stream channel and blocks on
// req.Reply until the UI dispatches a decision.
//
// Concurrency contract (must hold to avoid deadlock — see plan Phase 5):
//   - The stream channel must be buffered. UIApprover sends one event and
//     immediately blocks on Reply; if the channel were unbuffered and the
//     listener goroutine were not currently reading, the send itself would
//     block.
//   - req.Reply is allocated by the approver with capacity 1, so the UI's
//     single send never blocks.
//   - The approver honors ctx.Done() while waiting, so a sidebar-close /
//     Ctrl+C cancellation cleanly aborts even with the popup still up.
//
// Session policy ("always allow this session") is checked first: when set,
// the approver returns immediately without emitting any UI event.
type UIApprover struct {
	out    chan<- WtfStreamEvent
	policy *SessionApprovals
}

// NewUIApprover wires a UIApprover to the given event channel and session
// policy store.
func NewUIApprover(out chan<- WtfStreamEvent, policy *SessionApprovals) *UIApprover {
	return &UIApprover{out: out, policy: policy}
}

// Approve implements Approver.
func (a *UIApprover) Approve(ctx context.Context, req *ApprovalRequest) (ApprovalDecision, error) {
	if a.policy != nil && a.policy.IsAllowed(req.Name) {
		return ApprovalDecision{Allow: true, Persistent: true}, nil
	}
	if req.Reply == nil {
		req.Reply = make(chan ApprovalDecision, 1)
	}

	select {
	case a.out <- WtfStreamEvent{ToolApproval: req}:
	case <-ctx.Done():
		return ApprovalDecision{}, ctx.Err()
	}

	select {
	case d := <-req.Reply:
		if d.Allow && d.Persistent && a.policy != nil {
			a.policy.Allow(req.Name)
		}
		return d, nil
	case <-ctx.Done():
		return ApprovalDecision{}, ctx.Err()
	}
}
