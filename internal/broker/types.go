//file: internal/broker/types.go
// Package broker provides MQTT broker connection and message routing functionality
package broker

import (
	"context"
	"crypto/tls"
	"sync"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
)

// BrokerRole represents the role of a broker (source, target, or both)
type BrokerRole string

const (
	// BrokerRoleSource indicates a broker that only provides source messages
	BrokerRoleSource BrokerRole = "source"
	// BrokerRoleTarget indicates a broker that only receives routed messages
	BrokerRoleTarget BrokerRole = "target"
	// BrokerRoleBoth indicates a broker that can act as both source and target
	BrokerRoleBoth BrokerRole = "both"
)

// BrokerState represents the current state of a broker connection
type BrokerState string

const (
	// BrokerStateDisconnected indicates the broker is not connected
	BrokerStateDisconnected BrokerState = "disconnected"
	// BrokerStateConnecting indicates the broker is attempting to connect
	BrokerStateConnecting BrokerState = "connecting"
	// BrokerStateConnected indicates the broker is connected
	BrokerStateConnected BrokerState = "connected"
	// BrokerStateReconnecting indicates the broker is attempting to reconnect
	BrokerStateReconnecting BrokerState = "reconnecting"
	// BrokerStateError indicates the broker is in an error state
	BrokerStateError BrokerState = "error"
)

// BrokerManager interface defines the methods for managing multiple broker connections
type BrokerManager interface {
	// Start initializes and connects all managed brokers
	Start(ctx context.Context) error

	// Stop gracefully disconnects all managed brokers
	Stop(ctx context.Context) error

	// GetBroker returns a broker by ID
	GetBroker(id string) (BrokerConnection, error)

	// GetBrokersByRole returns all brokers matching a specific role
	GetBrokersByRole(role BrokerRole) []BrokerConnection

	// RouteMessage routes a message between brokers based on rules
	RouteMessage(msg *RoutedMessage) error
}

// BrokerConnection interface defines the methods for managing a single broker connection
type BrokerConnection interface {
	// Connect establishes the broker connection
	Connect(ctx context.Context) error

	// Disconnect gracefully closes the broker connection
	Disconnect(ctx context.Context) error

	// Subscribe adds a topic subscription
	Subscribe(topic string, qos byte) error

	// Unsubscribe removes a topic subscription
	Unsubscribe(topic string) error

	// Publish publishes a message to the broker
	Publish(topic string, payload []byte, qos byte, retain bool) error

	// IsConnected returns the current connection state
	IsConnected() bool

	// GetStats returns current broker statistics
	GetStats() BrokerStats

	// GetID returns the broker's ID
	GetID() string

	// GetRole returns the broker's role
	GetRole() BrokerRole
}

// ManagedBroker represents a managed MQTT broker connection
type ManagedBroker struct {
	ID            string
	Role          BrokerRole
	State         BrokerState
	Client        mqtt.Client
	ClientOptions *mqtt.ClientOptions
	Subscriptions *SubscriptionManager
	Config        *config.BrokerConfig
	Logger        *logger.Logger
	Metrics       *metrics.Metrics
	LastError     error
	ConnectTime   time.Time
	Stats         BrokerStats
	mu            sync.RWMutex
}

// BrokerStats holds statistics for a broker connection
type BrokerStats struct {
	MessagesReceived  uint64
	MessagesPublished uint64
	LastReconnect     time.Time
	Errors            uint64
}

// RoutedMessage represents a message being routed between brokers
type RoutedMessage struct {
	SourceBroker string
	TargetBroker string
	Topic        string
	Payload      []byte
	QoS          byte
	Retain       bool
	Timestamp    time.Time
}

// TopicMatch represents a matched subscription
type TopicMatch struct {
	Topic string
	QoS   byte
}

// SubscriptionManager handles topic subscriptions and matching
type SubscriptionManager struct {
	topics    map[string]byte // topic -> QoS
	wildcards *TopicTree
	mu        sync.RWMutex
}

// TopicTree implements a prefix tree for efficient wildcard topic matching
type TopicTree struct {
	root *TopicNode
	mu   sync.RWMutex
}

// TopicNode represents a node in the topic tree
type TopicNode struct {
	segment    string
	isWildcard bool
	isEnd      bool
	qos        byte
	children   map[string]*TopicNode
}

// BrokerOption defines a function type for configuring broker options
type BrokerOption func(*ManagedBroker) error

// WithTLSConfig sets the TLS configuration for a broker
func WithTLSConfig(tlsConfig *tls.Config) BrokerOption {
	return func(b *ManagedBroker) error {
		if b.ClientOptions == nil {
			return nil
		}
		b.ClientOptions.SetTLSConfig(tlsConfig)
		return nil
	}
}

// WithReconnectDelay sets the reconnection delay for a broker
func WithReconnectDelay(delay time.Duration) BrokerOption {
	return func(b *ManagedBroker) error {
		if b.ClientOptions == nil {
			return nil
		}
		b.ClientOptions.SetConnectRetry(true)
		b.ClientOptions.SetConnectRetryInterval(delay)
		return nil
	}
}

// WithClientID sets the client ID for a broker
func WithClientID(clientID string) BrokerOption {
	return func(b *ManagedBroker) error {
		if b.ClientOptions == nil {
			return nil
		}
		b.ClientOptions.SetClientID(clientID)
		return nil
	}
}

// GetID returns the broker's ID
func (b *ManagedBroker) GetID() string {
	return b.ID
}

// GetRole returns the broker's role
func (b *ManagedBroker) GetRole() BrokerRole {
	return b.Role
}
