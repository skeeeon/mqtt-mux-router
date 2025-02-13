//file: internal/broker/manager.go
package broker

import (
	"context"
	"fmt"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
)

// Manager implements the BrokerManager interface
type Manager struct {
	brokers    map[string]*ManagedBroker
	logger     *logger.Logger
	metrics    *metrics.Metrics
	mu         sync.RWMutex
	wg         sync.WaitGroup
}

// NewManager creates a new broker manager instance
func NewManager(logger *logger.Logger, metrics *metrics.Metrics) *Manager {
	return &Manager{
		brokers: make(map[string]*ManagedBroker),
		logger:  logger,
		metrics: metrics,
	}
}

// AddBroker adds a new broker to the manager
func (m *Manager) AddBroker(cfg *config.BrokerConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.brokers[cfg.ID]; exists {
		return fmt.Errorf("broker with ID %s already exists", cfg.ID)
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Address).
		SetClientID(cfg.ClientID).
		SetUsername(cfg.Username).
		SetPassword(cfg.Password).
		SetCleanSession(true).
		SetAutoReconnect(true).
		SetConnectRetry(true)

	broker := &ManagedBroker{
		ID:            cfg.ID,
		Role:          BrokerRole(cfg.Role),
		State:         BrokerStateDisconnected,
		ClientOptions: opts,
		Config:        cfg,
		Logger:        m.logger,
		Metrics:       m.metrics,
		Subscriptions: &SubscriptionManager{
			topics:    make(map[string]byte),
			wildcards: &TopicTree{root: &TopicNode{children: make(map[string]*TopicNode)}},
		},
	}

	// Set up connection handlers
	opts.SetOnConnectHandler(func(c mqtt.Client) {
		broker.handleConnect()
	})

	opts.SetConnectionLostHandler(func(c mqtt.Client, err error) {
		broker.handleDisconnect(err)
	})

	opts.SetReconnectingHandler(func(c mqtt.Client, opts *mqtt.ClientOptions) {
		broker.handleReconnecting()
	})

	// Create MQTT client
	broker.Client = mqtt.NewClient(opts)

	m.brokers[cfg.ID] = broker
	return nil
}

// Start initializes and connects all managed brokers
func (m *Manager) Start(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, broker := range m.brokers {
		if err := broker.Connect(ctx); err != nil {
			return fmt.Errorf("failed to connect broker %s: %w", broker.ID, err)
		}
	}

	return nil
}

// Stop gracefully disconnects all managed brokers
func (m *Manager) Stop(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, broker := range m.brokers {
		if err := broker.Disconnect(ctx); err != nil {
			m.logger.Error("failed to disconnect broker",
				"broker", broker.ID,
				"error", err)
		}
	}

	m.wg.Wait()
	return nil
}

// GetBroker returns a broker by ID
func (m *Manager) GetBroker(id string) (BrokerConnection, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	broker, exists := m.brokers[id]
	if !exists {
		return nil, fmt.Errorf("broker %s not found", id)
	}

	return broker, nil
}

// GetBrokersByRole returns all brokers matching a specific role
func (m *Manager) GetBrokersByRole(role BrokerRole) []BrokerConnection {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []BrokerConnection
	for _, broker := range m.brokers {
		if broker.Role == role || broker.Role == BrokerRoleBoth {
			result = append(result, broker)
		}
	}

	return result
}

// RouteMessage routes a message between brokers based on rules
func (m *Manager) RouteMessage(msg *RoutedMessage) error {
	if msg == nil {
		return fmt.Errorf("message is nil")
	}

	m.mu.RLock()
	targetBroker, exists := m.brokers[msg.TargetBroker]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("target broker %s not found", msg.TargetBroker)
	}

	return targetBroker.Publish(msg.Topic, msg.Payload, msg.QoS, msg.Retain)
}

