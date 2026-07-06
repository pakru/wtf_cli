package commands

import (
	"testing"

	"wtf_cli/pkg/ai/tools"
	"wtf_cli/pkg/config"
)

func TestBuildToolRegistry_DefaultsBothToolsRegistered(t *testing.T) {
	cfg := config.Default()
	registry := buildToolRegistry(cfg, t.TempDir())

	if registry.Len() != 2 {
		t.Fatalf("registry.Len() = %d, want 2", registry.Len())
	}
	if _, ok := registry.Get("read_file"); !ok {
		t.Error("expected read_file to be registered by default")
	}
	if _, ok := registry.Get("list_directory"); !ok {
		t.Error("expected list_directory to be registered by default")
	}
}

func TestBuildToolRegistry_ReadFileDisabledIndependently(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Tools.ReadFile.Enabled = false
	registry := buildToolRegistry(cfg, t.TempDir())

	if _, ok := registry.Get("read_file"); ok {
		t.Error("expected read_file to be absent when disabled")
	}
	if _, ok := registry.Get("list_directory"); !ok {
		t.Error("expected list_directory to remain registered when only read_file is disabled")
	}
	if registry.Len() != 1 {
		t.Fatalf("registry.Len() = %d, want 1", registry.Len())
	}
}

func TestBuildToolRegistry_ListDirectoryDisabledIndependently(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Tools.ListDirectory.Enabled = false
	registry := buildToolRegistry(cfg, t.TempDir())

	if _, ok := registry.Get("list_directory"); ok {
		t.Error("expected list_directory to be absent when disabled")
	}
	if _, ok := registry.Get("read_file"); !ok {
		t.Error("expected read_file to remain registered when only list_directory is disabled")
	}
	if registry.Len() != 1 {
		t.Fatalf("registry.Len() = %d, want 1", registry.Len())
	}
}

func TestBuildToolRegistry_BothToolsDisabled(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Tools.ReadFile.Enabled = false
	cfg.Agent.Tools.ListDirectory.Enabled = false
	registry := buildToolRegistry(cfg, t.TempDir())

	if registry.Len() != 0 {
		t.Fatalf("registry.Len() = %d, want 0", registry.Len())
	}
}

func TestBuildToolRegistry_CapsAndCwdPropagate(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Tools.ReadFile.MaxLines = 42
	cfg.Agent.Tools.ReadFile.MaxBytes = 4096
	cfg.Agent.Tools.ListDirectory.MaxEntries = 7
	cfg.Agent.Tools.ListDirectory.MaxBytes = 2048
	cwd := t.TempDir()

	registry := buildToolRegistry(cfg, cwd)

	rf, ok := registry.Get("read_file")
	if !ok {
		t.Fatal("read_file not registered")
	}
	readFile, ok := rf.(*tools.ReadFile)
	if !ok {
		t.Fatalf("read_file has unexpected concrete type %T", rf)
	}
	if readFile.Cwd != cwd || readFile.MaxLines != 42 || readFile.MaxBytes != 4096 {
		t.Errorf("read_file config not propagated: %+v", readFile)
	}

	ld, ok := registry.Get("list_directory")
	if !ok {
		t.Fatal("list_directory not registered")
	}
	listDirectory, ok := ld.(*tools.ListDirectory)
	if !ok {
		t.Fatalf("list_directory has unexpected concrete type %T", ld)
	}
	if listDirectory.Cwd != cwd || listDirectory.MaxEntries != 7 || listDirectory.MaxBytes != 2048 {
		t.Errorf("list_directory config not propagated: %+v", listDirectory)
	}
}

func TestBuildToolRegistry_OutOfWorkdirAccessAskEnablesEscapesOnBothTools(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Tools.OutOfWorkdirAccess = config.WorkdirAccessAsk
	registry := buildToolRegistry(cfg, t.TempDir())

	rf, _ := registry.Get("read_file")
	if !rf.(*tools.ReadFile).AllowEscapes {
		t.Error("expected read_file.AllowEscapes=true under the ask policy")
	}
	ld, _ := registry.Get("list_directory")
	if !ld.(*tools.ListDirectory).AllowEscapes {
		t.Error("expected list_directory.AllowEscapes=true under the ask policy")
	}
}

func TestBuildToolRegistry_OutOfWorkdirAccessDenyDisablesEscapesOnBothTools(t *testing.T) {
	cfg := config.Default()
	cfg.Agent.Tools.OutOfWorkdirAccess = config.WorkdirAccessDeny
	registry := buildToolRegistry(cfg, t.TempDir())

	rf, _ := registry.Get("read_file")
	if rf.(*tools.ReadFile).AllowEscapes {
		t.Error("expected read_file.AllowEscapes=false under the deny policy")
	}
	ld, _ := registry.Get("list_directory")
	if ld.(*tools.ListDirectory).AllowEscapes {
		t.Error("expected list_directory.AllowEscapes=false under the deny policy")
	}
}
