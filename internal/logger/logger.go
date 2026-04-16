package logger

import (
	"context"
	"log/slog"
	"os"
	"strings"
)

// Logger is a thin wrapper around slog.Logger that provides a consistent
// interface with pre-attached default attributes (agent_id, tenant, etc.).
type Logger struct {
	inner *slog.Logger
}

// New creates a root Logger. format is "json" (default) or "text".
// level is "debug", "info" (default), "warn", or "error".
func New(format, level string) *Logger {
	var lvl slog.Level
	switch strings.ToLower(level) {
	case "debug":
		lvl = slog.LevelDebug
	case "warn":
		lvl = slog.LevelWarn
	case "error":
		lvl = slog.LevelError
	default:
		lvl = slog.LevelInfo
	}

	opts := &slog.HandlerOptions{Level: lvl}

	var handler slog.Handler
	if strings.ToLower(format) == "text" {
		handler = slog.NewTextHandler(os.Stderr, opts)
	} else {
		handler = slog.NewJSONHandler(os.Stderr, opts)
	}

	return &Logger{inner: slog.New(handler)}
}

// Default returns a logger with JSON format at Info level, suitable for
// use before configuration is loaded.
func Default() *Logger {
	return New("json", "info")
}

// With returns a new Logger with the given key-value pairs attached to
// every subsequent log entry. Keys must be strings; values can be anything.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{inner: l.inner.With(args...)}
}

// Debug logs at DEBUG level.
func (l *Logger) Debug(msg string, args ...any) {
	l.inner.Debug(msg, args...)
}

// Info logs at INFO level.
func (l *Logger) Info(msg string, args ...any) {
	l.inner.Info(msg, args...)
}

// Warn logs at WARN level.
func (l *Logger) Warn(msg string, args ...any) {
	l.inner.Warn(msg, args...)
}

// Error logs at ERROR level.
func (l *Logger) Error(msg string, args ...any) {
	l.inner.Error(msg, args...)
}

// Fatal logs at ERROR level then calls os.Exit(1).
func (l *Logger) Fatal(msg string, args ...any) {
	l.inner.Error(msg, args...)
	os.Exit(1)
}

// WithContext returns the logger unchanged. Provided for future use if
// context-based trace propagation is added (e.g. trace_id from ctx).
func (l *Logger) WithContext(_ context.Context) *Logger {
	return l
}
