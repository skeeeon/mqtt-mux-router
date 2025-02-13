//file: internal/broker/types_test.go
package broker

import (
	"crypto/tls"
	"testing"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"mqtt-mux-router/config"
)

func TestBrokerRole(t *testing.T) {
	tests := []struct {
		name  string
		role  BrokerRole
		valid bool
	}{
		{"Source role", BrokerRoleSource, true},
		{"Target role", BrokerRoleTarget, true},
		{"Both role", BrokerRoleBoth, true},
		{"Invalid role", BrokerRole("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.role == BrokerRoleSource ||
				tt.role == BrokerRoleTarget ||
				tt.role == BrokerRoleBoth

			if valid != tt.valid {
				t.Errorf("BrokerRole %s validity = %v, want %v",
					tt.role, valid, tt.valid)
			}
		})
	}
}

func TestBrokerState(t *testing.T) {
	tests := []struct {
		name  string
		state BrokerState
		valid bool
	}{
		{"Disconnected state", BrokerStateDisconnected, true},
		{"Connecting state", BrokerStateConnecting, true},
		{"Connected state", BrokerStateConnected, true},
		{"Reconnecting state", BrokerStateReconnecting, true},
		{"Error state", BrokerStateError, true},
		{"Invalid state", BrokerState("invalid"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			valid := tt.state == BrokerStateDisconnected ||
				tt.state == BrokerStateConnecting ||
				tt.state == BrokerStateConnected ||
				tt.state == BrokerStateReconnecting ||
				tt.state == BrokerStateError

			if valid != tt.valid {
				t.Errorf("BrokerState %s validity = %v, want %v",
					tt.state, valid, tt.valid)
			}
		})
	}
}

func TestBrokerOptions(t *testing.T) {
	opts := mqtt.NewClientOptions()
	broker := &ManagedBroker{
		ClientOptions: opts,
		Config:       &config.BrokerConfig{},
	}

	t.Run("WithTLSConfig", func(t *testing.T) {
		tlsConfig := &tls.Config{}
		opt := WithTLSConfig(tlsConfig)
		err := opt(broker)
		if err != nil {
			t.Errorf("WithTLSConfig returned error: %v", err)
		}
	})

	t.Run("WithReconnectDelay", func(t *testing.T) {
		delay := 5 * time.Second
		opt := WithReconnectDelay(delay)
		err := opt(broker)
		if err != nil {
			t.Errorf("WithReconnectDelay returned error: %v", err)
		}
	})

	t.Run("WithClientID", func(t *testing.T) {
		clientID := "test-client"
		opt := WithClientID(clientID)
		err := opt(broker)
		if err != nil {
			t.Errorf("WithClientID returned error: %v", err)
		}
		
		// Verify the client ID was set
		if broker.ClientOptions.ClientID != clientID {
			t.Errorf("ClientID = %s, want %s", broker.ClientOptions.ClientID, clientID)
		}
	})

	t.Run("Options with nil ClientOptions", func(t *testing.T) {
		broker := &ManagedBroker{
			ClientOptions: nil,
			Config:       &config.BrokerConfig{},
		}

		tests := []struct {
			name string
			opt  BrokerOption
		}{
			{"TLS Config", WithTLSConfig(&tls.Config{})},
			{"Reconnect Delay", WithReconnectDelay(5 * time.Second)},
			{"Client ID", WithClientID("test-client")},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				err := tt.opt(broker)
				if err != nil {
					t.Errorf("%s returned error with nil ClientOptions: %v", tt.name, err)
				}
			})
		}
	})
}

func TestRoutedMessage(t *testing.T) {
	msg := &RoutedMessage{
		SourceBroker: "source1",
		TargetBroker: "target1",
		Topic:        "test/topic",
		Payload:      []byte("test payload"),
		QoS:          1,
		Retain:       true,
		Timestamp:    time.Now(),
	}

	t.Run("Field validation", func(t *testing.T) {
		if msg.SourceBroker != "source1" {
			t.Errorf("SourceBroker = %s, want source1", msg.SourceBroker)
		}
		if msg.TargetBroker != "target1" {
			t.Errorf("TargetBroker = %s, want target1", msg.TargetBroker)
		}
		if msg.Topic != "test/topic" {
			t.Errorf("Topic = %s, want test/topic", msg.Topic)
		}
		if string(msg.Payload) != "test payload" {
			t.Errorf("Payload = %s, want test payload", string(msg.Payload))
		}
		if msg.QoS != 1 {
			t.Errorf("QoS = %d, want 1", msg.QoS)
		}
		if !msg.Retain {
			t.Error("Retain = false, want true")
		}
		if msg.Timestamp.IsZero() {
			t.Error("Timestamp is zero")
		}
	})
}

func TestSubscriptionManager(t *testing.T) {
	sm := &SubscriptionManager{
		topics:    make(map[string]byte),
		wildcards: &TopicTree{
			root: &TopicNode{
				children: make(map[string]*TopicNode),
			},
		},
	}

	t.Run("Initial state", func(t *testing.T) {
		if len(sm.topics) != 0 {
			t.Errorf("Initial topics map length = %d, want 0", len(sm.topics))
		}
		if sm.wildcards == nil {
			t.Error("Wildcards TopicTree is nil")
		}
		if sm.wildcards.root == nil {
			t.Error("TopicTree root is nil")
		}
		if sm.wildcards.root.children == nil {
			t.Error("TopicTree root children map is nil")
		}
	})
}
