package logging

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"wtf_cli/pkg/config"

	"gopkg.in/natefinch/lumberjack.v2"
)

const defaultLogFile = "wtf_cli.log"
const (
	maxLogSizeMB  = 5
	maxLogBackups = 5
	maxLogAgeDays = 14
)

// Init configures slog to write structured logs to a file.
func Init(cfg config.Config) (*slog.Logger, error) {
	level := parseLogLevel(cfg.LogLevel)
	handlerOptions := &slog.HandlerOptions{Level: level}

	logPath := strings.TrimSpace(cfg.LogFile)
	if logPath == "" {
		logPath = defaultLogPath()
	}
	if err := os.MkdirAll(filepath.Dir(logPath), 0700); err != nil {
		logger := slog.New(newHandler(cfg.LogFormat, io.Discard, handlerOptions))
		slog.SetDefault(logger)
		return logger, err
	}

	writer := &lumberjack.Logger{
		Filename:   logPath,
		MaxSize:    maxLogSizeMB,
		MaxBackups: maxLogBackups,
		MaxAge:     maxLogAgeDays,
		Compress:   true,
	}

	logger := slog.New(newHandler(cfg.LogFormat, writer, handlerOptions))
	slog.SetDefault(logger)
	return logger, nil
}

func defaultLogPath() string {
	homeDir, err := os.UserHomeDir()
	if err != nil || strings.TrimSpace(homeDir) == "" {
		return filepath.Join(".wtf_cli", "logs", defaultLogFile)
	}
	return filepath.Join(homeDir, ".wtf_cli", "logs", defaultLogFile)
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	case "info", "":
		return slog.LevelInfo
	default:
		return slog.LevelInfo
	}
}

func newHandler(format string, out io.Writer, opts *slog.HandlerOptions) slog.Handler {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text":
		return slog.NewTextHandler(out, opts)
	default:
		return slog.NewJSONHandler(out, opts)
	}
}
