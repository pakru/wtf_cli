package pty

import (
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

// lineWriter writes complete lines to the buffer
type lineWriter struct {
	buffer      *buffer.CircularBuffer
	currentLine []byte
}

func (lw *lineWriter) Write(p []byte) (n int, err error) {
	for _, b := range p {
		if b == '\n' {
			// Complete line - write to buffer
			if len(lw.currentLine) > 0 {
				lw.buffer.Write(append([]byte(nil), lw.currentLine...))
				lw.currentLine = lw.currentLine[:0]
			}
		} else {
			lw.currentLine = append(lw.currentLine, b)
		}
	}
	return len(p), nil
}

// ProxyIOWithBuffer handles bidirectional I/O and captures output to buffer
func (bw *BufferedWrapper) ProxyIOWithBuffer() error {
	// Copy stdin to PTY (unchanged)
	go func() {
		io.Copy(bw.ptmx, os.Stdin)
	}()

	// Create a line writer that buffers output
	lw := &lineWriter{buffer: bw.buffer}

	// Tee PTY output to both stdout AND line writer (for buffer)
	tee := io.TeeReader(bw.ptmx, lw)

	// Copy to stdout - this provides real-time output
	io.Copy(os.Stdout, tee)

	return nil
}

// GetBuffer returns the circular buffer for reading captured output
func (bw *BufferedWrapper) GetBuffer() *buffer.CircularBuffer {
	return bw.buffer
}
