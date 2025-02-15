package mqtt

import (
    "fmt"
    "sync"
    "sync/atomic"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "mqtt-mux-router/internal/metrics"
)

// SubscriptionManagerImpl implements the SubscriptionManager interface
type SubscriptionManagerImpl struct {
    broker     *MQTTBroker
    conn       ConnectionManager
    pub        Publisher
    topics     []string
    subscribed bool
    mu         sync.RWMutex
}

// NewSubscriptionManager creates a new subscription manager
func NewSubscriptionManager(broker *MQTTBroker) SubscriptionManager {
    return &SubscriptionManagerImpl{
        broker: broker,
        conn:   broker.conn,
        pub:    broker.pub,
        topics: make([]string, 0),
    }
}

// Subscribe subscribes to the provided topics
func (s *SubscriptionManagerImpl) Subscribe(topics []string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if !s.conn.IsConnected() {
        return fmt.Errorf("not connected to broker")
    }

    s.topics = topics
    s.broker.logger.Info("subscribing to topics", "count", len(topics))

    for _, topic := range topics {
        if token := s.conn.GetClient().Subscribe(topic, 0, s.HandleMessage); token.Wait() && token.Error() != nil {
            s.broker.logger.Error("failed to subscribe to topic",
                "topic", topic,
                "error", token.Error())
            return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
        }
        s.broker.logger.Debug("subscribed to topic", "topic", topic)
    }

    s.subscribed = true
    return nil
}

// Unsubscribe removes subscriptions for the provided topics
func (s *SubscriptionManagerImpl) Unsubscribe(topics []string) error {
    s.mu.Lock()
    defer s.mu.Unlock()

    if !s.conn.IsConnected() {
        return fmt.Errorf("not connected to broker")
    }

    for _, topic := range topics {
        if token := s.conn.GetClient().Unsubscribe(topic); token.Wait() && token.Error() != nil {
            s.broker.logger.Error("failed to unsubscribe from topic",
                "topic", topic,
                "error", token.Error())
            return fmt.Errorf("failed to unsubscribe from topic %s: %w", topic, token.Error())
        }
        s.broker.logger.Debug("unsubscribed from topic", "topic", topic)
    }

    // Update topics list
    remaining := make([]string, 0)
    topicSet := make(map[string]struct{})
    for _, t := range topics {
        topicSet[t] = struct{}{}
    }

    for _, t := range s.topics {
        if _, exists := topicSet[t]; !exists {
            remaining = append(remaining, t)
        }
    }
    s.topics = remaining

    if len(s.topics) == 0 {
        s.subscribed = false
    }

    return nil
}

// HandleMessage processes received MQTT messages
func (s *SubscriptionManagerImpl) HandleMessage(client mqtt.Client, msg mqtt.Message) {
    atomic.AddUint64(&s.broker.stats.MessagesReceived, 1)

    s.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.IncMessagesTotal("received")
    })

    s.broker.logger.Debug("processing message",
        "topic", msg.Topic(),
        "payloadSize", len(msg.Payload()))

    actions, err := s.broker.processor.Process(msg.Topic(), msg.Payload())
    if err != nil {
        atomic.AddUint64(&s.broker.stats.Errors, 1)
        s.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
            m.IncMessagesTotal("error")
        })
        s.broker.logger.Error("failed to process message",
            "error", err,
            "topic", msg.Topic())
        return
    }

    s.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.IncMessagesTotal("processed")
    })

    // Publish resulting actions
    for _, action := range actions {
        if err := s.pub.PublishAction(action); err != nil {
            s.broker.logger.Error("failed to publish action",
                "error", err,
                "topic", action.Topic)
        }
    }

    // Update queue metrics if enabled
    s.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.SetMessageQueueDepth(float64(len(s.broker.processor.GetJobChannel())))
        m.SetProcessingBacklog(float64(
            atomic.LoadUint64(&s.broker.stats.MessagesReceived) -
            atomic.LoadUint64(&s.broker.stats.MessagesPublished)))
    })
}

// ResubscribeAll resubscribes to all topics after a reconnection
func (s *SubscriptionManagerImpl) ResubscribeAll() error {
    s.mu.RLock()
    needsSubscribe := !s.subscribed && len(s.topics) > 0
    topics := make([]string, len(s.topics))
    copy(topics, s.topics)
    s.mu.RUnlock()

    if needsSubscribe {
        return s.Subscribe(topics)
    }
    return nil
}

// GetSubscribedTopics returns the list of currently subscribed topics
func (s *SubscriptionManagerImpl) GetSubscribedTopics() []string {
    s.mu.RLock()
    defer s.mu.RUnlock()
    
    topics := make([]string, len(s.topics))
    copy(topics, s.topics)
    return topics
}

// IsSubscribed returns whether there are active subscriptions
func (s *SubscriptionManagerImpl) IsSubscribed() bool {
    s.mu.RLock()
    defer s.mu.RUnlock()
    return s.subscribed
}
