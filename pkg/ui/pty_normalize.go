package ui

import (
	"bytes"
	"strings"
)

const tabWidth = 4

var tabSpaces = []byte(strings.Repeat(" ", tabWidth))

func appendPTYContent(content string, data []byte, pendingCR *bool) string {
	if len(data) == 0 {
		return content
	}

	buf := []byte(content)

	for _, b := range data {
		if pendingCR != nil && *pendingCR {
			if b == '\n' {
				buf = append(buf, '\n')
				*pendingCR = false
				continue
			}
			if b == '\r' {
				continue
			}
			buf = trimToLineStart(buf)
			*pendingCR = false
		}

		switch b {
		case '\r':
			if pendingCR != nil {
				*pendingCR = true
			}
		case '\n':
			buf = append(buf, '\n')
		case '\t':
			buf = append(buf, tabSpaces...)
		default:
			buf = append(buf, b)
		}
	}

	return string(buf)
}

func trimToLineStart(buf []byte) []byte {
	idx := bytes.LastIndexByte(buf, '\n')
	if idx == -1 {
		return buf[:0]
	}
	return buf[:idx+1]
}
