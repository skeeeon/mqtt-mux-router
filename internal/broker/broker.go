package broker

import (
    "context"
    "time"
    "mqtt-mux-router/internal/rule"
)

// Broker defines the core interface for message broker implementations
type Broker interface {
    // Start initializes the broker with rules and begins processing
    Start(ctx context.Context, rules []rule.Rule) error
    // Close shuts down the broker and releases resources
    Close()
    // GetStats returns current broker statistics
    GetStats() BrokerStats
}

// BrokerStats contains statistics about broker operation
type BrokerStats struct {
    MessagesReceived  uint64
    MessagesPublished uint64
    LastReconnect     time.Time
    Errors            uint64
}
