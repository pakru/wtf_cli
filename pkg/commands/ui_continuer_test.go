package commands

import (
	"context"
	"errors"
	"testing"
	"time"
)

// TestUIContinuer_HappyPath drives the full event-and-reply round trip: the
// continuer emits a ContinuePrompt event and blocks until the UI dispatches a
// decision on the request's Reply channel.
func TestUIContinuer_HappyPath(t *testing.T) {
	out := make(chan WtfStreamEvent, 4)
	continuer := NewUIContinuer(out)

	type result struct {
		decision ContinuationDecision
		err      error
	}
	resultCh := make(chan result, 1)
	go func() {
		d, err := continuer.Continue(context.Background(), &ContinuationRequest{ToolCalls: 5})
		resultCh <- result{d, err}
	}()

	var ev WtfStreamEvent
	select {
	case ev = <-out:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for ContinuePrompt event")
	}
	if ev.ContinuePrompt == nil {
		t.Fatalf("expected ContinuePrompt event, got %+v", ev)
	}
	if ev.ContinuePrompt.Reply == nil {
		t.Fatal("continuer should have allocated Reply channel before sending event")
	}
	if ev.ContinuePrompt.ToolCalls != 5 {
		t.Fatalf("ToolCalls = %d, want 5", ev.ContinuePrompt.ToolCalls)
	}

	ev.ContinuePrompt.Reply <- ContinuationDecision{Continue: true}

	got := <-resultCh
	if got.err != nil {
		t.Fatalf("Continue err: %v", got.err)
	}
	if !got.decision.Continue {
		t.Fatalf("decision = %+v, want Continue=true", got.decision)
	}
}

// TestUIContinuer_StopDecision verifies a "stop" reply propagates back.
func TestUIContinuer_StopDecision(t *testing.T) {
	out := make(chan WtfStreamEvent, 4)
	continuer := NewUIContinuer(out)

	resultCh := make(chan ContinuationDecision, 1)
	go func() {
		d, _ := continuer.Continue(context.Background(), &ContinuationRequest{})
		resultCh <- d
	}()

	ev := <-out
	ev.ContinuePrompt.Reply <- ContinuationDecision{Continue: false}

	got := <-resultCh
	if got.Continue {
		t.Fatalf("decision = %+v, want Continue=false", got)
	}
}

// TestUIContinuer_CtxCancelDuringWait covers cancellation while the popup is
// up: the continuer must return promptly with the context error rather than
// blocking forever on Reply.
func TestUIContinuer_CtxCancelDuringWait(t *testing.T) {
	out := make(chan WtfStreamEvent, 4)
	continuer := NewUIContinuer(out)

	ctx, cancel := context.WithCancel(context.Background())
	resultCh := make(chan error, 1)
	go func() {
		_, err := continuer.Continue(ctx, &ContinuationRequest{})
		resultCh <- err
	}()

	select {
	case <-out:
	case <-time.After(time.Second):
		t.Fatal("did not receive ContinuePrompt event")
	}

	cancel()

	select {
	case err := <-resultCh:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("continuer did not unblock after ctx cancel")
	}
}

// TestUIContinuer_ChannelBufferAvoidsDeadlock verifies the documented
// concurrency contract: with a buffered out channel, the send returns even
// when no listener is currently reading.
func TestUIContinuer_ChannelBufferAvoidsDeadlock(t *testing.T) {
	out := make(chan WtfStreamEvent, 16)
	continuer := NewUIContinuer(out)

	resultCh := make(chan ContinuationDecision, 1)
	go func() {
		d, _ := continuer.Continue(context.Background(), &ContinuationRequest{})
		resultCh <- d
	}()

	time.Sleep(50 * time.Millisecond)
	ev := <-out
	if ev.ContinuePrompt == nil {
		t.Fatalf("expected ContinuePrompt, got %+v", ev)
	}
	ev.ContinuePrompt.Reply <- ContinuationDecision{Continue: true}

	select {
	case <-resultCh:
	case <-time.After(time.Second):
		t.Fatal("continuer did not complete after dispatch")
	}
}
