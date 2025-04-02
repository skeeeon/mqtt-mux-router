package nats

import (
	"github.com/nats-io/nats.go"
	"mqtt-mux-router/internal/rule"
)

// Defining interfaces (similar to MQTT package)

// ConnectionManager handles NATS connection lifecycle
type ConnectionManager interface {
	Connect() error
	Disconnect()
	IsConnected() bool
	GetConnection() *nats.Conn
}

// SubscriptionManager handles subject subscriptions and message reception
type SubscriptionManager interface {
	Subscribe(topics []string) error
	Unsubscribe(topics []string) error
	UnsubscribeAll() error
	GetSubscribedTopics() []string
	IsSubscribed() bool
}

// Publisher handles message publishing
type Publisher interface {
	Publish(topic string, payload []byte) error
	PublishAction(action *rule.Action) error
}
