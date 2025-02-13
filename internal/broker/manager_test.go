//file: internal/broker/manager_test.go
package broker

import (
	"context"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/prometheus/client_golang/prometheus"
	
	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
)

// mockMQTTClient implements mqtt.Client for testing
type mockMQTTClient struct {
	connected bool
}

func (m *mockMQTTClient) IsConnected() bool { return m.connected }
func (m *mockMQTTClient) IsConnectionOpen() bool { return m.connected }
func (m *mockMQTTClient) Connect() mqtt.Token { return &mockToken{} }
func (m *mockMQTTClient) Disconnect(uint) { m.connected = false }
func (m *mockMQTTClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
	return &mockToken{}
}
func (m *mockMQTTClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mockToken{}
}
func (m *mockMQTTClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
	return &mockToken{}
}
func (m *mockMQTTClient) Unsubscribe(topics ...string) mqtt.Token { return &mockToken{} }
func (m *mockMQTTClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (m *mockMQTTClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

// mockToken implements mqtt.Token for testing
type mockToken struct {
	err error
}

func (t *mockToken) Wait() bool                     { return true }
func (t *mockToken) WaitTimeout(time.Duration) bool { return true }
func (t *mockToken) Done() <-chan struct{}          { return make(chan struct{}) }
func (t *mockToken) Error() error                   { return t.err }

func setupTestManager(t *testing.T) (*Manager, *logger.Logger) {
	t.Helper()

	log, err := logger.NewLogger(&config.LogConfig{
		Level:      "debug",
		OutputPath: "stdout",
		Encoding:   "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	reg := prometheus.NewRegistry()
	metricsService, err := metrics.NewMetrics(reg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	return NewManager(log, metricsService), log
}

func TestNewManager(t *testing.T) {
	manager, _ := setupTestManager(t)

	if manager == nil {
		t.Fatal("Expected non-nil manager")
	}
	if manager.brokers == nil {
		t.Error("Expected non-nil brokers map")
	}
	if manager.logger == nil {
		t.Error("Expected non-nil logger")
	}
	if manager.metrics == nil {
		t.Error("Expected non-nil metrics")
	}
}

func TestAddBroker(t *testing.T) {
	manager, _ := setupTestManager(t)

	tests := []struct {
		name    string
		config  *config.BrokerConfig
		wantErr bool
	}{
		{
			name: "Valid broker config",
			config: &config.BrokerConfig{
				ID:       "test-broker",
				Role:     "source",
				Address:  "tcp://localhost:1883",
				ClientID: "test-client",
			},
			wantErr: false,
		},
		{
			name: "Duplicate broker ID",
			config: &config.BrokerConfig{
				ID:       "test-broker",
				Role:     "source",
				Address:  "tcp://localhost:1883",
				ClientID: "test-client",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.AddBroker(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddBroker() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				broker, err := manager.GetBroker(tt.config.ID)
				if err != nil {
					t.Errorf("Failed to get added broker: %v", err)
				}
				if broker == nil {
					t.Error("Expected non-nil broker")
				}
				if broker.GetID() != tt.config.ID {
					t.Errorf("Broker ID = %s, want %s", broker.GetID(), tt.config.ID)
				}
			}
		})
	}
}

func TestGetBrokersByRole(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Add test brokers
	brokers := []*config.BrokerConfig{
		{
			ID:       "source1",
			Role:     "source",
			Address:  "tcp://localhost:1883",
			ClientID: "source1-client",
		},
		{
			ID:       "target1",
			Role:     "target",
			Address:  "tcp://localhost:1884",
			ClientID: "target1-client",
		},
		{
			ID:       "both1",
			Role:     "both",
			Address:  "tcp://localhost:1885",
			ClientID: "both1-client",
		},
	}

	for _, cfg := range brokers {
		if err := manager.AddBroker(cfg); err != nil {
			t.Fatalf("Failed to add broker: %v", err)
		}
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
			name:          "Both role",
			role:          BrokerRoleBoth,
			expectedCount: 1, // both1
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			brokers := manager.GetBrokersByRole(tt.role)
			if len(brokers) != tt.expectedCount {
				t.Errorf("GetBrokersByRole() got %d brokers, want %d", len(brokers), tt.expectedCount)
			}

			// Verify each broker has the correct role
			for _, broker := range brokers {
				role := broker.GetRole()
				if role != tt.role && role != BrokerRoleBoth {
					t.Errorf("Broker %s has role %s, expected %s or both", broker.GetID(), role, tt.role)
				}
			}
		})
	}
}

func TestRouteMessage(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Add source and target brokers
	sourceCfg := &config.BrokerConfig{
		ID:       "source1",
		Role:     "source",
		Address:  "tcp://localhost:1883",
		ClientID: "source1-client",
	}
	targetCfg := &config.BrokerConfig{
		ID:       "target1",
		Role:     "target",
		Address:  "tcp://localhost:1884",
		ClientID: "target1-client",
	}

	if err := manager.AddBroker(sourceCfg); err != nil {
		t.Fatalf("Failed to add source broker: %v", err)
	}
	if err := manager.AddBroker(targetCfg); err != nil {
		t.Fatalf("Failed to add target broker: %v", err)
	}

	// Mock the target broker's connected state
	targetBroker := manager.brokers[targetCfg.ID]
	targetBroker.mu.Lock()
	targetBroker.State = BrokerStateConnected
	targetBroker.Client = &mockMQTTClient{connected: true}
	targetBroker.mu.Unlock()

	tests := []struct {
		name    string
		msg     *RoutedMessage
		wantErr bool
	}{
		{
			name: "Valid route",
			msg: &RoutedMessage{
				SourceBroker: "source1",
				TargetBroker: "target1",
				Topic:       "test/topic",
				Payload:     []byte("test payload"),
				QoS:         1,
				Retain:      false,
				Timestamp:   time.Now(),
			},
			wantErr: false,
		},
		{
			name:    "Nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name: "Invalid target broker",
			msg: &RoutedMessage{
				SourceBroker: "source1",
				TargetBroker: "nonexistent",
				Topic:       "test/topic",
				Payload:     []byte("test payload"),
				QoS:         1,
				Retain:      false,
				Timestamp:   time.Now(),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := manager.RouteMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("RouteMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBrokerConnectionLifecycle(t *testing.T) {
	manager, _ := setupTestManager(t)

	cfg := &config.BrokerConfig{
		ID:       "test-broker",
		Role:     "both",
		Address:  "tcp://localhost:1883",
		ClientID: "test-client",
	}

	if err := manager.AddBroker(cfg); err != nil {
		t.Fatalf("Failed to add broker: %v", err)
	}

	broker, err := manager.GetBroker(cfg.ID)
	if err != nil {
		t.Fatalf("Failed to get broker: %v", err)
	}

	// Test connection lifecycle
	t.Run("Connect", func(t *testing.T) {
		ctx := context.Background()
		err := broker.Connect(ctx)
		// Note: This will likely fail in a test environment without a real MQTT broker
		if err == nil {
			t.Error("Expected error when connecting to non-existent broker")
		}
	})

	t.Run("Disconnect", func(t *testing.T) {
		ctx := context.Background()
		err := broker.Disconnect(ctx)
		if err != nil {
			t.Errorf("Disconnect() error = %v", err)
		}
	})

	t.Run("IsConnected", func(t *testing.T) {
		if broker.IsConnected() {
			t.Error("Expected IsConnected() to return false after disconnect")
		}
	})

	t.Run("GetStats", func(t *testing.T) {
		stats := broker.GetStats()
		if stats.MessagesReceived != 0 || stats.MessagesPublished != 0 {
			t.Error("Expected zero messages in stats for unused broker")
		}
	})
}

func TestSubscriptionManagement(t *testing.T) {
	manager, _ := setupTestManager(t)

	cfg := &config.BrokerConfig{
		ID:       "test-broker",
		Role:     "both",
		Address:  "tcp://localhost:1883",
		ClientID: "test-client",
	}

	if err := manager.AddBroker(cfg); err != nil {
		t.Fatalf("Failed to add broker: %v", err)
	}

	broker, err := manager.GetBroker(cfg.ID)
	if err != nil {
		t.Fatalf("Failed to get broker: %v", err)
	}

	// Test subscription operations
	t.Run("Subscribe while disconnected", func(t *testing.T) {
		err := broker.Subscribe("test/topic", 1)
		if err == nil {
			t.Error("Expected error when subscribing while disconnected")
		}
	})

	t.Run("Unsubscribe while disconnected", func(t *testing.T) {
		err := broker.Unsubscribe("test/topic")
		if err == nil {
			t.Error("Expected error when unsubscribing while disconnected")
		}
	})
}

func TestManagerShutdown(t *testing.T) {
	manager, _ := setupTestManager(t)

	// Add test brokers
	brokers := []*config.BrokerConfig{
		{
			ID:       "broker1",
			Role:     "source",
			Address:  "tcp://localhost:1883",
			ClientID: "broker1-client",
		},
		{
			ID:       "broker2",
			Role:     "target",
			Address:  "tcp://localhost:1884",
			ClientID: "broker2-client",
		},
	}

	for _, cfg := range brokers {
		if err := manager.AddBroker(cfg); err != nil {
			t.Fatalf("Failed to add broker: %v", err)
		}
	}

	ctx := context.Background()
	err := manager.Stop(ctx)
	if err != nil {
		t.Errorf("Stop() error = %v", err)
	}

	// Verify all brokers are disconnected
	for _, cfg := range brokers {
		broker, err := manager.GetBroker(cfg.ID)
		if err != nil {
			t.Errorf("Failed to get broker %s: %v", cfg.ID, err)
			continue
		}

		if broker.IsConnected() {
			t.Errorf("Broker %s still connected after shutdown", cfg.ID)
		}
	}
}
