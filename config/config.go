package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

type Config struct {
	MQTT       MQTTConfig    `json:"mqtt"`
	Logging    LogConfig     `json:"logging"`
	Metrics    MetricsConfig `json:"metrics"`
	Processing ProcConfig    `json:"processing"`
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

type LogConfig struct {
	Level      string `json:"level"`      // debug, info, warn, error
	OutputPath string `json:"outputPath"` // file path or "stdout"
	Encoding   string `json:"encoding"`   // json or console
}

type MetricsConfig struct {
	Enabled        bool   `json:"enabled"`
	Address        string `json:"address"`
	Path           string `json:"path"`
	UpdateInterval string `json:"updateInterval"` // Duration string
}

type ProcConfig struct {
	Workers    int `json:"workers"`
	QueueSize  int `json:"queueSize"`
	BatchSize  int `json:"batchSize"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults for logging
	if config.Logging.Level == "" {
		config.Logging.Level = "info"
	}
	if config.Logging.OutputPath == "" {
		config.Logging.OutputPath = "stdout"
	}
	if config.Logging.Encoding == "" {
		config.Logging.Encoding = "json"
	}

	// Set defaults for metrics
	if config.Metrics.Address == "" {
		config.Metrics.Address = ":2112"
	}
	if config.Metrics.Path == "" {
		config.Metrics.Path = "/metrics"
	}
	if config.Metrics.UpdateInterval == "" {
		config.Metrics.UpdateInterval = "15s"
	}

	// Set defaults for processing
	if config.Processing.Workers <= 0 {
		config.Processing.Workers = runtime.NumCPU()
	}
	if config.Processing.QueueSize <= 0 {
		config.Processing.QueueSize = 1000
	}
	if config.Processing.BatchSize <= 0 {
		config.Processing.BatchSize = 100
	}

	// Validate the configuration
	if err := validateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// validateConfig performs validation of all configuration values
func validateConfig(cfg *Config) error {
	// Validate MQTT config
	if cfg.MQTT.Broker == "" {
		return fmt.Errorf("mqtt broker address is required")
	}

	// Validate TLS config if enabled
	if cfg.MQTT.TLS.Enable {
		if cfg.MQTT.TLS.CertFile == "" {
			return fmt.Errorf("tls cert file is required when tls is enabled")
		}
		if cfg.MQTT.TLS.KeyFile == "" {
			return fmt.Errorf("tls key file is required when tls is enabled")
		}
		if cfg.MQTT.TLS.CAFile == "" {
			return fmt.Errorf("tls ca file is required when tls is enabled")
		}
	}

	// Validate logging config
	switch cfg.Logging.Level {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("invalid log level: %s", cfg.Logging.Level)
	}

	switch cfg.Logging.Encoding {
	case "json", "console":
	default:
		return fmt.Errorf("invalid log encoding: %s", cfg.Logging.Encoding)
	}

	// Validate metrics config
	if cfg.Metrics.Enabled {
		if _, err := time.ParseDuration(cfg.Metrics.UpdateInterval); err != nil {
			return fmt.Errorf("invalid metrics update interval: %w", err)
		}
	}

	// Validate processing config
	if cfg.Processing.Workers < 1 {
		return fmt.Errorf("workers must be greater than 0")
	}
	if cfg.Processing.QueueSize < 1 {
		return fmt.Errorf("queue size must be greater than 0")
	}
	if cfg.Processing.BatchSize < 1 {
		return fmt.Errorf("batch size must be greater than 0")
	}

	return nil
}

// ApplyOverrides applies command line flag overrides to the configuration
func (c *Config) ApplyOverrides(workers, queueSize, batchSize int, metricsAddr, metricsPath string, metricsInterval time.Duration) {
	if workers > 0 {
		c.Processing.Workers = workers
	}
	if queueSize > 0 {
		c.Processing.QueueSize = queueSize
	}
	if batchSize > 0 {
		c.Processing.BatchSize = batchSize
	}
	if metricsAddr != "" {
		c.Metrics.Address = metricsAddr
	}
	if metricsPath != "" {
		c.Metrics.Path = metricsPath
	}
	if metricsInterval > 0 {
		c.Metrics.UpdateInterval = metricsInterval.String()
	}
}
