package mqtt

import (
    mqtt "github.com/eclipse/paho.mqtt.golang"
    "mqtt-mux-router/internal/rule"
)

// ConnectionManager handles MQTT connection lifecycle
type ConnectionManager interface {
    Connect() error
    Disconnect()
    IsConnected() bool
    HandleReconnect() error
    GetClient() mqtt.Client
}

// SubscriptionManager handles topic subscriptions and message reception
type SubscriptionManager interface {
    Subscribe(topics []string) error
    Unsubscribe(topics []string) error
    HandleMessage(client mqtt.Client, msg mqtt.Message)
    ResubscribeAll() error
    GetSubscribedTopics() []string
    IsSubscribed() bool
}

// Publisher handles message publishing
type Publisher interface {
    Publish(topic string, payload []byte) error
    PublishAction(action *rule.Action) error
}
