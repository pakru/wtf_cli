package ai

import (
	"strconv"
	"strings"
	"testing"
)

func TestStripANSICodes(t *testing.T) {
	input := "start\x1b[31mred\x1b[0m\x1b]0;title\x07end"
	output := stripANSICodes(input)
	if output != "startredend" {
		t.Fatalf("Expected stripped output, got %q", output)
	}
}

func TestBuildTerminalContext_MaxLines(t *testing.T) {
	lines := make([][]byte, 0, 150)
	for i := 0; i < 150; i++ {
		lines = append(lines, []byte("line-"+strconv.Itoa(i)))
	}

	ctx := BuildTerminalContext(lines, TerminalMetadata{})
	if !strings.HasPrefix(ctx.Output, "line-50") {
		t.Fatalf("Expected output to start with line-50, got %q", ctx.Output[:8])
	}
	if !strings.Contains(ctx.Output, "line-149") {
		t.Fatalf("Expected output to include line-149")
	}
}

func TestBuildTerminalContext_Truncate(t *testing.T) {
	lines := make([][]byte, 0, 120)
	for i := 0; i < 120; i++ {
		lines = append(lines, []byte(strings.Repeat("x", 200)))
	}

	ctx := BuildTerminalContext(lines, TerminalMetadata{})
	if !ctx.Truncated {
		t.Fatalf("Expected context to be truncated")
	}
	if !strings.HasPrefix(ctx.Output, "[truncated]\n") {
		t.Fatalf("Expected truncation prefix, got %q", ctx.Output)
	}
	if len(ctx.Output) > DefaultContextBytes {
		t.Fatalf("Expected output <= %d bytes, got %d", DefaultContextBytes, len(ctx.Output))
	}
}

func TestBuildWtfMessages_IncludesMetadata(t *testing.T) {
	lines := [][]byte{[]byte("error: something failed")}
	meta := TerminalMetadata{
		WorkingDir:  "/tmp",
		LastCommand: "make build",
		ExitCode:    2,
	}

	messages, ctx := BuildWtfMessages(lines, meta)
	if len(messages) != 2 {
		t.Fatalf("Expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "system" || messages[1].Role != "user" {
		t.Fatalf("Expected system and user roles, got %q and %q", messages[0].Role, messages[1].Role)
	}
	if !strings.Contains(ctx.UserPrompt, "cwd: /tmp") {
		t.Fatalf("Expected working dir in prompt, got %q", ctx.UserPrompt)
	}
	if !strings.Contains(ctx.UserPrompt, "last_command: make build") {
		t.Fatalf("Expected command in prompt, got %q", ctx.UserPrompt)
	}
	if !strings.Contains(ctx.UserPrompt, "last_exit_code: 2") {
		t.Fatalf("Expected exit code in prompt, got %q", ctx.UserPrompt)
	}
	if !strings.Contains(ctx.UserPrompt, "error: something failed") {
		t.Fatalf("Expected output in prompt, got %q", ctx.UserPrompt)
	}
}
