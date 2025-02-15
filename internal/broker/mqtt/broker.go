package mqtt

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

// MQTTBroker implements the broker.Broker interface for MQTT
type MQTTBroker struct {
    logger    *logger.Logger
    config    *config.Config
    processor *rule.Processor
    metrics   *metrics.Metrics
    stats     broker.BrokerStats

    conn ConnectionManager
    sub  SubscriptionManager
    pub  Publisher

    mu sync.RWMutex
    wg sync.WaitGroup
}

// BrokerConfig contains MQTT broker configuration
type BrokerConfig struct {
    ProcessorWorkers int
    QueueSize       int
    BatchSize       int
}

// NewBroker creates a new MQTT broker instance
func NewBroker(cfg *config.Config, log *logger.Logger, brokerCfg BrokerConfig, metricsService *metrics.Metrics) (broker.Broker, error) {
    processorCfg := rule.ProcessorConfig{
        Workers:   brokerCfg.ProcessorWorkers,
        QueueSize: brokerCfg.QueueSize,
        BatchSize: brokerCfg.BatchSize,
    }

    processor := rule.NewProcessor(processorCfg, log, metricsService)

    b := &MQTTBroker{
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

    // Initialize publisher before subscription manager since it's needed for message handling
    b.pub = NewPublisher(b)

    // Initialize subscription manager last since it depends on both connection and publisher
    b.sub = NewSubscriptionManager(b)

    return b, nil
}

// Start implements broker.Broker interface
func (b *MQTTBroker) Start(ctx context.Context, rules []rule.Rule) error {
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

    if err := b.sub.Subscribe(topicList); err != nil {
        return fmt.Errorf("failed to subscribe to topics: %w", err)
    }

    if b.metrics != nil {
        b.metrics.SetRulesActive(float64(len(rules)))
    }

    return nil
}

// Close implements broker.Broker interface
func (b *MQTTBroker) Close() {
    b.logger.Info("shutting down mqtt broker")
    b.conn.Disconnect()
    b.processor.Close()
}

// GetStats implements broker.Broker interface
func (b *MQTTBroker) GetStats() broker.BrokerStats {
    return b.stats
}

// safeMetricsUpdate safely updates metrics if they are enabled
func (b *MQTTBroker) safeMetricsUpdate(fn func(*metrics.Metrics)) {
    if b.metrics != nil {
        fn(b.metrics)
    }
}
