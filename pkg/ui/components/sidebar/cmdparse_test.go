package sidebar

import "testing"

func TestExtractCommands_Single(t *testing.T) {
	content := "Try <cmd>ls -la</cmd> first."
	entries := ExtractCommands(content)

	if len(entries) != 1 {
		t.Fatalf("Expected 1 command, got %d", len(entries))
	}
	if entries[0].Command != "ls -la" {
		t.Fatalf("Expected command %q, got %q", "ls -la", entries[0].Command)
	}
	if entries[0].SourceIndex < 0 || entries[0].SourceIndex >= len(content) {
		t.Fatalf("Unexpected source index: %d", entries[0].SourceIndex)
	}
}

func TestExtractCommands_Multiple(t *testing.T) {
	content := "<cmd>pwd</cmd> then <cmd>git status</cmd>"
	entries := ExtractCommands(content)

	if len(entries) != 2 {
		t.Fatalf("Expected 2 commands, got %d", len(entries))
	}
	if entries[0].Command != "pwd" {
		t.Fatalf("Expected first command %q, got %q", "pwd", entries[0].Command)
	}
	if entries[1].Command != "git status" {
		t.Fatalf("Expected second command %q, got %q", "git status", entries[1].Command)
	}
}

func TestExtractCommands_None(t *testing.T) {
	entries := ExtractCommands("No command markers here")
	if len(entries) != 0 {
		t.Fatalf("Expected 0 commands, got %d", len(entries))
	}
}

func TestExtractCommands_Malformed(t *testing.T) {
	content := "open <cmd>ls -la and <cmd>pwd</cmd>"
	entries := ExtractCommands(content)

	if len(entries) != 1 {
		t.Fatalf("Expected 1 valid command, got %d", len(entries))
	}
	if entries[0].Command != "ls -la and <cmd>pwd" {
		t.Fatalf("Unexpected parsed command: %q", entries[0].Command)
	}
}

func TestStripCommandMarkers(t *testing.T) {
	content := "Use <cmd>ls -la</cmd> and <cmd>pwd</cmd>."
	got := StripCommandMarkers(content)
	want := "Use ls -la and pwd."

	if got != want {
		t.Fatalf("Expected %q, got %q", want, got)
	}
}

func TestSanitizeCommand(t *testing.T) {
	cases := []struct {
		name  string
		input string
		want  string
		ok    bool
	}{
		{name: "trimmed", input: "  ls -la  ", want: "ls -la", ok: true},
		{name: "empty", input: "   ", want: "", ok: false},
		{name: "multiline_lf", input: "ls\n-la", want: "", ok: false},
		{name: "multiline_cr", input: "ls\r-la", want: "", ok: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := SanitizeCommand(tc.input)
			if ok != tc.ok {
				t.Fatalf("Expected ok=%v, got %v", tc.ok, ok)
			}
			if got != tc.want {
				t.Fatalf("Expected %q, got %q", tc.want, got)
			}
		})
	}
}
