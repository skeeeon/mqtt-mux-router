package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	BrokerType string        `json:"brokerType" yaml:"brokerType"` // "mqtt" or "nats"
	MQTT       MQTTConfig    `json:"mqtt" yaml:"mqtt"`
	NATS       NATSConfig    `json:"nats" yaml:"nats"`
	Logging    LogConfig     `json:"logging" yaml:"logging"`
	Metrics    MetricsConfig `json:"metrics" yaml:"metrics"`
	Processing ProcConfig    `json:"processing" yaml:"processing"`
}

type MQTTConfig struct {
	Broker   string `json:"broker" yaml:"broker"`
	ClientID string `json:"clientId" yaml:"clientId"`
	Username string `json:"username" yaml:"username"`
	Password string `json:"password" yaml:"password"`
	TLS      struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		CertFile string `json:"certFile" yaml:"certFile"`
		KeyFile  string `json:"keyFile" yaml:"keyFile"`
		CAFile   string `json:"caFile" yaml:"caFile"`
	} `json:"tls" yaml:"tls"`
}

type NATSConfig struct {
	URLs     []string `json:"urls" yaml:"urls"`
	ClientID string   `json:"clientId" yaml:"clientId"`
	Username string   `json:"username" yaml:"username"`
	Password string   `json:"password" yaml:"password"`
	TLS      struct {
		Enable   bool   `json:"enable" yaml:"enable"`
		CertFile string `json:"certFile" yaml:"certFile"`
		KeyFile  string `json:"keyFile" yaml:"keyFile"`
		CAFile   string `json:"caFile" yaml:"caFile"`
	} `json:"tls" yaml:"tls"`
}

type LogConfig struct {
	Level      string `json:"level" yaml:"level"`           // debug, info, warn, error
	OutputPath string `json:"outputPath" yaml:"outputPath"` // file path or "stdout"
	Encoding   string `json:"encoding" yaml:"encoding"`     // json or console
}

type MetricsConfig struct {
	Enabled        bool   `json:"enabled" yaml:"enabled"`
	Address        string `json:"address" yaml:"address"`
	Path           string `json:"path" yaml:"path"`
	UpdateInterval string `json:"updateInterval" yaml:"updateInterval"` // Duration string
}

type ProcConfig struct {
	Workers    int `json:"workers" yaml:"workers"`
	QueueSize  int `json:"queueSize" yaml:"queueSize"`
	BatchSize  int `json:"batchSize" yaml:"batchSize"`
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	
	// Determine file type by extension
	ext := strings.ToLower(filepath.Ext(path))
	var parseErr error
	
	switch ext {
	case ".yaml", ".yml":
		parseErr = yaml.Unmarshal(data, &config)
	case ".json":
		parseErr = json.Unmarshal(data, &config)
	default:
		// Try JSON first, then YAML if JSON fails
		parseErr = json.Unmarshal(data, &config)
		if parseErr != nil {
			yamlErr := yaml.Unmarshal(data, &config)
			if yamlErr != nil {
				return nil, fmt.Errorf("failed to parse config file (tried JSON and YAML): %w", parseErr)
			}
			// YAML parsing succeeded
			parseErr = nil
		}
	}
	
	if parseErr != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", parseErr)
	}

	// Set default broker type if not specified
	if config.BrokerType == "" {
		config.BrokerType = "mqtt" // Default to MQTT for backward compatibility
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
	// Validate based on broker type
	switch cfg.BrokerType {
	case "mqtt":
		// Validate MQTT config
		if cfg.MQTT.Broker == "" {
			return fmt.Errorf("mqtt broker address is required")
		}

		// Validate MQTT TLS config if enabled
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
	case "nats":
		// Validate NATS config
		if len(cfg.NATS.URLs) == 0 {
			return fmt.Errorf("at least one nats server URL is required")
		}

		// Validate NATS TLS config if enabled
		if cfg.NATS.TLS.Enable {
			if cfg.NATS.TLS.CertFile == "" {
				return fmt.Errorf("tls cert file is required when tls is enabled")
			}
			if cfg.NATS.TLS.KeyFile == "" {
				return fmt.Errorf("tls key file is required when tls is enabled")
			}
			if cfg.NATS.TLS.CAFile == "" {
				return fmt.Errorf("tls ca file is required when tls is enabled")
			}
		}
	default:
		return fmt.Errorf("unsupported broker type: %s", cfg.BrokerType)
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
func (c *Config) ApplyOverrides(brokerType string, workers, queueSize, batchSize int, metricsAddr, metricsPath string, metricsInterval time.Duration) {
	if brokerType != "" {
		c.BrokerType = brokerType
	}
	
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
