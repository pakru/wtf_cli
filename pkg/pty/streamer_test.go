package pty

import (
	"bytes"
	"testing"
	"time"
)

func TestSpawnShellWithBuffer(t *testing.T) {
	bw, err := SpawnShellWithBuffer(100)
	if err != nil {
		t.Fatalf("SpawnShellWithBuffer() failed: %v", err)
	}
	defer bw.Close()

	if bw.buffer == nil {
		t.Error("Expected buffer to be initialized")
	}

	if bw.buffer.Capacity() != 100 {
		t.Errorf("Expected buffer capacity 100, got %d", bw.buffer.Capacity())
	}
}

func TestGetBuffer(t *testing.T) {
	bw, err := SpawnShellWithBuffer(50)
	if err != nil {
		t.Fatalf("SpawnShellWithBuffer() failed: %v", err)
	}
	defer bw.Close()

	buf := bw.GetBuffer()
	if buf == nil {
		t.Error("GetBuffer() returned nil")
	}

	if buf.Capacity() != 50 {
		t.Errorf("Expected capacity 50, got %d", buf.Capacity())
	}
}

func TestProxyIOWithBuffer_Capture(t *testing.T) {
	bw, err := SpawnShellWithBuffer(100)
	if err != nil {
		t.Fatalf("SpawnShellWithBuffer() failed: %v", err)
	}
	defer bw.Close()

	// This test verifies the buffer is accessible and starts empty
	// Full I/O testing would require PTY interaction which is complex

	if bw.GetBuffer().Size() != 0 {
		t.Errorf("Expected empty buffer initially, got %d lines", bw.GetBuffer().Size())
	}
}

func TestBufferWriteAsync(t *testing.T) {
	bw, err := SpawnShellWithBuffer(100)
	if err != nil {
		t.Fatalf("SpawnShellWithBuffer() failed: %v", err)
	}
	defer bw.Close()

	// Simulate buffer writes (as would happen during I/O)
	testLines := [][]byte{
		[]byte("line 1"),
		[]byte("line 2"),
		[]byte("line 3"),
	}

	for _, line := range testLines {
		bw.buffer.Write(line)
	}

	// Allow goroutines to complete
	time.Sleep(10 * time.Millisecond)

	if bw.buffer.Size() != 3 {
		t.Errorf("Expected 3 lines in buffer, got %d", bw.buffer.Size())
	}

	lines := bw.buffer.GetAll()
	for i, expected := range testLines {
		if !bytes.Equal(lines[i], expected) {
			t.Errorf("Line %d: expected %q, got %q", i, expected, lines[i])
		}
	}
}

func TestBufferPreservesANSI(t *testing.T) {
	bw, err := SpawnShellWithBuffer(10)
	if err != nil {
		t.Fatalf("SpawnShellWithBuffer() failed: %v", err)
	}
	defer bw.Close()

	// Test ANSI preservation through buffer
	ansiLine := []byte("\033[1;31mRed\033[0m text")
	bw.buffer.Write(ansiLine)

	lines := bw.buffer.GetAll()
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lines))
	}

	if !bytes.Equal(lines[0], ansiLine) {
		t.Errorf("ANSI codes not preserved: expected %q, got %q", ansiLine, lines[0])
	}
}
