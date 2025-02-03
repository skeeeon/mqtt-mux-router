package config

import (
	"encoding/json"
	"os"
)

type Config struct {
	MQTT    MQTTConfig `json:"mqtt"`
	Logging LogConfig  `json:"logging"`
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

// LogConfig holds the logger configuration
type LogConfig struct {
	Level      string `json:"level"`      // debug, info, warn, error
	OutputPath string `json:"outputPath"` // file path or "stdout"
	Encoding   string `json:"encoding"`   // json or console
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
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.OutputPath == "" {
		config.Logging.OutputPath = "stdout"
	}
	if config.Logging.Encoding == "" {
		config.Logging.Encoding = "json"
	}

	return &config, nil
}
