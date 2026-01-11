package ui

import (
	"testing"

	"wtf_cli/pkg/buffer"
	"wtf_cli/pkg/capture"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	buf := buffer.New(100)
	sess := capture.NewSessionContext()
	
	m := NewModel(nil, buf, sess)
	
	if m.buffer == nil {
		t.Error("Expected buffer to be set")
	}
	
	if m.session == nil {
		t.Error("Expected session to be set")
	}
	
	if m.currentDir == "" {
		t.Error("Expected currentDir to be set")
	}
}

func TestModel_Init(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext())
	
	cmd := m.Init()
	if cmd == nil {
		t.Error("Expected Init() to return a command")
	}
}

func TestModel_Update_WindowSize(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext())
	
	// Send window size message (using actual Bubble Tea type)
	newModel, _ := m.Update(tea.WindowSizeMsg{
		Width:  80,
		Height: 24,
	})
	
	updated := newModel.(Model)
	
	if updated.width != 80 {
		t.Errorf("Expected width 80, got %d", updated.width)
	}
	
	if updated.height != 24 {
		t.Errorf("Expected height 24, got %d", updated.height)
	}
	
	if !updated.ready {
		t.Error("Expected ready to be true after window size")
	}
}

func TestModel_Update_PTYOutput(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext())
	
	testData := []byte("test output")
	newModel, _ := m.Update(ptyOutputMsg{data: testData})
	
	updated := newModel.(Model)
	
	if string(updated.ptyOutput) != "test output" {
		t.Errorf("Expected 'test output', got %q", string(updated.ptyOutput))
	}
}

func TestModel_View_NotReady(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext())
	
	view := m.View()
	if view != "Initializing..." {
		t.Errorf("Expected 'Initializing...', got %q", view)
	}
}

func TestModel_View_Ready(t *testing.T) {
	m := NewModel(nil, buffer.New(100), capture.NewSessionContext())
	m.ready = true
	m.ptyOutput = []byte("hello world")
	
	view := m.View()
	if view != "hello world" {
		t.Errorf("Expected 'hello world', got %q", view)
	}
}
