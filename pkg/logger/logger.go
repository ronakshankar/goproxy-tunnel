// Package logger provides a thin, dependency-injection-friendly wrapper around
// Go 1.21's structured logging package (log/slog). All components receive a
// *Logger via their constructors — no package-level globals are used.
package logger

import (
	"log/slog"
	"os"
)

// Logger wraps slog.Logger to provide a stable interface that can be swapped
// for a test double without importing slog throughout the codebase.
type Logger struct {
	l *slog.Logger
}

// New returns a Logger that writes human-readable text to stdout at DEBUG level.
func New() *Logger {
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})
	return &Logger{l: slog.New(handler)}
}

// NewJSON returns a Logger that writes JSON to stdout — suitable for production
// environments where log aggregation tooling parses structured fields.
func NewJSON() *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	return &Logger{l: slog.New(handler)}
}

func (l *Logger) Debug(msg string, args ...any) { l.l.Debug(msg, args...) }
func (l *Logger) Info(msg string, args ...any)  { l.l.Info(msg, args...) }
func (l *Logger) Warn(msg string, args ...any)  { l.l.Warn(msg, args...) }
func (l *Logger) Error(msg string, args ...any) { l.l.Error(msg, args...) }

// With returns a new Logger with the given key-value pairs pre-attached to
// every subsequent log record. Useful for scoping logs to a connection ID.
func (l *Logger) With(args ...any) *Logger {
	return &Logger{l: l.l.With(args...)}
}
