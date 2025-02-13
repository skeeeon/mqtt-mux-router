//file: config/config.go
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"time"
)

// LogConfig defines logging configuration options
type LogConfig struct {
	Level      string `json:"level"`      // debug, info, warn, error
	OutputPath string `json:"outputPath"` // file path or "stdout"
	Encoding   string `json:"encoding"`   // json or console
}

// MetricsConfig defines Prometheus metrics configuration
type MetricsConfig struct {
	Enabled        bool   `json:"enabled"`
	Address        string `json:"address"`
	Path           string `json:"path"`
	UpdateInterval string `json:"updateInterval"` // Duration string
}

// ProcConfig defines message processing configuration
type ProcConfig struct {
	Workers   int `json:"workers"`
	QueueSize int `json:"queueSize"`
	BatchSize int `json:"batchSize"`
}

// BrokerRole defines the role of an MQTT broker in the system
type BrokerRole string

const (
	// BrokerRoleSource indicates a broker that only provides source messages
	BrokerRoleSource BrokerRole = "source"
	// BrokerRoleTarget indicates a broker that only receives routed messages
	BrokerRoleTarget BrokerRole = "target"
	// BrokerRoleBoth indicates a broker that can act as both source and target
	BrokerRoleBoth BrokerRole = "both"
)

// Config represents the main application configuration
type Config struct {
	// Brokers is a map of broker configurations keyed by broker ID
	Brokers    map[string]BrokerConfig `json:"brokers"`
	Logging    LogConfig               `json:"logging"`
	Metrics    MetricsConfig           `json:"metrics"`
	Processing ProcConfig              `json:"processing"`
}

// BrokerConfig represents the configuration for a single MQTT broker
type BrokerConfig struct {
	// ID is the unique identifier for this broker
	ID string `json:"id"`
	// Role defines how this broker is used in the system
	Role BrokerRole `json:"role"`
	// Address is the MQTT broker address (e.g., "ssl://mqtt.example.com:8883")
	Address string `json:"address"`
	// ClientID is the MQTT client identifier
	ClientID string `json:"clientId"`
	// Username for MQTT authentication (optional)
	Username string `json:"username,omitempty"`
	// Password for MQTT authentication (optional)
	Password string `json:"password,omitempty"`
	// TLS configuration for secure connections
	TLS *TLSConfig `json:"tls,omitempty"`
}

// TLSConfig represents TLS/SSL configuration for MQTT connections
type TLSConfig struct {
	Enable   bool   `json:"enable"`
	CertFile string `json:"certFile"`
	KeyFile  string `json:"keyFile"`
	CAFile   string `json:"caFile"`
}

// validateBrokerRole checks if the provided role is valid
func validateBrokerRole(role BrokerRole) error {
	switch role {
	case BrokerRoleSource, BrokerRoleTarget, BrokerRoleBoth:
		return nil
	default:
		return fmt.Errorf("invalid broker role: %s", role)
	}
}

// validateBrokerConfig performs validation of a single broker configuration
func validateBrokerConfig(id string, cfg BrokerConfig) error {
	if cfg.ID != id {
		return fmt.Errorf("broker ID mismatch: %s != %s", cfg.ID, id)
	}

	if cfg.Address == "" {
		return fmt.Errorf("broker address is required for broker %s", id)
	}

	if cfg.ClientID == "" {
		return fmt.Errorf("client ID is required for broker %s", id)
	}

	if err := validateBrokerRole(cfg.Role); err != nil {
		return fmt.Errorf("invalid role for broker %s: %w", id, err)
	}

	// Validate TLS config if enabled
	if cfg.TLS != nil && cfg.TLS.Enable {
		if cfg.TLS.CertFile == "" {
			return fmt.Errorf("TLS cert file is required when TLS is enabled for broker %s", id)
		}
		if cfg.TLS.KeyFile == "" {
			return fmt.Errorf("TLS key file is required when TLS is enabled for broker %s", id)
		}
		if cfg.TLS.CAFile == "" {
			return fmt.Errorf("TLS CA file is required when TLS is enabled for broker %s", id)
		}
	}

	return nil
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

	// Check if we have a modern multi-broker configuration
	if len(config.Brokers) > 0 {
		// Validate broker configurations
		for id, broker := range config.Brokers {
			if err := validateBrokerConfig(id, broker); err != nil {
				return nil, fmt.Errorf("invalid broker configuration: %w", err)
			}
		}

		// Ensure at least one source and one target broker
		hasSource := false
		hasTarget := false
		for _, broker := range config.Brokers {
			if broker.Role == BrokerRoleSource || broker.Role == BrokerRoleBoth {
				hasSource = true
			}
			if broker.Role == BrokerRoleTarget || broker.Role == BrokerRoleBoth {
				hasTarget = true
			}
		}

		if !hasSource {
			return nil, fmt.Errorf("configuration must include at least one source broker")
		}
		if !hasTarget {
			return nil, fmt.Errorf("configuration must include at least one target broker")
		}
	} else {
		// Try legacy single-broker config
		var oldConfig struct {
			MQTT struct {
				Broker   string     `json:"broker"`
				ClientID string     `json:"clientId"`
				Username string     `json:"username,omitempty"`
				Password string     `json:"password,omitempty"`
				TLS      *TLSConfig `json:"tls,omitempty"`
			} `json:"mqtt"`
			Logging    LogConfig     `json:"logging"`
			Metrics    MetricsConfig `json:"metrics"`
			Processing ProcConfig    `json:"processing"`
		}

		// Try to parse legacy format
		if err := json.Unmarshal(data, &oldConfig); err != nil {
			return nil, fmt.Errorf("failed to parse config file: %w", err)
		}

		// Validate legacy broker configuration
		if oldConfig.MQTT.Broker == "" {
			return nil, fmt.Errorf("invalid configuration: mqtt broker address is required")
		}

		// Convert old MQTT config to new broker config
		defaultBroker := BrokerConfig{
			ID:       "default",
			Role:     BrokerRoleBoth,
			Address:  oldConfig.MQTT.Broker,
			ClientID: oldConfig.MQTT.ClientID,
			Username: oldConfig.MQTT.Username,
			Password: oldConfig.MQTT.Password,
			TLS:      oldConfig.MQTT.TLS,
		}

		config.Brokers = map[string]BrokerConfig{
			"default": defaultBroker,
		}
		config.Logging = oldConfig.Logging
		config.Metrics = oldConfig.Metrics
		config.Processing = oldConfig.Processing
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

	return &config, nil
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

// GetDefaultSourceBroker returns the first available source broker configuration
func (c *Config) GetDefaultSourceBroker() (*BrokerConfig, error) {
	for _, broker := range c.Brokers {
		if broker.Role == BrokerRoleSource || broker.Role == BrokerRoleBoth {
			// Return a copy to prevent modification
			b := broker
			return &b, nil
		}
	}
	return nil, fmt.Errorf("no source broker configured")
}

// GetBrokersByRole returns all broker configurations matching the specified role
func (c *Config) GetBrokersByRole(role BrokerRole) []BrokerConfig {
	var brokers []BrokerConfig
	for _, broker := range c.Brokers {
		if broker.Role == role || broker.Role == BrokerRoleBoth {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}
