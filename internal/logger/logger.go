// File: internal/logger/logger.go
package logger

import (
	"log/slog"
	"os"
)

type Logger struct {
	*slog.Logger
}

func NewLogger() *Logger {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	})

	return &Logger{
		Logger: slog.New(handler),
	}
}

// Fatal logs a message at Fatal level and exits the program
func (l *Logger) Fatal(msg string, args ...interface{}) {
	l.Error(msg, args...)
	os.Exit(1)
}

// Error logs a message at Error level
func (l *Logger) Error(msg string, args ...interface{}) {
	l.Logger.Error(msg, args...)
}

// Info logs a message at Info level
func (l *Logger) Info(msg string, args ...interface{}) {
	l.Logger.Info(msg, args...)
}

// Debug logs a message at Debug level
func (l *Logger) Debug(msg string, args ...interface{}) {
	l.Logger.Debug(msg, args...)
}
