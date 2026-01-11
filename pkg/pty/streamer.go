package pty

import (
	"bufio"
	"io"
	"os"

	"wtf_cli/pkg/buffer"
)

// BufferedWrapper extends Wrapper with output buffering capabilities
type BufferedWrapper struct {
	*Wrapper
	buffer *buffer.CircularBuffer
}

// SpawnShellWithBuffer creates a new PTY with output buffering
func SpawnShellWithBuffer(bufferSize int) (*BufferedWrapper, error) {
	wrapper, err := SpawnShell()
	if err != nil {
		return nil, err
	}

	return &BufferedWrapper{
		Wrapper: wrapper,
		buffer:  buffer.New(bufferSize),
	}, nil
}

// ProxyIOWithBuffer handles bidirectional I/O and captures output to buffer
func (bw *BufferedWrapper) ProxyIOWithBuffer() error {
	// Copy stdin to PTY (unchanged)
	go func() {
		io.Copy(bw.ptmx, os.Stdin)
	}()

	// Copy PTY to stdout AND buffer
	// Use a scanner to split output into lines for buffer
	scanner := bufio.NewScanner(bw.ptmx)
	
	for scanner.Scan() {
		line := scanner.Bytes()
		
		// Write to buffer (in goroutine to avoid blocking)
		lineCopy := make([]byte, len(line))
		copy(lineCopy, line)
		go bw.buffer.Write(lineCopy)
		
		// Write to stdout (with newline that scanner consumed)
		os.Stdout.Write(line)
		os.Stdout.Write([]byte("\n"))
	}

	return scanner.Err()
}

// GetBuffer returns the circular buffer for reading captured output
func (bw *BufferedWrapper) GetBuffer() *buffer.CircularBuffer {
	return bw.buffer
}
