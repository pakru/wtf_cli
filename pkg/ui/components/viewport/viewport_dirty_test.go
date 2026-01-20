package viewport

import (
	"testing"
)

func TestViewportDirtyFlag(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	// Initially not dirty
	if vp.dirty {
		t.Error("Expected viewport to not be dirty initially")
	}

	// Append output should set dirty flag
	vp.AppendOutput([]byte("test output"))
	if !vp.dirty {
		t.Error("Expected viewport to be dirty after AppendOutput")
	}

	// View() should clear dirty flag
	_ = vp.View()
	if vp.dirty {
		t.Error("Expected viewport dirty flag to be cleared after View()")
	}

	// Empty data should not set dirty flag
	vp.AppendOutput([]byte{})
	if vp.dirty {
		t.Error("Expected viewport to not be dirty after empty AppendOutput")
	}
}

func TestViewportDirtyClear(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	// Add content
	vp.AppendOutput([]byte("content"))
	if !vp.dirty {
		t.Error("Expected dirty after append")
	}

	// Clear should set dirty
	vp.Clear()
	if !vp.dirty {
		t.Error("Expected dirty after Clear()")
	}

	// View should clear flag
	_ = vp.View()
	if vp.dirty {
		t.Error("Expected dirty flag cleared after View()")
	}
}

func TestViewportDirtyMultipleAppends(t *testing.T) {
	vp := NewPTYViewport()
	vp.SetSize(80, 24)

	// Multiple appends - dirty should stay true
	vp.AppendOutput([]byte("chunk1"))
	vp.AppendOutput([]byte("chunk2"))
	vp.AppendOutput([]byte("chunk3"))

	if !vp.dirty {
		t.Error("Expected dirty after multiple appends")
	}

	// Single View() clears it
	_ = vp.View()
	if vp.dirty {
		t.Error("Expected dirty cleared after View()")
	}
}
