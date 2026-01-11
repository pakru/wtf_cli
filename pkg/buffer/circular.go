package buffer

import (
	"sync"
)

// CircularBuffer is a thread-safe ring buffer for storing terminal output
type CircularBuffer struct {
	mu       sync.RWMutex
	data     [][]byte // Store as slices of bytes (lines)
	capacity int      // Maximum number of lines
	size     int      // Current number of lines
	head     int      // Write position
}

// New creates a new circular buffer with the specified capacity (in lines)
func New(capacity int) *CircularBuffer {
	if capacity <= 0 {
		capacity = 2000 // Default capacity
	}
	
	return &CircularBuffer{
		data:     make([][]byte, capacity),
		capacity: capacity,
		size:     0,
		head:     0,
	}
}

// Write adds a line to the buffer
func (cb *CircularBuffer) Write(line []byte) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	// Make a copy of the line to avoid external modifications
	lineCopy := make([]byte, len(line))
	copy(lineCopy, line)

	cb.data[cb.head] = lineCopy
	cb.head = (cb.head + 1) % cb.capacity

	if cb.size < cb.capacity {
		cb.size++
	}
}

// GetLastN retrieves the last N lines from the buffer
// Returns fewer lines if buffer contains fewer than N lines
func (cb *CircularBuffer) GetLastN(n int) [][]byte {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if n <= 0 {
		return [][]byte{}
	}

	if n > cb.size {
		n = cb.size
	}

	result := make([][]byte, n)
	
	// Calculate start position
	start := (cb.head - n + cb.capacity) % cb.capacity

	for i := 0; i < n; i++ {
		pos := (start + i) % cb.capacity
		// Copy to prevent external modifications
		result[i] = make([]byte, len(cb.data[pos]))
		copy(result[i], cb.data[pos])
	}

	return result
}

// GetAll retrieves all lines currently in the buffer
func (cb *CircularBuffer) GetAll() [][]byte {
	return cb.GetLastN(cb.Size())
}

// Size returns the current number of lines in the buffer
func (cb *CircularBuffer) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}

// Capacity returns the maximum capacity of the buffer
func (cb *CircularBuffer) Capacity() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.capacity
}

// Clear empties the buffer
func (cb *CircularBuffer) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.data = make([][]byte, cb.capacity)
	cb.size = 0
	cb.head = 0
}

// ExportAsText returns all buffer contents as a single string
// with lines separated by newlines
func (cb *CircularBuffer) ExportAsText() string {
	lines := cb.GetAll()
	
	if len(lines) == 0 {
		return ""
	}

	// Calculate total size
	totalSize := 0
	for _, line := range lines {
		totalSize += len(line) + 1 // +1 for newline
	}

	// Build string efficiently
	result := make([]byte, 0, totalSize)
	for i, line := range lines {
		result = append(result, line...)
		if i < len(lines)-1 {
			result = append(result, '\n')
		}
	}

	return string(result)
}

// ExportLastNAsText returns the last N lines as a single string
func (cb *CircularBuffer) ExportLastNAsText(n int) string {
	lines := cb.GetLastN(n)
	
	if len(lines) == 0 {
		return ""
	}

	// Calculate total size
	totalSize := 0
	for _, line := range lines {
		totalSize += len(line) + 1
	}

	// Build string
	result := make([]byte, 0, totalSize)
	for i, line := range lines {
		result = append(result, line...)
		if i < len(lines)-1 {
			result = append(result, '\n')
		}
	}

	return string(result)
}
