package commands

import "context"

// UIContinuer bridges the agent goroutine and the Bubble Tea main loop for the
// "continue this tool-call loop?" prompt. It emits a
// WtfStreamEvent{ContinuePrompt:...} on the stream channel and blocks on
// req.Reply until the UI dispatches a decision.
//
// Concurrency contract (mirrors UIApprover — see ui_approver.go):
//   - The stream channel must be buffered. UIContinuer sends one event and
//     immediately blocks on Reply; an unbuffered channel could deadlock.
//   - req.Reply is allocated by the continuer with capacity 1, so the UI's
//     single send never blocks.
//   - The continuer honors ctx.Done() while waiting. Today nothing cancels the
//     loop context from the UI, so in practice the user answers via the modal;
//     the ctx.Done() branch is defensive. The continue feature does not depend
//     on ctx cancellation — a "stop" decision is a normal reply that makes the
//     loop emit a graceful Done.
type UIContinuer struct {
	out chan<- WtfStreamEvent
}

// NewUIContinuer wires a UIContinuer to the given event channel.
func NewUIContinuer(out chan<- WtfStreamEvent) *UIContinuer {
	return &UIContinuer{out: out}
}

// Continue implements Continuer.
func (c *UIContinuer) Continue(ctx context.Context, req *ContinuationRequest) (ContinuationDecision, error) {
	if req.Reply == nil {
		req.Reply = make(chan ContinuationDecision, 1)
	}

	select {
	case c.out <- WtfStreamEvent{ContinuePrompt: req}:
	case <-ctx.Done():
		return ContinuationDecision{}, ctx.Err()
	}

	select {
	case d := <-req.Reply:
		return d, nil
	case <-ctx.Done():
		return ContinuationDecision{}, ctx.Err()
	}
}
