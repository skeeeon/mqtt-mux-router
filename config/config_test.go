package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBrokerRoleValidation(t *testing.T) {
	tests := []struct {
		name    string
		role    BrokerRole
		wantErr bool
	}{
		{"Valid source role", BrokerRoleSource, false},
		{"Valid target role", BrokerRoleTarget, false},
		{"Valid both role", BrokerRoleBoth, false},
		{"Invalid role", BrokerRole("invalid"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBrokerRole(tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBrokerRole() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBrokerConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		config  BrokerConfig
		wantErr bool
	}{
		{
			name: "Valid config",
			id:   "broker1",
			config: BrokerConfig{
				ID:       "broker1",
				Role:     BrokerRoleSource,
				Address:  "mqtt://localhost:1883",
				ClientID: "test-client",
			},
			wantErr: false,
		},
		{
			name: "ID mismatch",
			id:   "broker1",
			config: BrokerConfig{
				ID:       "broker2",
				Role:     BrokerRoleSource,
				Address:  "mqtt://localhost:1883",
				ClientID: "test-client",
			},
			wantErr: true,
		},
		{
			name: "Missing address",
			id:   "broker1",
			config: BrokerConfig{
				ID:       "broker1",
				Role:     BrokerRoleSource,
				ClientID: "test-client",
			},
			wantErr: true,
		},
		{
			name: "Invalid TLS config",
			id:   "broker1",
			config: BrokerConfig{
				ID:       "broker1",
				Role:     BrokerRoleSource,
				Address:  "mqtt://localhost:1883",
				ClientID: "test-client",
				TLS: &TLSConfig{
					Enable: true,
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBrokerConfig(tt.id, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateBrokerConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestLoad(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir, err := os.MkdirTemp("", "config_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Test cases
	tests := []struct {
		name     string
		config   map[string]interface{}
		wantErr  bool
		validate func(*testing.T, *Config)
	}{
		{
			name: "Valid multi-broker config",
			config: map[string]interface{}{
				"brokers": map[string]interface{}{
					"source1": map[string]interface{}{
						"id":       "source1",
						"role":     "source",
						"address":  "mqtt://source:1883",
						"clientId": "source-client",
					},
					"target1": map[string]interface{}{
						"id":       "target1",
						"role":     "target",
						"address":  "mqtt://target:1883",
						"clientId": "target-client",
					},
				},
			},
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Brokers) != 2 {
					t.Errorf("expected 2 brokers, got %d", len(c.Brokers))
				}
			},
		},
		{
			name: "Backward compatibility test",
			config: map[string]interface{}{
				"mqtt": map[string]interface{}{
					"broker":   "mqtt://localhost:1883",
					"clientId": "test-client",
					"address":  "mqtt://localhost:1883", // Added this field
				},
			},
			wantErr: false,
			validate: func(t *testing.T, c *Config) {
				if len(c.Brokers) != 1 {
					t.Errorf("expected 1 broker, got %d", len(c.Brokers))
				}
				if broker, ok := c.Brokers["default"]; ok {
					if broker.Role != BrokerRoleBoth {
						t.Errorf("expected role 'both', got %s", broker.Role)
					}
				} else {
					t.Error("default broker not found")
				}
			},
		},
		{
			name: "Missing source broker",
			config: map[string]interface{}{
				"brokers": map[string]interface{}{
					"target1": map[string]interface{}{
						"id":       "target1",
						"role":     "target",
						"address":  "mqtt://target:1883",
						"clientId": "target-client",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Missing target broker",
			config: map[string]interface{}{
				"brokers": map[string]interface{}{
					"source1": map[string]interface{}{
						"id":       "source1",
						"role":     "source",
						"address":  "mqtt://source:1883",
						"clientId": "source-client",
					},
				},
			},
			wantErr: true,
		},
		{
			name: "Invalid broker role",
			config: map[string]interface{}{
				"brokers": map[string]interface{}{
					"source1": map[string]interface{}{
						"id":       "source1",
						"role":     "invalid",
						"address":  "mqtt://source:1883",
						"clientId": "source-client",
					},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a temporary config file
			configPath := filepath.Join(tmpDir, "config.json")
			configData, err := json.Marshal(tt.config)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(configPath, configData, 0644); err != nil {
				t.Fatal(err)
			}

			// Load the configuration
			cfg, err := Load(configPath)
			if (err != nil) != tt.wantErr {
				t.Errorf("Load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if err == nil && tt.validate != nil {
				tt.validate(t, cfg)
			}
		})
	}
}

func TestGetDefaultSourceBroker(t *testing.T) {
	tests := []struct {
		name     string
		brokers  map[string]BrokerConfig
		wantErr  bool
		validate func(*testing.T, *BrokerConfig)
	}{
		{
			name: "Single source broker",
			brokers: map[string]BrokerConfig{
				"source1": {
					ID:       "source1",
					Role:     BrokerRoleSource,
					Address:  "mqtt://source:1883",
					ClientID: "source-client",
				},
				"target1": {
					ID:       "target1",
					Role:     BrokerRoleTarget,
					Address:  "mqtt://target:1883",
					ClientID: "target-client",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, b *BrokerConfig) {
				if b.ID != "source1" {
					t.Errorf("expected source1, got %s", b.ID)
				}
			},
		},
		{
			name: "Both role as source",
			brokers: map[string]BrokerConfig{
				"both1": {
					ID:       "both1",
					Role:     BrokerRoleBoth,
					Address:  "mqtt://both:1883",
					ClientID: "both-client",
				},
				"target1": {
					ID:       "target1",
					Role:     BrokerRoleTarget,
					Address:  "mqtt://target:1883",
					ClientID: "target-client",
				},
			},
			wantErr: false,
			validate: func(t *testing.T, b *BrokerConfig) {
				if b.ID != "both1" {
					t.Errorf("expected both1, got %s", b.ID)
				}
			},
		},
		{
			name: "No source broker",
			brokers: map[string]BrokerConfig{
				"target1": {
					ID:       "target1",
					Role:     BrokerRoleTarget,
					Address:  "mqtt://target:1883",
					ClientID: "target-client",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{Brokers: tt.brokers}
			broker, err := cfg.GetDefaultSourceBroker()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetDefaultSourceBroker() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err == nil && tt.validate != nil {
				tt.validate(t, broker)
			}
		})
	}
}

func TestGetBrokersByRole(t *testing.T) {
	cfg := &Config{
		Brokers: map[string]BrokerConfig{
			"source1": {
				ID:       "source1",
				Role:     BrokerRoleSource,
				Address:  "mqtt://source:1883",
				ClientID: "source-client",
			},
			"target1": {
				ID:       "target1",
				Role:     BrokerRoleTarget,
				Address:  "mqtt://target:1883",
				ClientID: "target-client",
			},
			"both1": {
				ID:       "both1",
				Role:     BrokerRoleBoth,
				Address:  "mqtt://both:1883",
				ClientID: "both-client",
			},
		},
	}

	tests := []struct {
		name          string
		role          BrokerRole
		expectedCount int
	}{
		{
			name:          "Source brokers",
			role:          BrokerRoleSource,
			expectedCount: 2, // source1 and both1
		},
		{
			name:          "Target brokers",
			role:          BrokerRoleTarget,
			expectedCount: 2, // target1 and both1
		},
		{
			name:          "Both role brokers",
			role:          BrokerRoleBoth,
			expectedCount: 1, // both1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokers := cfg.GetBrokersByRole(tt.role)
			if len(brokers) != tt.expectedCount {
				t.Errorf("GetBrokersByRole() got %d brokers, want %d", len(brokers), tt.expectedCount)
			}
		})
	}
}

func TestApplyOverrides(t *testing.T) {
	cfg := &Config{
		Processing: ProcConfig{
			Workers:    4,
			QueueSize: 1000,
			BatchSize: 100,
		},
		Metrics: MetricsConfig{
			Address:        ":2112",
			Path:          "/metrics",
			UpdateInterval: "15s",
		},
	}

	tests := []struct {
		name            string
		workers         int
		queueSize       int
		batchSize       int
		metricsAddr     string
		metricsPath     string
		metricsInterval time.Duration
		validate        func(*testing.T, *Config)
	}{
		{
			name:            "Override all values",
			workers:         8,
			queueSize:      2000,
			batchSize:      200,
			metricsAddr:    ":3000",
			metricsPath:    "/prometheus",
			metricsInterval: 30 * time.Second,
			validate: func(t *testing.T, c *Config) {
				if c.Processing.Workers != 8 {
					t.Errorf("expected Workers=8, got %d", c.Processing.Workers)
				}
				if c.Processing.QueueSize != 2000 {
					t.Errorf("expected QueueSize=2000, got %d", c.Processing.QueueSize)
				}
				if c.Processing.BatchSize != 200 {
					t.Errorf("expected BatchSize=200, got %d", c.Processing.BatchSize)
				}
				if c.Metrics.Address != ":3000" {
					t.Errorf("expected Address=:3000, got %s", c.Metrics.Address)
				}
				if c.Metrics.Path != "/prometheus" {
					t.Errorf("expected Path=/prometheus, got %s", c.Metrics.Path)
				}
				if c.Metrics.UpdateInterval != "30s" {
					t.Errorf("expected UpdateInterval=30s, got %s", c.Metrics.UpdateInterval)
				}
			},
		},
		{
			name:            "No overrides",
			workers:         0,
			queueSize:       0,
			batchSize:       0,
			metricsAddr:     "",
			metricsPath:     "",
			metricsInterval: 0,
			validate: func(t *testing.T, c *Config) {
				if c.Processing.Workers != 4 {
					t.Errorf("expected Workers=4, got %d", c.Processing.Workers)
				}
				if c.Processing.QueueSize != 1000 {
					t.Errorf("expected QueueSize=1000, got %d", c.Processing.QueueSize)
				}
				if c.Processing.BatchSize != 100 {
					t.Errorf("expected BatchSize=100, got %d", c.Processing.BatchSize)
				}
				if c.Metrics.Address != ":2112" {
					t.Errorf("expected Address=:2112, got %s", c.Metrics.Address)
				}
				if c.Metrics.Path != "/metrics" {
					t.Errorf("expected Path=/metrics, got %s", c.Metrics.Path)
				}
				if c.Metrics.UpdateInterval != "15s" {
					t.Errorf("expected UpdateInterval=15s, got %s", c.Metrics.UpdateInterval)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a copy of the config for each test
			testCfg := *cfg
			testCfg.ApplyOverrides(
				tt.workers,
				tt.queueSize,
				tt.batchSize,
				tt.metricsAddr,
				tt.metricsPath,
				tt.metricsInterval,
			)
			tt.validate(t, &testCfg)
		})
	}
}
