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

// LevelTrace enables verbose diagnostic logging below debug.
const LevelTrace slog.Level = -8

var levelVar slog.LevelVar

func init() {
	levelVar.Set(slog.LevelInfo)
}

// Init configures slog to write structured logs to a file.
func Init(cfg config.Config) (*slog.Logger, error) {
	levelVar.Set(parseLogLevel(cfg.LogLevel))
	handlerOptions := &slog.HandlerOptions{
		Level:       &levelVar,
		ReplaceAttr: replaceLevelAttr,
	}

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

// SetLevel updates the active log level for the default logger.
func SetLevel(level string) {
	levelVar.Set(parseLogLevel(level))
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
	case "trace":
		return LevelTrace
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

func replaceLevelAttr(_ []string, a slog.Attr) slog.Attr {
	if a.Key != slog.LevelKey {
		return a
	}

	switch a.Value.Kind() {
	case slog.KindInt64:
		if a.Value.Int64() == int64(LevelTrace) {
			a.Value = slog.StringValue("TRACE")
		}
	case slog.KindAny:
		if lvl, ok := a.Value.Any().(slog.Level); ok && lvl == LevelTrace {
			a.Value = slog.StringValue("TRACE")
		}
	}
	return a
}

func newHandler(format string, out io.Writer, opts *slog.HandlerOptions) slog.Handler {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "text":
		return slog.NewTextHandler(out, opts)
	default:
		return slog.NewJSONHandler(out, opts)
	}
}
