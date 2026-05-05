package toolapproval

import (
	"encoding/json"
	"strings"
	"testing"

	"wtf_cli/pkg/commands"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func mkRequest(name, args string) *commands.ApprovalRequest {
	return &commands.ApprovalRequest{
		Name: name,
		Args: json.RawMessage(args),
	}
}

func runKey(t *testing.T, p *Panel, key string) DecisionMsg {
	t.Helper()
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: rune(key[0]), Text: key}))
	if cmd == nil {
		t.Fatalf("key %q produced no command", key)
	}
	msg := cmd()
	d, ok := msg.(DecisionMsg)
	if !ok {
		t.Fatalf("expected DecisionMsg, got %T", msg)
	}
	return d
}

func TestPanel_ShowSetsState(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	if p.IsVisible() {
		t.Fatal("fresh panel should be invisible")
	}
	p.Show(mkRequest("echo", `{"x":1}`))
	if !p.IsVisible() {
		t.Fatal("panel should be visible after Show")
	}
	if p.Request() == nil {
		t.Fatal("Request() should be set after Show")
	}
}

func TestPanel_HideClearsRequest(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	p.Show(mkRequest("echo", `{}`))
	p.Hide()
	if p.IsVisible() {
		t.Fatal("hidden panel should not be visible")
	}
	if p.Request() != nil {
		t.Fatal("Hide should clear Request")
	}
}

func TestPanel_View_RendersToolNameAndArgs(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	p.Show(mkRequest("read_file", `{"path":"foo.go","start_line":1,"end_line":10}`))
	v := ansi.Strip(p.View())
	if !strings.Contains(v, "read_file") {
		t.Errorf("view missing tool name:\n%s", v)
	}
	if !strings.Contains(v, "foo.go") {
		t.Errorf("view missing pretty-printed arg:\n%s", v)
	}
	if !strings.Contains(v, "Permission Required") || !strings.Contains(v, "Tool") || !strings.Contains(v, "Path") {
		t.Errorf("view missing approval metadata:\n%s", v)
	}
	if !strings.Contains(v, "Allow") || !strings.Contains(v, "Allow for Session") || !strings.Contains(v, "Deny") {
		t.Errorf("view missing one of the three options:\n%s", v)
	}
}

func TestPanel_View_HeaderAndButtonsDoNotWrapAtNormalWidth(t *testing.T) {
	p := NewPanel()
	p.SetSize(120, 24)
	p.Show(mkRequest("read_file", `{"path":"README.md"}`))

	lines := strings.Split(ansi.Strip(p.View()), "\n")
	var headerLine, buttonLine string
	for _, line := range lines {
		if strings.Contains(line, "Permission Required") {
			headerLine = line
		}
		if strings.Contains(line, "Allow") && strings.Contains(line, "Deny") {
			buttonLine = line
		}
	}
	if !strings.Contains(headerLine, "=") {
		t.Fatalf("header should keep fill on the title line, got:\n%s", strings.Join(lines, "\n"))
	}
	if buttonLine == "" {
		t.Fatalf("buttons should render on one row, got:\n%s", strings.Join(lines, "\n"))
	}
}

func TestPanel_View_HiddenIsEmpty(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	if p.View() != "" {
		t.Errorf("hidden panel View should be empty, got %q", p.View())
	}
}

func TestPanel_DigitKeysMapToDecisions(t *testing.T) {
	cases := []struct {
		key  string
		want DecisionKind
	}{
		{"1", DecisionAllowOnce},
		{"2", DecisionAllowSession},
		{"3", DecisionDeny},
		{"y", DecisionAllowOnce},
		{"a", DecisionAllowSession},
		{"s", DecisionAllowSession},
		{"d", DecisionDeny},
		{"n", DecisionDeny},
	}
	for _, c := range cases {
		t.Run(c.key, func(t *testing.T) {
			p := NewPanel()
			p.SetSize(80, 24)
			p.Show(mkRequest("echo", `{}`))
			d := runKey(t, p, c.key)
			if d.Kind != c.want {
				t.Fatalf("key %q -> kind %d, want %d", c.key, d.Kind, c.want)
			}
		})
	}
}

func TestPanel_EscDenies(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	p.Show(mkRequest("echo", `{}`))
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEscape}))
	if cmd == nil {
		t.Fatal("Esc should produce a command")
	}
	msg := cmd()
	d, ok := msg.(DecisionMsg)
	if !ok {
		t.Fatalf("expected DecisionMsg, got %T", msg)
	}
	if d.Kind != DecisionDeny {
		t.Fatalf("Esc kind = %d, want Deny (%d)", d.Kind, DecisionDeny)
	}
}

func TestPanel_ArrowsMoveCursorEnterConfirms(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	p.Show(mkRequest("echo", `{}`))

	// Move cursor down twice (allow once -> allow session -> deny).
	if cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})); cmd != nil {
		t.Fatalf("down arrow should not emit a decision")
	}
	if cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown})); cmd != nil {
		t.Fatalf("down arrow should not emit a decision")
	}
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	if cmd == nil {
		t.Fatal("Enter should produce a decision")
	}
	d, ok := cmd().(DecisionMsg)
	if !ok {
		t.Fatalf("expected DecisionMsg, got %T", cmd())
	}
	if d.Kind != DecisionDeny {
		t.Fatalf("after 2x down + Enter, kind = %d, want Deny", d.Kind)
	}
}

func TestPanel_ArrowsClampAtBounds(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	p.Show(mkRequest("echo", `{}`))
	// Up at the top is a no-op.
	if cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyUp})); cmd != nil {
		t.Fatalf("up at top should be a no-op")
	}
	// Move past the bottom and confirm cursor doesn't overflow.
	for i := 0; i < 10; i++ {
		_ = p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyDown}))
	}
	cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: tea.KeyEnter}))
	d := cmd().(DecisionMsg)
	if d.Kind != DecisionDeny {
		t.Fatalf("over-scrolling should leave cursor at last option (Deny); got %d", d.Kind)
	}
}

func TestPanel_DecisionAttachesRequest(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	req := mkRequest("read_file", `{"path":"x"}`)
	p.Show(req)
	d := runKey(t, p, "1")
	if d.Request != req {
		t.Fatalf("DecisionMsg.Request = %p, want %p", d.Request, req)
	}
}

func TestPanel_HiddenPanelIgnoresKeys(t *testing.T) {
	p := NewPanel()
	p.SetSize(80, 24)
	if cmd := p.Update(tea.KeyPressMsg(tea.Key{Code: '1', Text: "1"})); cmd != nil {
		t.Fatalf("hidden panel should ignore keys, got cmd")
	}
}

func TestFormatArgs_PrettyPrintsValidJSON(t *testing.T) {
	out := formatArgs(json.RawMessage(`{"path":"foo","line":42}`))
	if !strings.Contains(out, "\"path\": \"foo\"") {
		t.Errorf("expected pretty-printed JSON, got %q", out)
	}
}

func TestFormatArgs_FallsBackOnInvalid(t *testing.T) {
	raw := `not json`
	if got := formatArgs(json.RawMessage(raw)); got != raw {
		t.Errorf("invalid JSON should pass through verbatim, got %q", got)
	}
}

func TestFormatArgs_EmptyInput(t *testing.T) {
	if got := formatArgs(nil); got != "" {
		t.Errorf("nil input should return empty string, got %q", got)
	}
}
