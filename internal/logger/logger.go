package logger

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"mqtt-mux-router/config"
	"gopkg.in/natefinch/lumberjack.v2"
)

type Logger struct {
	*slog.Logger
}

func NewLogger(cfg *config.LoggingConfig) (*Logger, error) {
	// Create logging directory if it doesn't exist
	if cfg.LogToFile && cfg.Directory != "" {
		if err := os.MkdirAll(cfg.Directory, 0755); err != nil {
			return nil, err
		}
	}

	// Set up log level
	var level slog.Level
	switch cfg.Level {
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

	// Create the appropriate writer(s)
	var writer io.Writer

	if cfg.LogToFile && cfg.LogToStdout {
		// Log to both file and stdout
		fileWriter := &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Directory, "mqtt-mux-router.log"),
			MaxSize:    cfg.MaxSize,
			MaxAge:     cfg.MaxAge,
			MaxBackups: cfg.MaxBackups,
			Compress:   cfg.Compress,
		}
		writer = io.MultiWriter(os.Stdout, fileWriter)
	} else if cfg.LogToFile {
		// Log to file only
		writer = &lumberjack.Logger{
			Filename:   filepath.Join(cfg.Directory, "mqtt-mux-router.log"),
			MaxSize:    cfg.MaxSize,
			MaxAge:     cfg.MaxAge,
			MaxBackups: cfg.MaxBackups,
			Compress:   cfg.Compress,
		}
	} else {
		// Log to stdout only (default)
		writer = os.Stdout
	}

	// Create the handler with the configured options
	handler := slog.NewJSONHandler(writer, &slog.HandlerOptions{
		Level: level,
	})

	return &Logger{
		Logger: slog.New(handler),
	}, nil
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