// handleConnect handles broker connection events
func (b *ManagedBroker) handleConnect() {
	b.mu.Lock()
	b.State = BrokerStateConnected
	b.ConnectTime = time.Now()
	b.mu.Unlock()

	b.Logger.Info("broker connected",
		"id", b.ID,
		"address", b.Config.Address)

	if b.Metrics != nil {
		b.Metrics.SetMQTTConnectionStatus(true)
	}

	// Resubscribe to topics
	b.Subscriptions.mu.RLock()
	topics := make(map[string]byte, len(b.Subscriptions.topics))
	for topic, qos := range b.Subscriptions.topics {
		topics[topic] = qos
	}
	b.Subscriptions.mu.RUnlock()

	for topic, qos := range topics {
		if err := b.Subscribe(topic, qos); err != nil {
			b.Logger.Error("failed to resubscribe",
				"broker", b.ID,
				"topic", topic,
				"error", err)
		}
	}
}

// handleDisconnect handles broker disconnection events
func (b *ManagedBroker) handleDisconnect(err error) {
	b.mu.Lock()
	b.State = BrokerStateDisconnected
	b.LastError = err
	b.Stats.LastReconnect = time.Now()
	b.mu.Unlock()

	b.Logger.Error("broker disconnected",
		"id", b.ID,
		"error", err)

	if b.Metrics != nil {
		b.Metrics.SetMQTTConnectionStatus(false)
	}
}

// handleReconnecting handles broker reconnection events
func (b *ManagedBroker) handleReconnecting() {
	b.mu.Lock()
	b.State = BrokerStateReconnecting
	b.mu.Unlock()

	b.Logger.Info("broker reconnecting",
		"id", b.ID,
		"address", b.Config.Address)

	if b.Metrics != nil {
		b.Metrics.IncMQTTReconnects()
	}
}

// Connect establishes the broker connection
func (b *ManagedBroker) Connect(ctx context.Context) error {
	b.mu.Lock()
	b.State = BrokerStateConnecting
	b.mu.Unlock()

	token := b.Client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("connection timeout")
	}
	if err := token.Error(); err != nil {
		b.mu.Lock()
		b.State = BrokerStateError
		b.LastError = err
		b.mu.Unlock()
		return fmt.Errorf("connection failed: %w", err)
	}

	return nil
}

// Disconnect gracefully closes the broker connection
func (b *ManagedBroker) Disconnect(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.State == BrokerStateDisconnected {
		return nil
	}

	b.Client.Disconnect(250)
	b.State = BrokerStateDisconnected
	return nil
}

// Subscribe adds a topic subscription
func (b *ManagedBroker) Subscribe(topic string, qos byte) error {
	if !b.IsConnected() {
		return fmt.Errorf("broker not connected")
	}

	token := b.Client.Subscribe(topic, qos, nil)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("subscription timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("subscription failed: %w", err)
	}

	b.Subscriptions.mu.Lock()
	b.Subscriptions.topics[topic] = qos
	b.Subscriptions.mu.Unlock()

	return nil
}

// Unsubscribe removes a topic subscription
func (b *ManagedBroker) Unsubscribe(topic string) error {
	if !b.IsConnected() {
		return fmt.Errorf("broker not connected")
	}

	token := b.Client.Unsubscribe(topic)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("unsubscribe timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("unsubscribe failed: %w", err)
	}

	b.Subscriptions.mu.Lock()
	delete(b.Subscriptions.topics, topic)
	b.Subscriptions.mu.Unlock()

	return nil
}

// Publish publishes a message to the broker
func (b *ManagedBroker) Publish(topic string, payload []byte, qos byte, retain bool) error {
	if !b.IsConnected() {
		return fmt.Errorf("broker not connected")
	}

	token := b.Client.Publish(topic, qos, retain, payload)
	if !token.WaitTimeout(5 * time.Second) {
		return fmt.Errorf("publish timeout")
	}
	if err := token.Error(); err != nil {
		return fmt.Errorf("publish failed: %w", err)
	}

	return nil
}

// IsConnected returns the current connection state
func (b *ManagedBroker) IsConnected() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.State == BrokerStateConnected && b.Client.IsConnected()
}

// GetStats returns current broker statistics
func (b *ManagedBroker) GetStats() BrokerStats {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.Stats
}
