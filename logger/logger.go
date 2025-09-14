package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

var Logger *slog.Logger

// InitLogger initializes the global logger based on configuration
func InitLogger(logLevel string) {
	var level slog.Level

	// Set log level based on configuration
	switch strings.ToLower(logLevel) {
	case "debug":
		level = slog.LevelDebug
	case "info":
		level = slog.LevelInfo
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Use text handler for debug level (more readable), JSON for others
	var handler slog.Handler
	if level == slog.LevelDebug {
		handler = slog.NewTextHandler(os.Stdout, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stdout, opts)
	}

	Logger = slog.New(handler)
}

// Debug logs a debug message
func Debug(msg string, args ...any) {
	Logger.Debug(msg, args...)
}

// Info logs an info message
func Info(msg string, args ...any) {
	Logger.Info(msg, args...)
}

// Warn logs a warning message
func Warn(msg string, args ...any) {
	Logger.Warn(msg, args...)
}

// Error logs an error message
func Error(msg string, args ...any) {
	Logger.Error(msg, args...)
}

// DebugEnabled returns true if debug logging is enabled
func DebugEnabled() bool {
	return Logger.Enabled(context.TODO(), slog.LevelDebug)
}
