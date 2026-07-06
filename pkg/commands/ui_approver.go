package commands

import (
	"context"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
)

// SessionApprovals is a per-process set of "allow always this session"
// approvals keyed by tool name. Safe for concurrent use.
//
// The UI's Model holds a single instance for the lifetime of the wtf_cli
// process so that approving "always" for a tool persists across multiple
// /explain or /chat invocations.
//
// This store governs in-workdir calls only: an escape request (a call
// targeting a path outside the working directory) is never auto-allowed by
// a tool-name grant here, no matter how permissive — see PathGrants.
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

// PathGrants is a per-process set of "allow this tool to access this
// directory for the rest of the session" grants for out-of-workdir tool
// calls. Safe for concurrent use.
//
// Keyed by (tool name, directory) — deliberately NOT shared across tools.
// Directory listing and file-content reading are different capabilities: a
// grant that lets list_directory enumerate $HOME must never let read_file
// open ~/.ssh/id_rsa. Each tool's grants are stored and checked completely
// independently of every other tool's.
type PathGrants struct {
	mu     sync.RWMutex
	grants map[string][]string // tool name -> resolved, absolute, clean directories
}

// NewPathGrants returns a fresh, empty path-grant store.
func NewPathGrants() *PathGrants {
	return &PathGrants{grants: make(map[string][]string)}
}

// Allow records dir as granted to tool for the rest of the session. dir is
// expected to already be an absolute, filepath.Clean-ed directory (the
// approval flow verifies this and the containment it implies before
// calling Allow); as a defense-in-depth backstop for any other caller, a
// malformed dir is rejected and logged rather than silently coerced or
// stored, since this store is a security boundary in its own right.
func (g *PathGrants) Allow(tool, dir string) {
	if g == nil {
		return
	}
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		slog.Warn("path_grant_rejected_malformed_dir", "tool", tool, "dir", dir)
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.grants == nil {
		g.grants = make(map[string][]string)
	}
	g.grants[tool] = append(g.grants[tool], dir)
}

// IsAllowed reports whether path falls under any directory granted to tool —
// a path-separator-boundary prefix match, never a bare string prefix (which
// would wrongly accept a sibling like "/var/log2" under a grant for
// "/var/log"). Both path and the stored grant directories are expected to
// already be resolved, absolute paths.
func (g *PathGrants) IsAllowed(tool, path string) bool {
	if g == nil {
		return false
	}
	g.mu.RLock()
	defer g.mu.RUnlock()
	for _, dir := range g.grants[tool] {
		if dirContainsPath(dir, path) {
			return true
		}
	}
	return false
}

// dirContainsPath reports whether path is dir itself or lies under it, using
// a path-separator boundary rather than a bare string prefix (which would
// wrongly accept a sibling like "/var/log2" under "/var/log"). dir == "/" is
// special-cased: string(filepath.Separator)+dir would otherwise build the
// non-matching prefix "//", which is the one shape a boundary check can't
// derive generically. This is the single boundary-check implementation
// shared by PathGrants.IsAllowed and recordEscapeGrant's containment
// validation — duplicating it once already caused them to disagree on the
// root case.
func dirContainsPath(dir, path string) bool {
	if path == dir || dir == string(filepath.Separator) {
		return true
	}
	return strings.HasPrefix(path, dir+string(filepath.Separator))
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
// Two independent stores back "always allow this session", checked first
// and skipping the UI event entirely when they apply: SessionApprovals for
// ordinary (in-workdir) calls, keyed by tool name; PathGrants for escape
// (out-of-workdir) calls, keyed by (tool name, directory). A tool-name grant
// never auto-allows an escape, and an escape grant is recorded only in
// PathGrants — the two stores never influence each other.
type UIApprover struct {
	out    chan<- WtfStreamEvent
	policy *SessionApprovals
	grants *PathGrants
}

// NewUIApprover wires a UIApprover to the given event channel, session
// policy store, and path-grant store.
func NewUIApprover(out chan<- WtfStreamEvent, policy *SessionApprovals, grants *PathGrants) *UIApprover {
	return &UIApprover{out: out, policy: policy, grants: grants}
}

// Approve implements Approver.
func (a *UIApprover) Approve(ctx context.Context, req *ApprovalRequest) (ApprovalDecision, error) {
	if req.Escape != nil {
		return a.approveEscape(ctx, req)
	}

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

// approveEscape handles the out-of-workdir branch of Approve: a directory
// grant already covering this exact tool and resolved path auto-allows
// without any UI event; otherwise the popup is shown, and a "remember this"
// reply is recorded per (tool, directory) in PathGrants — never in the
// tool-name SessionApprovals store, which stays reserved for in-workdir
// calls.
func (a *UIApprover) approveEscape(ctx context.Context, req *ApprovalRequest) (ApprovalDecision, error) {
	if a.grants != nil && a.grants.IsAllowed(req.Name, req.Escape.ResolvedPath) {
		return ApprovalDecision{Allow: true, AllowOutsideWorkdir: true, Persistent: true}, nil
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
		if !d.Allow {
			return d, nil
		}
		d.AllowOutsideWorkdir = true
		if d.Persistent && a.grants != nil {
			a.recordEscapeGrant(req)
		}
		return d, nil
	case <-ctx.Done():
		return ApprovalDecision{}, ctx.Err()
	}
}

// recordEscapeGrant persists req.Escape.GrantDir for req.Name after
// verifying it actually contains the resolved path the user approved. A
// malformed or non-containing GrantDir is logged and dropped rather than
// stored — the in-flight decision the caller already returned still stands
// as a one-off allow, but nothing broader is remembered for next time.
func (a *UIApprover) recordEscapeGrant(req *ApprovalRequest) {
	dir := req.Escape.GrantDir
	resolved := req.Escape.ResolvedPath
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir {
		slog.Warn("path_grant_rejected_malformed_dir", "tool", req.Name, "dir", dir)
		return
	}
	if !dirContainsPath(dir, resolved) {
		slog.Warn("path_grant_rejected_dir_does_not_contain_resolved_path",
			"tool", req.Name, "dir", dir, "resolved_path", resolved)
		return
	}
	a.grants.Allow(req.Name, dir)
}
