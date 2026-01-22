package capture

import "strings"

// ExtractCommandFromPrompt attempts to extract a command from a prompt line.
// It supports common Bash/Zsh prompt delimiters like "$ " and "# ".
func ExtractCommandFromPrompt(line string) string {
	text := strings.TrimSpace(line)
	if text == "" {
		return ""
	}

	delim := strings.LastIndex(text, "$ ")
	if delim == -1 {
		delim = strings.LastIndex(text, "# ")
	}
	if delim == -1 {
		return ""
	}

	cmd := strings.TrimSpace(text[delim+2:])
	return cmd
}
