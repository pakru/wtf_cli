package capture

import "testing"

func TestExtractCommandFromPrompt_Dollar(t *testing.T) {
	line := "dev@host:~/project$ ifconfig"
	if got := ExtractCommandFromPrompt(line); got != "ifconfig" {
		t.Fatalf("expected %q, got %q", "ifconfig", got)
	}
}

func TestExtractCommandFromPrompt_Root(t *testing.T) {
	line := "root@host:/# ls -la"
	if got := ExtractCommandFromPrompt(line); got != "ls -la" {
		t.Fatalf("expected %q, got %q", "ls -la", got)
	}
}

func TestExtractCommandFromPrompt_Empty(t *testing.T) {
	line := "dev@host:~/project$ "
	if got := ExtractCommandFromPrompt(line); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestExtractCommandFromPrompt_NoPrompt(t *testing.T) {
	line := "not a prompt line"
	if got := ExtractCommandFromPrompt(line); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
