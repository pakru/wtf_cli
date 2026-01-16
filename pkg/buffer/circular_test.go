package buffer

import (
	"bytes"
	"sync"
	"testing"
)

func TestNew(t *testing.T) {
	cb := New(100)

	if cb.Capacity() != 100 {
		t.Errorf("Expected capacity 100, got %d", cb.Capacity())
	}

	if cb.Size() != 0 {
		t.Errorf("Expected size 0, got %d", cb.Size())
	}
}

func TestNew_DefaultCapacity(t *testing.T) {
	cb := New(0)

	if cb.Capacity() != 2000 {
		t.Errorf("Expected default capacity 2000, got %d", cb.Capacity())
	}
}

func TestWrite_Single(t *testing.T) {
	cb := New(10)

	line := []byte("test line")
	cb.Write(line)

	if cb.Size() != 1 {
		t.Errorf("Expected size 1, got %d", cb.Size())
	}

	lines := cb.GetAll()
	if len(lines) != 1 {
		t.Fatalf("Expected 1 line, got %d", len(lines))
	}

	if !bytes.Equal(lines[0], line) {
		t.Errorf("Expected %q, got %q", line, lines[0])
	}
}

func TestWrite_WrapAround(t *testing.T) {
	cb := New(3) // Small buffer to test wrap-around

	cb.Write([]byte("line1"))
	cb.Write([]byte("line2"))
	cb.Write([]byte("line3"))
	cb.Write([]byte("line4")) // This should overwrite line1

	if cb.Size() != 3 {
		t.Errorf("Expected size 3, got %d", cb.Size())
	}

	lines := cb.GetAll()
	expected := []string{"line2", "line3", "line4"}

	for i, exp := range expected {
		if string(lines[i]) != exp {
			t.Errorf("Line %d: expected %q, got %q", i, exp, string(lines[i]))
		}
	}
}

func TestGetLastN(t *testing.T) {
	cb := New(10)

	for i := 1; i <= 5; i++ {
		cb.Write([]byte{byte('0' + i)})
	}

	// Get last 3 lines
	lines := cb.GetLastN(3)

	if len(lines) != 3 {
		t.Fatalf("Expected 3 lines, got %d", len(lines))
	}

	expected := []byte{'3', '4', '5'}
	for i, exp := range expected {
		if lines[i][0] != exp {
			t.Errorf("Line %d: expected %c, got %c", i, exp, lines[i][0])
		}
	}
}

func TestGetLastN_MoreThanSize(t *testing.T) {
	cb := New(10)

	cb.Write([]byte("line1"))
	cb.Write([]byte("line2"))

	lines := cb.GetLastN(100) // Request more than available

	if len(lines) != 2 {
		t.Errorf("Expected 2 lines, got %d", len(lines))
	}
}

func TestClear(t *testing.T) {
	cb := New(10)

	cb.Write([]byte("line1"))
	cb.Write([]byte("line2"))

	cb.Clear()

	if cb.Size() != 0 {
		t.Errorf("Expected size 0 after clear, got %d", cb.Size())
	}

	lines := cb.GetAll()
	if len(lines) != 0 {
		t.Errorf("Expected 0 lines after clear, got %d", len(lines))
	}
}

func TestConcurrentWrites(t *testing.T) {
	cb := New(1000)

	var wg sync.WaitGroup
	writers := 10
	linesPerWriter := 100

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < linesPerWriter; j++ {
				cb.Write([]byte{byte(id)})
			}
		}(i)
	}

	wg.Wait()

	// Should have exactly 1000 lines (buffer capacity)
	if cb.Size() != 1000 {
		t.Errorf("Expected size 1000, got %d", cb.Size())
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	cb := New(100)

	var wg sync.WaitGroup

	// Writer goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 1000; i++ {
			cb.Write([]byte{byte(i % 256)})
		}
	}()

	// Reader goroutine
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			_ = cb.GetLastN(10)
			_ = cb.Size()
		}
	}()

	wg.Wait()

	// Should not panic and should have 100 lines (capacity)
	if cb.Size() != 100 {
		t.Errorf("Expected size 100, got %d", cb.Size())
	}
}

func TestANSIPreservation(t *testing.T) {
	cb := New(10)

	// ANSI colored text
	line := []byte("\033[1;31mRed text\033[0m")
	cb.Write(line)

	lines := cb.GetAll()
	if !bytes.Equal(lines[0], line) {
		t.Errorf("ANSI codes not preserved: expected %q, got %q", line, lines[0])
	}
}

func BenchmarkWrite(b *testing.B) {
	cb := New(10000)
	line := []byte("benchmark line with some text")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cb.Write(line)
	}
}

func BenchmarkGetLastN(b *testing.B) {
	cb := New(10000)

	// Fill buffer
	for i := 0; i < 10000; i++ {
		cb.Write([]byte("test line"))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = cb.GetLastN(1000)
	}
}

func TestExportAsText(t *testing.T) {
	cb := New(10)

	cb.Write([]byte("line1"))
	cb.Write([]byte("line2"))
	cb.Write([]byte("line3"))

	text := cb.ExportAsText()
	expected := "line1\nline2\nline3"

	if text != expected {
		t.Errorf("Expected %q, got %q", expected, text)
	}
}

func TestExportAsText_Empty(t *testing.T) {
	cb := New(10)

	text := cb.ExportAsText()
	if text != "" {
		t.Errorf("Expected empty string, got %q", text)
	}
}

func TestExportLastNAsText(t *testing.T) {
	cb := New(10)

	for i := 1; i <= 5; i++ {
		cb.Write([]byte{byte('0' + i)})
	}

	text := cb.ExportLastNAsText(3)
	expected := "3\n4\n5"

	if text != expected {
		t.Errorf("Expected %q, got %q", expected, text)
	}
}

func TestExportWithANSI(t *testing.T) {
	cb := New(10)

	cb.Write([]byte("\033[1;31mRed\033[0m"))
	cb.Write([]byte("\033[1;32mGreen\033[0m"))

	text := cb.ExportAsText()

	// Should preserve ANSI codes
	if !contains_str(text, "\033[1;31m") {
		t.Error("ANSI codes not preserved in export")
	}
}

func contains_str(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
