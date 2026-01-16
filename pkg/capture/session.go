package capture

import (
	"sync"
	"time"
)

// CommandRecord represents a single command execution
type CommandRecord struct {
	Command     string
	ExitCode    int
	StartTime   time.Time
	EndTime     time.Time
	WorkingDir  string
	BufferStart int // Position in buffer where this command's output starts
	BufferEnd   int // Position in buffer where this command's output ends
}

// SessionContext tracks the current terminal session state
type SessionContext struct {
	mu           sync.RWMutex
	history      []CommandRecord
	currentDir   string
	maxHistory   int // Maximum number of commands to keep
	sessionStart time.Time
}

// NewSessionContext creates a new session context tracker
func NewSessionContext() *SessionContext {
	return &SessionContext{
		history:      make([]CommandRecord, 0),
		currentDir:   "/", // Default to root, will be updated
		maxHistory:   1000,
		sessionStart: time.Now(),
	}
}

// AddCommand records a new command execution
func (sc *SessionContext) AddCommand(record CommandRecord) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	sc.history = append(sc.history, record)

	// Trim history if it exceeds max
	if len(sc.history) > sc.maxHistory {
		// Remove oldest entries
		sc.history = sc.history[len(sc.history)-sc.maxHistory:]
	}

	// Update current directory if it changed
	if record.WorkingDir != "" {
		sc.currentDir = record.WorkingDir
	}
}

// GetHistory returns all command records
func (sc *SessionContext) GetHistory() []CommandRecord {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	// Return a copy to avoid data races
	result := make([]CommandRecord, len(sc.history))
	copy(result, sc.history)
	return result
}

// GetLastN returns the last N command records
func (sc *SessionContext) GetLastN(n int) []CommandRecord {
	sc.mu.RLock()
	defer sc.mu.RUnlock()

	if n > len(sc.history) {
		n = len(sc.history)
	}

	if n <= 0 {
		return []CommandRecord{}
	}

	start := len(sc.history) - n
	result := make([]CommandRecord, n)
	copy(result, sc.history[start:])
	return result
}

// GetCurrentDir returns the current working directory
func (sc *SessionContext) GetCurrentDir() string {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return sc.currentDir
}

// SetCurrentDir updates the current working directory
func (sc *SessionContext) SetCurrentDir(dir string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.currentDir = dir
}

// GetSessionDuration returns how long the session has been active
func (sc *SessionContext) GetSessionDuration() time.Duration {
	return time.Since(sc.sessionStart)
}

// HistorySize returns the number of commands in history
func (sc *SessionContext) HistorySize() int {
	sc.mu.RLock()
	defer sc.mu.RUnlock()
	return len(sc.history)
}

// Clear resets the session context
func (sc *SessionContext) Clear() {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.history = make([]CommandRecord, 0)
}
