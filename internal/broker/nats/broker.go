package nats

import (
	"context"
	"fmt"
	"sync"
	"time"

	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/broker"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
	"mqtt-mux-router/internal/rule"
)

// NATSBroker implements the broker.Broker interface for NATS
type NATSBroker struct {
	logger    *logger.Logger
	config    *config.Config
	processor *rule.Processor
	metrics   *metrics.Metrics
	stats     broker.BrokerStats

	conn      ConnectionManager
	sub       SubscriptionManager
	pub       Publisher

	// Store rules for reconnection
	rules     []rule.Rule

	mu        sync.RWMutex
	wg        sync.WaitGroup
}

// BrokerConfig contains NATS broker configuration
type BrokerConfig struct {
	ProcessorWorkers int
	QueueSize        int
	BatchSize        int
}

// NewBroker creates a new NATS broker instance
func NewBroker(cfg *config.Config, log *logger.Logger, brokerCfg BrokerConfig, metricsService *metrics.Metrics) (broker.Broker, error) {
	processorCfg := rule.ProcessorConfig{
		Workers:   brokerCfg.ProcessorWorkers,
		QueueSize: brokerCfg.QueueSize,
		BatchSize: brokerCfg.BatchSize,
	}

	processor := rule.NewProcessor(processorCfg, log, metricsService)

	b := &NATSBroker{
		logger:    log,
		config:    cfg,
		processor: processor,
		metrics:   metricsService,
		stats: broker.BrokerStats{
			LastReconnect: time.Now(),
		},
	}

	// Initialize connection manager first
	var err error
	b.conn, err = NewConnectionManager(b)
	if err != nil {
		return nil, fmt.Errorf("failed to create connection manager: %w", err)
	}

	// Initialize publisher
	b.pub = NewPublisher(b, b.conn)

	// Initialize subscription manager
	b.sub = NewSubscriptionManager(b, b.conn, b.pub)

	return b, nil
}

// Start implements broker.Broker interface
func (b *NATSBroker) Start(ctx context.Context, rules []rule.Rule) error {
	b.mu.Lock()
	// Store rules for reconnection
	b.rules = make([]rule.Rule, len(rules))
	copy(b.rules, rules)
	b.mu.Unlock()

	if err := b.processor.LoadRules(rules); err != nil {
		return fmt.Errorf("failed to load rules: %w", err)
	}

	// Extract unique topics from rules
	topics := make(map[string]struct{})
	for _, rule := range rules {
		topics[rule.Topic] = struct{}{}
	}

	topicList := make([]string, 0, len(topics))
	for topic := range topics {
		topicList = append(topicList, topic)
	}

	// Subscribe to topics
	if err := b.sub.Subscribe(topicList); err != nil {
		return fmt.Errorf("failed to subscribe to topics: %w", err)
	}

	if b.metrics != nil {
		b.metrics.SetRulesActive(float64(len(rules)))
	}

	// Start a goroutine to monitor context cancellation
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		<-ctx.Done()
		b.logger.Info("context done, unsubscribing from all subjects")
		b.sub.UnsubscribeAll()
	}()

	return nil
}

// Close implements broker.Broker interface
func (b *NATSBroker) Close() {
	b.logger.Info("shutting down NATS broker")

	// Unsubscribe from all topics
	b.sub.UnsubscribeAll()

	// Close NATS connection
	b.conn.Disconnect()

	// Close the processor
	b.processor.Close()

	// Wait for all goroutines to complete
	b.wg.Wait()
}

// GetStats implements broker.Broker interface
func (b *NATSBroker) GetStats() broker.BrokerStats {
	return b.stats
}

// RestoreState restores rules and subscriptions after reconnection
func (b *NATSBroker) RestoreState() {
	b.mu.RLock()
	rules := make([]rule.Rule, len(b.rules))
	copy(rules, b.rules)
	b.mu.RUnlock()

	b.logger.Info("restoring rules after reconnection", "ruleCount", len(rules))

	// Reload rules
	if err := b.processor.LoadRules(rules); err != nil {
		b.logger.Error("failed to restore rules after reconnection", "error", err)
		return
	}

	// Extract unique topics
	topics := make(map[string]struct{})
	for _, rule := range rules {
		topics[rule.Topic] = struct{}{}
	}

	topicList := make([]string, 0, len(topics))
	for topic := range topics {
		topicList = append(topicList, topic)
	}

	// Clear existing subscriptions
	b.sub.UnsubscribeAll()

	// Resubscribe to all topics
	if err := b.sub.Subscribe(topicList); err != nil {
		b.logger.Error("failed to resubscribe to topics after reconnection", "error", err)
		return
	}

	b.logger.Info("successfully restored rules and subscriptions")

	if b.metrics != nil {
		b.metrics.SetRulesActive(float64(len(rules)))
	}
}

// safeMetricsUpdate safely updates metrics if they are enabled
func (b *NATSBroker) safeMetricsUpdate(fn func(*metrics.Metrics)) {
	if b.metrics != nil {
		fn(b.metrics)
	}
}
