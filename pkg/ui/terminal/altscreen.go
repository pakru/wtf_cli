// Package terminal provides terminal escape sequence handling.
// This includes detection of alternate screen buffer enter/exit sequences
// used by full-screen applications like vim, nano, htop, and less.
package terminal

import "bytes"

// Alternate screen buffer escape sequences
// Used by full-screen applications like vim, nano, htop, less
var (
	// smcup - Enter alternate screen buffer
	// Various forms used by different terminal types
	altScreenEnterSeqs = [][]byte{
		[]byte("\x1b[?1049h"),             // xterm
		[]byte("\x1b[?1047h"),             // alternate screen (xterm)
		[]byte("\x1b[?47h"),               // older xterm
		[]byte("\x1b7\x1b[?47h"),          // with cursor save
		[]byte("\x1b[?1049h\x1b[22;0;0t"), // with title save
	}

	// rmcup - Exit alternate screen buffer
	altScreenExitSeqs = [][]byte{
		[]byte("\x1b[?1049l"),             // xterm
		[]byte("\x1b[?1047l"),             // alternate screen (xterm)
		[]byte("\x1b[?47l"),               // older xterm
		[]byte("\x1b[?47l\x1b8"),          // with cursor restore
		[]byte("\x1b[?1049l\x1b[23;0;0t"), // with title restore
	}
)

var maxAltScreenSeqLen = maxAltScreenSequenceLength()

// AltScreenChunk represents a chunk of PTY data with transition info
type AltScreenChunk struct {
	Data     []byte
	Entering bool
	Exiting  bool
}

// AltScreenState tracks the alternate screen buffer detection state
type AltScreenState struct {
	// pending holds bytes that might be part of an incomplete escape sequence
	pending []byte
}

// NewAltScreenState creates a new alternate screen state tracker
func NewAltScreenState() *AltScreenState {
	return &AltScreenState{
		pending: make([]byte, 0, 32),
	}
}

// DetectAltScreen checks if the data contains alternate screen enter/exit sequences.
// Returns (entering, exiting) flags.
// This handles sequences that may be split across multiple read boundaries.
func (s *AltScreenState) DetectAltScreen(data []byte) (entering bool, exiting bool) {
	// Combine pending bytes with new data for detection
	combined := data
	if len(s.pending) > 0 {
		combined = append(s.pending, data...)
		s.pending = s.pending[:0]
	}

	// Check for entering sequences
	for _, seq := range altScreenEnterSeqs {
		if bytes.Contains(combined, seq) {
			entering = true
			break
		}
	}

	// Check for exiting sequences
	for _, seq := range altScreenExitSeqs {
		if bytes.Contains(combined, seq) {
			exiting = true
			break
		}
	}

	// If data ends with partial escape sequence, save it for next call
	// Look for incomplete ESC sequence at end
	if len(combined) > 0 {
		for i := len(combined) - 1; i >= 0 && i >= len(combined)-20; i-- {
			if combined[i] == 0x1b { // ESC
				// Check if this could be start of an incomplete sequence
				remaining := combined[i:]
				if !isCompleteEscapeSequence(remaining) {
					s.pending = append(s.pending, remaining...)
					break
				}
			}
		}
	}

	return entering, exiting
}

// SplitTransitions returns data chunks split by alternate screen enter/exit sequences.
// Sequence chunks are returned with entering/exiting flags and should be handled
// with the transition-aware routing logic.
func (s *AltScreenState) SplitTransitions(data []byte) []AltScreenChunk {
	combined := data
	if len(s.pending) > 0 {
		combined = append(s.pending, data...)
		s.pending = s.pending[:0]
	}

	pendingLen := trailingAltScreenPrefixLen(combined)
	if pendingLen > 0 {
		s.pending = append(s.pending, combined[len(combined)-pendingLen:]...)
		combined = combined[:len(combined)-pendingLen]
	}

	if len(combined) == 0 {
		return nil
	}

	chunks := make([]AltScreenChunk, 0, 4)
	offset := 0
	for offset < len(combined) {
		match, found := findNextAltScreenMatch(combined[offset:])
		if !found {
			chunks = append(chunks, AltScreenChunk{Data: combined[offset:]})
			break
		}

		if match.idx > 0 {
			chunks = append(chunks, AltScreenChunk{Data: combined[offset : offset+match.idx]})
		}

		seqStart := offset + match.idx
		seqEnd := seqStart + len(match.seq)
		chunks = append(chunks, AltScreenChunk{
			Data:     combined[seqStart:seqEnd],
			Entering: match.entering,
			Exiting:  match.exiting,
		})
		offset = seqEnd
	}

	return chunks
}

