package continueprompt

import (
	"strings"
	"testing"

	"wtf_cli/pkg/commands"

	tea "charm.land/bubbletea/v2"
)

func mkRequest(toolCalls int) *commands.ContinuationRequest {
	return &commands.ContinuationRequest{ToolCalls: toolCalls}
}

func runKey(t *testing.T, p *Panel, code rune, text string) DecisionMsg {
	t.Helper()
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: code, Text: text}))
	if cmd == nil {
		t.Fatalf("key %q produced no command", text)
	}
	msg := cmd()
	d, ok := msg.(DecisionMsg)
	if !ok {
		t.Fatalf("expected DecisionMsg, got %T", msg)
	}
	return d
}

func TestPanel_ShowAndHide(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	if p.IsVisible() {
		t.Fatal("fresh panel should be invisible")
	}
	req := mkRequest(7)
	p.Show(req)
	if !p.IsVisible() {
		t.Fatal("panel should be visible after Show")
	}
	if p.Request() != req {
		t.Fatal("Request() should return the shown request")
	}
	p.Hide()
	if p.IsVisible() || p.Request() != nil {
		t.Fatal("panel should be invisible and forget request after Hide")
	}
}

func TestPanel_HiddenUpdateIsNoop(t *testing.T) {
	p := NewPanel()
	if cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter})); cmd != nil {
		t.Fatal("hidden panel should ignore keys")
	}
}

func TestPanel_ContinueShortcut(t *testing.T) {
	p := NewPanel()
	p.Show(mkRequest(3))
	d := runKey(t, p, '1', "1")
	if !d.Continue {
		t.Fatalf("key '1' should continue, got %+v", d)
	}
}

func TestPanel_StopShortcuts(t *testing.T) {
	for _, key := range []rune{'2', 'n', 'q'} {
		p := NewPanel()
		p.Show(mkRequest(3))
		d := runKey(t, p, key, string(key))
		if d.Continue {
			t.Fatalf("key %q should stop, got %+v", string(key), d)
		}
	}
}

func TestPanel_EscStops(t *testing.T) {
	p := NewPanel()
	p.Show(mkRequest(3))
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if cmd == nil {
		t.Fatal("esc should produce a command")
	}
	d := cmd().(DecisionMsg)
	if d.Continue {
		t.Fatalf("esc should stop (safe default), got %+v", d)
	}
}

func TestPanel_EnterConfirmsCursor(t *testing.T) {
	p := NewPanel()
	p.Show(mkRequest(3))
	// Default cursor is on "Continue".
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if d := cmd().(DecisionMsg); !d.Continue {
		t.Fatalf("enter on default cursor should continue, got %+v", d)
	}

	// Move cursor to "Stop" and confirm.
	p2 := NewPanel()
	p2.Show(mkRequest(3))
	_ = p2.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyRight}))
	cmd2 := p2.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if d := cmd2().(DecisionMsg); d.Continue {
		t.Fatalf("enter on Stop cursor should stop, got %+v", d)
	}
}

func TestPanel_CursorClampsAtBounds(t *testing.T) {
	p := NewPanel()
	p.Show(mkRequest(3))
	// Up at the top stays on Continue.
	if cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})); cmd != nil {
		t.Fatal("up at top should not emit a decision")
	}
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if d := cmd().(DecisionMsg); !d.Continue {
		t.Fatalf("cursor should still be on Continue, got %+v", d)
	}
}

func TestPanel_ViewMentionsToolCount(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	p.Show(mkRequest(42))
	view := p.View()
	if !strings.Contains(view, "42") {
		t.Fatalf("view should mention the tool-call count; got:\n%s", view)
	}
	if !strings.Contains(view, "Continue") {
		t.Fatalf("view should render the Continue button; got:\n%s", view)
	}
}
