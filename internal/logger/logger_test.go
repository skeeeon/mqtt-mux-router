package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"mqtt-mux-router/config"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *config.LogConfig
		wantErr bool
	}{
		{
			name: "valid config",
			cfg: &config.LogConfig{
				Level:      "info",
				OutputPath: "stdout",
				Encoding:   "json",
			},
			wantErr: false,
		},
		{
			name:    "nil config",
			cfg:     nil,
			wantErr: true,
		},
		{
			name: "invalid level",
			cfg: &config.LogConfig{
				Level:      "invalid",
				OutputPath: "stdout",
				Encoding:   "json",
			},
			wantErr: false, // defaults to info level
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger, err := NewLogger(tt.cfg)
			if tt.wantErr {
				assert.Error(t, err)
				assert.Nil(t, logger)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, logger)
			}
		})
	}
}

func TestLoggerMethods(t *testing.T) {
	cfg := &config.LogConfig{
		Level:      "debug",
		OutputPath: "stdout",
		Encoding:   "json",
	}

	logger, err := NewLogger(cfg)
	assert.NoError(t, err)
	assert.NotNil(t, logger)

	// Test each log level
	logger.Debug("debug message", "key", "value")
	logger.Info("info message", "key", "value")
	logger.Error("error message", "key", "value")
}