// DetectAltScreenSimple is a stateless version for simple detection.
// Use DetectAltScreen with AltScreenState for handling split sequences.
func DetectAltScreenSimple(data []byte) (entering bool, exiting bool) {
	for _, seq := range altScreenEnterSeqs {
		if bytes.Contains(data, seq) {
			entering = true
			break
		}
	}

	for _, seq := range altScreenExitSeqs {
		if bytes.Contains(data, seq) {
			exiting = true
			break
		}
	}

	return entering, exiting
}

type altScreenMatch struct {
	idx      int
	seq      []byte
	entering bool
	exiting  bool
}

func findNextAltScreenMatch(data []byte) (altScreenMatch, bool) {
	bestIdx := -1
	bestLen := 0
	var best altScreenMatch

	check := func(seq []byte, entering bool) {
		if idx := bytes.Index(data, seq); idx >= 0 {
			if bestIdx == -1 || idx < bestIdx || (idx == bestIdx && len(seq) > bestLen) {
				bestIdx = idx
				bestLen = len(seq)
				best = altScreenMatch{
					idx:      idx,
					seq:      seq,
					entering: entering,
					exiting:  !entering,
				}
			}
		}
	}

	for _, seq := range altScreenEnterSeqs {
		check(seq, true)
	}
	for _, seq := range altScreenExitSeqs {
		check(seq, false)
	}

	if bestIdx == -1 {
		return altScreenMatch{}, false
	}

	return best, true
}

func trailingAltScreenPrefixLen(data []byte) int {
	if len(data) == 0 || maxAltScreenSeqLen == 0 {
		return 0
	}

	maxCheck := len(data)
	if maxAltScreenSeqLen-1 < maxCheck {
		maxCheck = maxAltScreenSeqLen - 1
	}

	for length := maxCheck; length > 0; length-- {
		suffix := data[len(data)-length:]
		if isAltScreenPrefix(suffix) {
			return length
		}
	}

	return 0
}

func isAltScreenPrefix(suffix []byte) bool {
	if len(suffix) == 0 {
		return false
	}

	hasPrefix := false
	for _, seq := range altScreenEnterSeqs {
		if bytes.Equal(seq, suffix) {
			return false
		}
		if len(suffix) < len(seq) && bytes.HasPrefix(seq, suffix) {
			hasPrefix = true
		}
	}
	for _, seq := range altScreenExitSeqs {
		if bytes.Equal(seq, suffix) {
			return false
		}
		if len(suffix) < len(seq) && bytes.HasPrefix(seq, suffix) {
			hasPrefix = true
		}
	}

	return hasPrefix
}

func maxAltScreenSequenceLength() int {
	maxLen := 0
	for _, seq := range altScreenEnterSeqs {
		if len(seq) > maxLen {
			maxLen = len(seq)
		}
	}
	for _, seq := range altScreenExitSeqs {
		if len(seq) > maxLen {
			maxLen = len(seq)
		}
	}
	return maxLen
}

// isCompleteEscapeSequence checks if the bytes form a complete escape sequence
func isCompleteEscapeSequence(data []byte) bool {
	if len(data) == 0 || data[0] != 0x1b {
		return true // Not an escape sequence
	}

	if len(data) == 1 {
		return false // Just ESC, incomplete
	}

	// CSI sequences end with a letter
	if data[1] == '[' {
		for i := 2; i < len(data); i++ {
			if data[i] >= 0x40 && data[i] <= 0x7e {
				return true // Found terminator
			}
		}
		return false // No terminator found
	}

	// Other escape sequences (ESC followed by single char)
	return len(data) >= 2
}

// Reset clears any pending state
func (s *AltScreenState) Reset() {
	s.pending = s.pending[:0]
}
