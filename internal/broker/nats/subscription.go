package nats

import (
	"fmt"
	"sync"
	"sync/atomic"

	"github.com/nats-io/nats.go"
	"mqtt-mux-router/internal/metrics"
)

// SubscriptionManagerImpl implements SubscriptionManager for NATS
type SubscriptionManagerImpl struct {
	broker     *NATSBroker
	conn       ConnectionManager
	pub        Publisher
	topics     []string
	subs       map[string]*nats.Subscription
	subscribed bool
	mu         sync.RWMutex
}

// NewSubscriptionManager creates a new NATS subscription manager
func NewSubscriptionManager(broker *NATSBroker, conn ConnectionManager, pub Publisher) SubscriptionManager {
	return &SubscriptionManagerImpl{
		broker: broker,
		conn:   conn,
		pub:    pub,
		topics: make([]string, 0),
		subs:   make(map[string]*nats.Subscription),
	}
}

// Subscribe subscribes to the provided topics
func (s *SubscriptionManagerImpl) Subscribe(topics []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.conn.IsConnected() {
		return fmt.Errorf("not connected to NATS server")
	}

	s.topics = topics
	s.broker.logger.Info("subscribing to topics", "count", len(topics))

	for _, topic := range topics {
		if err := s.subscribeTopic(topic); err != nil {
			s.broker.logger.Error("failed to subscribe to topic",
				"topic", topic,
				"error", err)
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, err)
		}
		s.broker.logger.Debug("subscribed to topic", 
			"topic", topic, 
			"subject", ToNATSSubject(topic))
	}

	s.subscribed = true
	return nil
}

// subscribeTopic handles subscription to a single topic
func (s *SubscriptionManagerImpl) subscribeTopic(topic string) error {
	// Convert MQTT topic to NATS subject
	subject := ToNATSSubject(topic)

	// Subscribe to the NATS subject
	natConn := s.conn.GetConnection()
	sub, err := natConn.Subscribe(subject, func(msg *nats.Msg) {
		s.handleMessage(topic, msg)
	})

	if err != nil {
		return err
	}

	// Store subscription for later cleanup
	s.subs[topic] = sub

	return nil
}

// Unsubscribe removes subscriptions for provided topics
func (s *SubscriptionManagerImpl) Unsubscribe(topics []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.conn.IsConnected() {
		return fmt.Errorf("not connected to NATS server")
	}

	for _, topic := range topics {
		if sub, exists := s.subs[topic]; exists {
			if err := sub.Unsubscribe(); err != nil {
				s.broker.logger.Error("failed to unsubscribe from topic",
					"topic", topic,
					"error", err)
				return fmt.Errorf("failed to unsubscribe from topic %s: %w", topic, err)
			}
			delete(s.subs, topic)
			s.broker.logger.Debug("unsubscribed from topic", "topic", topic)
		}
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

// UnsubscribeAll unsubscribes from all topics
func (s *SubscriptionManagerImpl) UnsubscribeAll() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for topic, sub := range s.subs {
		if err := sub.Unsubscribe(); err != nil {
			s.broker.logger.Error("failed to unsubscribe from topic",
				"topic", topic,
				"error", err)
		} else {
			s.broker.logger.Debug("unsubscribed from topic", "topic", topic)
		}
	}

	// Clear the subscriptions map
	s.subs = make(map[string]*nats.Subscription)
	s.topics = make([]string, 0)
	s.subscribed = false

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

// handleMessage processes a received NATS message
func (s *SubscriptionManagerImpl) handleMessage(originalTopic string, msg *nats.Msg) {
	// Update statistics
	atomic.AddUint64(&s.broker.stats.MessagesReceived, 1)

	s.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
		m.IncMessagesTotal("received")
	})

	s.broker.logger.Debug("processing message",
		"topic", originalTopic,
		"subject", msg.Subject,
		"payloadSize", len(msg.Data))

	// Process the message with the rule processor
	// Using the original MQTT-format topic
	actions, err := s.broker.processor.Process(originalTopic, msg.Data)
	if err != nil {
		atomic.AddUint64(&s.broker.stats.Errors, 1)
		s.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
			m.IncMessagesTotal("error")
		})
		s.broker.logger.Error("failed to process message",
			"error", err,
			"topic", originalTopic)
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
