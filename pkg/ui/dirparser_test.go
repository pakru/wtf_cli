package ui

import (
	"testing"
)

func TestNewDirectoryParser(t *testing.T) {
	dp := NewDirectoryParser()
	
	if dp == nil {
		t.Fatal("NewDirectoryParser() returned nil")
	}
	
	// Initially nothing parsed
	if dp.GetDirectory() != "" {
		t.Error("Expected empty directory before parsing")
	}
}

func TestDirectoryParser_SimplePrompt(t *testing.T) {
	dp := NewDirectoryParser()
	
	// Simple bash prompt: user@host:path$
	dp.ParseFromOutput([]byte("user@hostname:~/projects$"))
	
	if dp.GetDirectory() != "~/projects" {
		t.Errorf("Expected '~/projects', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_PromptWithSpace(t *testing.T) {
	dp := NewDirectoryParser()
	
	// Prompt with space before $
	dp.ParseFromOutput([]byte("user@hostname:~/projects $ "))
	
	if dp.GetDirectory() != "~/projects" {
		t.Errorf("Expected '~/projects', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_PromptWithGitBranch(t *testing.T) {
	dp := NewDirectoryParser()
	
	// Prompt with git branch like: user@host:path (branch) $
	dp.ParseFromOutput([]byte("pavel@pavel-HP-Probook-G6:~/STORAGE/Projects/my_projects/wtf_cli (feature/pty-wrapper) $ "))
	
	if dp.GetDirectory() != "~/STORAGE/Projects/my_projects/wtf_cli" {
		t.Errorf("Expected '~/STORAGE/Projects/my_projects/wtf_cli', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_PromptWithGitBranchNoSpace(t *testing.T) {
	dp := NewDirectoryParser()
	
	// Prompt with git branch, no space before $
	dp.ParseFromOutput([]byte("user@host:/home/user/project (main)$"))
	
	if dp.GetDirectory() != "/home/user/project" {
		t.Errorf("Expected '/home/user/project', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_RootPrompt(t *testing.T) {
	dp := NewDirectoryParser()
	
	// Root prompt uses #
	dp.ParseFromOutput([]byte("root@server:/etc#"))
	
	if dp.GetDirectory() != "/etc" {
		t.Errorf("Expected '/etc', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_PWDVariable(t *testing.T) {
	dp := NewDirectoryParser()
	
	// From echo $PWD or env output
	dp.ParseFromOutput([]byte("PWD=/var/log"))
	
	if dp.GetDirectory() != "/var/log" {
		t.Errorf("Expected '/var/log', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_UpdatesOnCd(t *testing.T) {
	dp := NewDirectoryParser()
	
	// First location
	dp.ParseFromOutput([]byte("user@host:~/projects $"))
	if dp.GetDirectory() != "~/projects" {
		t.Errorf("Expected '~/projects', got %q", dp.GetDirectory())
	}
	
	// After cd
	dp.ParseFromOutput([]byte("user@host:/tmp $"))
	if dp.GetDirectory() != "/tmp" {
		t.Errorf("Expected '/tmp', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_IgnoresNonPromptOutput(t *testing.T) {
	dp := NewDirectoryParser()
	
	// First, set a directory
	dp.ParseFromOutput([]byte("user@host:~/projects $"))
	
	// Random output that doesn't look like a prompt
	dp.ParseFromOutput([]byte("some random output here"))
	
	// Should still have the last parsed directory
	if dp.GetDirectory() != "~/projects" {
		t.Errorf("Expected '~/projects', got %q", dp.GetDirectory())
	}
}

func TestDirectoryParser_ComplexPath(t *testing.T) {
	dp := NewDirectoryParser()
	
	// Path with dots and dashes
	dp.ParseFromOutput([]byte("user@host:~/my-project/v1.0.0/src $"))
	
	if dp.GetDirectory() != "~/my-project/v1.0.0/src" {
		t.Errorf("Expected '~/my-project/v1.0.0/src', got %q", dp.GetDirectory())
	}
}
