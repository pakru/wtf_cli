package logging

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"wtf_cli/pkg/config"
)

func TestInitCreatesLogFile(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "logs", "wtf_cli.log")

	cfg := config.Default()
	cfg.OpenRouter.APIKey = "test-key"
	cfg.LogFile = logPath
	cfg.LogFormat = "json"
	cfg.LogLevel = "info"

	logger, err := Init(cfg)
	if err != nil {
		t.Fatalf("Init() error: %v", err)
	}

	logger.Info("hello", slog.String("component", "test"))

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile() error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Expected log file to have content")
	}
	if !strings.Contains(string(data), "hello") {
		t.Fatalf("Expected log to contain message, got: %s", string(data))
	}
}
