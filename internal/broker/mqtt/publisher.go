package mqtt

import (
    "fmt"
    "sync/atomic"

    "mqtt-mux-router/internal/metrics"
    "mqtt-mux-router/internal/rule"
)

// PublisherImpl handles MQTT message publishing
type PublisherImpl struct {
    broker *MQTTBroker
    conn   ConnectionManager
}

// NewPublisher creates a new MQTT publisher
func NewPublisher(broker *MQTTBroker) Publisher {
    return &PublisherImpl{
        broker: broker,
        conn:   broker.conn,
    }
}

// Publish sends a message to a specific topic
func (p *PublisherImpl) Publish(topic string, payload []byte) error {
    if !p.conn.IsConnected() {
        return fmt.Errorf("not connected to broker")
    }

    token := p.conn.GetClient().Publish(topic, 0, false, payload)
    if token.Wait() && token.Error() != nil {
        atomic.AddUint64(&p.broker.stats.Errors, 1)
        p.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
            m.IncActionsTotal("error")
        })
        p.broker.logger.Error("failed to publish message",
            "error", token.Error(),
            "topic", topic)
        return token.Error()
    }

    atomic.AddUint64(&p.broker.stats.MessagesPublished, 1)
    p.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.IncActionsTotal("success")
    })

    p.broker.logger.Debug("published message",
        "topic", topic,
        "payloadSize", len(payload))

    return nil
}

// PublishAction publishes a rule action
func (p *PublisherImpl) PublishAction(action *rule.Action) error {
    if action == nil {
        return fmt.Errorf("action cannot be nil")
    }

    err := p.Publish(action.Topic, []byte(action.Payload))
    if err != nil {
        p.broker.logger.Error("failed to publish action",
            "error", err,
            "topic", action.Topic)
        return fmt.Errorf("failed to publish action: %w", err)
    }

    p.broker.logger.Debug("published action",
        "topic", action.Topic,
        "payload", action.Payload)

    return nil
}
