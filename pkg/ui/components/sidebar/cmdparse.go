package sidebar

import "strings"

const (
	cmdOpenTag  = "<cmd>"
	cmdCloseTag = "</cmd>"
)

// CommandEntry stores an extracted command and its source index in raw content.
type CommandEntry struct {
	Command     string
	SourceIndex int
}

// ExtractCommands parses commands wrapped in <cmd>...</cmd> markers.
func ExtractCommands(content string) []CommandEntry {
	if content == "" {
		return nil
	}

	entries := make([]CommandEntry, 0, 2)
	searchStart := 0

	for searchStart < len(content) {
		openRel := strings.Index(content[searchStart:], cmdOpenTag)
		if openRel < 0 {
			break
		}
		open := searchStart + openRel
		cmdStart := open + len(cmdOpenTag)

		closeRel := strings.Index(content[cmdStart:], cmdCloseTag)
		if closeRel < 0 {
			break
		}
		close := cmdStart + closeRel

		entries = append(entries, CommandEntry{
			Command:     content[cmdStart:close],
			SourceIndex: cmdStart,
		})

		searchStart = close + len(cmdCloseTag)
	}

	return entries
}

// StripCommandMarkers removes <cmd> markers while preserving command text.
func StripCommandMarkers(content string) string {
	if content == "" {
		return content
	}
	replacer := strings.NewReplacer(cmdOpenTag, "", cmdCloseTag, "")
	return replacer.Replace(content)
}

// SanitizeCommand validates a command before sending it to PTY input.
func SanitizeCommand(cmd string) (string, bool) {
	trimmed := strings.TrimSpace(cmd)
	if trimmed == "" {
		return "", false
	}
	if strings.ContainsAny(trimmed, "\n\r") {
		return "", false
	}
	return trimmed, true
}
