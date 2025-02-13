package logger

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"mqtt-mux-router/config"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

// TestNewLogger verifies logger initialization with different configurations
func TestNewLogger(t *testing.T) {
	testCases := []struct {
		name           string
		logConfig      *config.LogConfig
		expectedLevel  zapcore.Level
		expectedOutput string
		shouldFail     bool
	}{
		{
			name: "Valid Debug Configuration",
			logConfig: &config.LogConfig{
				Level:      "debug",
				Encoding:   "json",
				OutputPath: "./test-debug.log",
			},
			expectedLevel:  zapcore.DebugLevel,
			expectedOutput: "./test-debug.log",
		},
		{
			name: "Valid Info Configuration",
			logConfig: &config.LogConfig{
				Level:      "info",
				Encoding:   "json",
				OutputPath: "./test-info.log",
			},
			expectedLevel:  zapcore.InfoLevel,
			expectedOutput: "./test-info.log",
		},
		{
			name: "Default Level Configuration",
			logConfig: &config.LogConfig{
				Level:      "unknown",
				Encoding:   "json",
				OutputPath: "./test-default.log",
			},
			expectedLevel:  zapcore.InfoLevel,
			expectedOutput: "./test-default.log",
		},
		{
			name:           "Nil Configuration",
			logConfig:      nil,
			shouldFail:     true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logger, err := NewLogger(tc.logConfig)

			if tc.shouldFail {
				require.Error(t, err, "NewLogger should fail with nil config")
				return
			}

			require.NoError(t, err, "NewLogger should not return an error")
			require.NotNil(t, logger, "Logger should be created")

			// Verify log level
			atomicLevel := logger.Logger.Level()
			assert.Equal(t, tc.expectedLevel, atomicLevel, "Log level should match configuration")

			// Clean up log file
			if tc.logConfig != nil && tc.logConfig.OutputPath != "" {
				os.Remove(tc.logConfig.OutputPath)
			}
		})
	}
}

// TestLoggerMethods verifies different logging methods
func TestLoggerMethods(t *testing.T) {
	// Create a temporary log file
	logFile := filepath.Join(os.TempDir(), "test-logger-methods.log")
	defer os.Remove(logFile)

	// Create logger configuration
	logConfig := &config.LogConfig{
		Level:      "debug",
		Encoding:   "json",
		OutputPath: logFile,
	}

	logger, err := NewLogger(logConfig)
	require.NoError(t, err, "Logger creation should succeed")

	// Test different logging methods
	testCases := []struct {
		name       string
		logMethod  func(msg string, args ...interface{})
		logLevel   string
		msg        string
		fields     []interface{}
		checkField string
	}{
		{
			name:       "Debug Log",
			logMethod:  logger.Debug,
			logLevel:   "debug",
			msg:        "Debug message",
			fields:     []interface{}{"key", "debug-value"},
			checkField: "debug-value",
		},
		{
			name:       "Info Log",
			logMethod:  logger.Info,
			logLevel:   "info",
			msg:        "Info message",
			fields:     []interface{}{"key", "info-value"},
			checkField: "info-value",
		},
		{
			name:       "Error Log",
			logMethod:  logger.Error,
			logLevel:   "error",
			msg:        "Error message",
			fields:     []interface{}{"key", "error-value"},
			checkField: "error-value",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear log file before each test
			err := os.Truncate(logFile, 0)
			require.NoError(t, err, "Should truncate log file")

			// Capture log output
			tc.logMethod(tc.msg, tc.fields...)

			// Sync to ensure log is written
			err = logger.Sync()
			require.NoError(t, err, "Logger sync should succeed")

			// Open and read log file
			file, err := os.Open(logFile)
			require.NoError(t, err, "Should open log file")
			defer file.Close()

			// Scan log file line by line (each line is a JSON object)
			scanner := bufio.NewScanner(file)
			require.True(t, scanner.Scan(), "Should have a log entry")

			// Parse JSON log entry
			var logEntry map[string]interface{}
			err = json.Unmarshal(scanner.Bytes(), &logEntry)
			require.NoError(t, err, "Should parse log JSON")

			// Verify log entry
			assert.Equal(t, tc.msg, logEntry["msg"], "Log message should match")
			assert.Equal(t, tc.checkField, logEntry["key"], "Log field should match")
			assert.Contains(t, logEntry, "timestamp", "Should have timestamp")
			assert.Contains(t, logEntry, "level", "Should have log level")
		})
	}
}

// TestArgsToFields ensures correct field conversion
func TestArgsToFields(t *testing.T) {
	testCases := []struct {
		name           string
		args           []interface{}
		expectedLength int
	}{
		{
			name:           "Valid key-value pairs",
			args:           []interface{}{"key1", "value1", "key2", 42},
			expectedLength: 2,
		},
		{
			name:           "Odd number of arguments",
			args:           []interface{}{"key1", "value1", "key2"},
			expectedLength: 1,
		},
		{
			name:           "Invalid key type",
			args:           []interface{}{42, "value1", "key2", "value2"},
			expectedLength: 1,
		},
		{
			name:           "Empty arguments",
			args:           []interface{}{},
			expectedLength: 0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			fields := argsToFields(tc.args...)
			assert.Len(t, fields, tc.expectedLength, "Number of converted fields should match")
		})
	}
}

// TestSync verifies log synchronization
func TestSync(t *testing.T) {
	logConfig := &config.LogConfig{
		Level:      "info",
		Encoding:   "json",
		OutputPath: filepath.Join(os.TempDir(), "test-sync.log"),
	}

	logger, err := NewLogger(logConfig)
	require.NoError(t, err, "Logger creation should succeed")

	err = logger.Sync()
	assert.NoError(t, err, "Sync should not return an error")
}
