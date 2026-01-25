package statusbar

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func truncatePath(path string, maxWidth int) string {
	if maxWidth <= 0 {
		return ""
	}
	if ansi.StringWidth(path) <= maxWidth {
		return path
	}
	if path == "" {
		return ""
	}
	if path == "/" || path == "~" {
		if ansi.StringWidth(path) <= maxWidth {
			return path
		}
		return ansi.Truncate(path, maxWidth, "")
	}

	rest := path
	prefix := ""
	isHome := false
	isAbs := false

	if strings.HasPrefix(path, "~") {
		isHome = true
		prefix = "~"
		rest = strings.TrimPrefix(path, "~")
		rest = strings.TrimPrefix(rest, "/")
	} else if strings.HasPrefix(path, "/") {
		isAbs = true
		prefix = "/"
		rest = strings.TrimPrefix(path, "/")
	}

	if rest == "" {
		if prefix != "" {
			if ansi.StringWidth(prefix) <= maxWidth {
				return prefix
			}
			return ansi.Truncate(prefix, maxWidth, "..")
		}
		return ansi.Truncate(path, maxWidth, "..")
	}

	segments := strings.Split(rest, "/")
	if len(segments) == 0 {
		return ansi.Truncate(path, maxWidth, "..")
	}

	prefixSeg := ""
	trailing := segments
	if isAbs {
		prefixSeg = prefix + segments[0]
		if len(segments) > 1 {
			trailing = segments[1:]
		} else {
			trailing = nil
		}
	} else if isHome {
		prefixSeg = prefix
		trailing = segments
	} else {
		prefixSeg = segments[0]
		if len(segments) > 1 {
			trailing = segments[1:]
		} else {
			trailing = nil
		}
	}

	if len(trailing) == 0 {
		if ansi.StringWidth(prefixSeg) <= maxWidth {
			return prefixSeg
		}
		return ansi.Truncate(prefixSeg, maxWidth, "..")
	}

	maxTail := 3
	if len(trailing) < maxTail {
		maxTail = len(trailing)
	}
	for n := maxTail; n >= 1; n-- {
		tail := strings.Join(trailing[len(trailing)-n:], "/")
		candidate := prefixSeg + "/../" + tail
		if ansi.StringWidth(candidate) <= maxWidth {
			return candidate
		}
	}

	tail := trailing[len(trailing)-1]
	prefixPart := prefixSeg + "/../"
	avail := maxWidth - ansi.StringWidth(prefixPart)
	if avail <= 0 {
		if ansi.StringWidth(prefixSeg) <= maxWidth {
			return prefixSeg
		}
		return ansi.Truncate(prefixSeg, maxWidth, "..")
	}

	truncatedTail := ansi.Truncate(tail, avail, "..")
	return prefixPart + truncatedTail
}
