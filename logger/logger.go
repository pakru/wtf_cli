package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

var Logger *slog.Logger

// InitLogger initializes the global logger based on configuration
func InitLogger(debug bool, logLevel string) {
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

	// Override to debug level if debug mode is enabled
	if debug {
		level = slog.LevelDebug
	}

	// Create handler options
	opts := &slog.HandlerOptions{
		Level: level,
	}

	// Use text handler for debug mode (more readable), JSON for production
	var handler slog.Handler
	if debug {
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
