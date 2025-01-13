package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	MQTT    MQTTConfig    `json:"mqtt"`
	Logging LoggingConfig `json:"logging"`
}

type MQTTConfig struct {
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Username string `json:"username"`
	Password string `json:"password"`
	TLS      struct {
		Enable   bool   `json:"enable"`
		CertFile string `json:"certFile"`
		KeyFile  string `json:"keyFile"`
		CAFile   string `json:"caFile"`
	} `json:"tls"`
}

type LoggingConfig struct {
	Directory   string `json:"directory"`
	MaxSize     int    `json:"maxSize"`     // megabytes
	MaxAge      int    `json:"maxAge"`      // days
	MaxBackups  int    `json:"maxBackups"`  // number of backup files
	Compress    bool   `json:"compress"`    // compress rotated files
	LogToFile   bool   `json:"logToFile"`   // enable file logging
	LogToStdout bool   `json:"logToStdout"` // enable stdout logging
	Level       string `json:"level"`       // debug, info, warn, error
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	// Set default values for logging if not specified
	if config.Logging.MaxSize == 0 {
		config.Logging.MaxSize = 10 // 10MB default
	}
	if config.Logging.MaxAge == 0 {
		config.Logging.MaxAge = 7 // 7 days default
	}
	if config.Logging.MaxBackups == 0 {
		config.Logging.MaxBackups = 5 // 5 backups default
	}
	if config.Logging.Level == "" {
		config.Logging.Level = "info" // default log level
	}
	if !config.Logging.LogToFile && !config.Logging.LogToStdout {
		config.Logging.LogToStdout = true // default to stdout if nothing specified
	}

	return &config, nil
}
