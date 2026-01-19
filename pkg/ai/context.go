package ai

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultContextLines = 100
	DefaultContextBytes = 12000
)

// TerminalMetadata captures shell context for LLM requests.
type TerminalMetadata struct {
	WorkingDir  string
	LastCommand string
	ExitCode    int
}

// TerminalContext contains the assembled prompts and output.
type TerminalContext struct {
	Output       string
	LineCount    int
	Truncated    bool
	SystemPrompt string
	UserPrompt   string
}

// BuildTerminalContext assembles prompts and sanitized output.
func BuildTerminalContext(lines [][]byte, meta TerminalMetadata) TerminalContext {
	limited := limitLines(lines, DefaultContextLines)
	output := sanitizeLines(limited)
	output, truncated := truncateOutput(output, DefaultContextBytes)

	ctx := TerminalContext{
		Output:       output,
		LineCount:    len(limited),
		Truncated:    truncated,
		SystemPrompt: wtfSystemPrompt(),
	}
	ctx.UserPrompt = buildUserPrompt(meta, ctx)

	return ctx
}

// BuildWtfMessages builds system/user messages for the /explain command.
func BuildWtfMessages(lines [][]byte, meta TerminalMetadata) ([]Message, TerminalContext) {
	ctx := BuildTerminalContext(lines, meta)
	messages := []Message{
		{Role: "system", Content: ctx.SystemPrompt},
		{Role: "user", Content: ctx.UserPrompt},
	}
	return messages, ctx
}

func limitLines(lines [][]byte, maxLines int) [][]byte {
	if maxLines <= 0 || len(lines) <= maxLines {
		return lines
	}
	return lines[len(lines)-maxLines:]
}

func sanitizeLines(lines [][]byte) string {
	if len(lines) == 0 {
		return ""
	}

	var sb strings.Builder
	for i, line := range lines {
		clean := stripANSICodes(string(line))
		clean = strings.ToValidUTF8(clean, "")
		sb.WriteString(clean)
		if i < len(lines)-1 {
			sb.WriteByte('\n')
		}
	}

	return sb.String()
}

func truncateOutput(output string, maxBytes int) (string, bool) {
	if maxBytes <= 0 || len(output) <= maxBytes {
		return output, false
	}

	const prefix = "[truncated]\n"
	if maxBytes <= len(prefix) {
		return prefix[:maxBytes], true
	}

	keepBytes := maxBytes - len(prefix)
	start := len(output) - keepBytes
	if start < 0 {
		start = 0
	}
	for start < len(output) && !utf8.ValidString(output[start:]) {
		start++
	}

	return prefix + output[start:], true
}

func buildUserPrompt(meta TerminalMetadata, ctx TerminalContext) string {
	workingDir := strings.TrimSpace(meta.WorkingDir)
	lastCommand := strings.TrimSpace(meta.LastCommand)
	output := ctx.Output
	if strings.TrimSpace(output) == "" {
		output = "<no output captured>"
	}

	var sb strings.Builder
	sb.WriteString("Please help diagnose the terminal issue and suggest fixes.\n")
	sb.WriteString("Terminal metadata (captured fields):\n")
	if workingDir != "" {
		sb.WriteString(fmt.Sprintf("cwd: %s\n", workingDir))
	}
	if lastCommand != "" {
		sb.WriteString(fmt.Sprintf("last_command: %s\n", lastCommand))
	}
	if meta.ExitCode >= 0 {
		sb.WriteString(fmt.Sprintf("last_exit_code: %d\n", meta.ExitCode))
	}
	sb.WriteString(fmt.Sprintf("output_lines: %d\n", ctx.LineCount))
	if ctx.Truncated {
		sb.WriteString("note: output truncated\n")
	}
	sb.WriteString("\nRecent output (most recent lines, oldest -> newest):\n")
	sb.WriteString(output)

	return sb.String()
}

func wtfSystemPrompt() string {
	return strings.Join([]string{
		"You are a terminal assistant.",
		"Use the provided terminal output and metadata to diagnose issues.",
		"If last_command is provided, focus on that command and its output first.",
		"If a metadata field is missing, do not assume or invent it.",
		"Field definitions: cwd is the current working directory; last_command is the most recent captured command; last_exit_code is the exit code for last_command; output_lines is the number of lines in the output block; output may be truncated when noted.",
		"Provide concise, actionable suggestions and likely causes.",
		"If you need more information, ask focused questions.",
	}, " ")
}

func stripANSICodes(s string) string {
	if s == "" {
		return ""
	}

	var sb strings.Builder
	sb.Grow(len(s))

	for i := 0; i < len(s); i++ {
		ch := s[i]
		if ch == 0x1b { // ESC
			if i+1 >= len(s) {
				continue
			}
			next := s[i+1]
			switch next {
			case '[': // CSI
				i += 2
				for i < len(s) {
					ch = s[i]
					if ch >= 0x40 && ch <= 0x7E {
						break
					}
					i++
				}
			case ']': // OSC
				i += 2
				for i < len(s) {
					if s[i] == 0x07 {
						break
					}
					if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
						i++
						break
					}
					i++
				}
			default:
				i++
			}
			continue
		}

		if ch == '\r' {
			sb.WriteByte('\n')
			continue
		}
		if ch < 0x20 && ch != '\n' && ch != '\t' {
			continue
		}
		sb.WriteByte(ch)
	}

	return sb.String()
}
