package broker

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"sync"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/rule"
)

type Broker struct {
	client     mqtt.Client
	logger     *logger.Logger
	config     *config.Config
	processor  *rule.Processor
	topics     []string
	subscribed bool
	mu         sync.RWMutex
	wg         sync.WaitGroup
}

func NewBroker(cfg *config.Config, log *logger.Logger) (*Broker, error) {
	broker := &Broker{
		logger: log,
		config: cfg,
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MQTT.Broker).
		SetClientID(cfg.MQTT.ClientID).
		SetUsername(cfg.MQTT.Username).
		SetPassword(cfg.MQTT.Password).
		SetCleanSession(true).
		SetAutoReconnect(true)

	// Add connect handler to resubscribe to topics
	opts.OnConnect = func(client mqtt.Client) {
		broker.logger.Info("mqtt client connected", "broker", cfg.MQTT.Broker)
		broker.handleReconnect()
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		broker.logger.Error("mqtt connection lost", "error", err)
		broker.mu.Lock()
		broker.subscribed = false
		broker.mu.Unlock()
	}

	opts.OnReconnecting = func(client mqtt.Client, opts *mqtt.ClientOptions) {
		broker.logger.Info("mqtt client reconnecting", "broker", cfg.MQTT.Broker)
	}

	if cfg.MQTT.TLS.Enable {
		tlsConfig, err := newTLSConfig(cfg.MQTT.TLS.CertFile, cfg.MQTT.TLS.KeyFile, cfg.MQTT.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	client := mqtt.NewClient(opts)
	broker.client = client

	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to broker: %w", token.Error())
	}

	return broker, nil
}

func (b *Broker) Start(ctx context.Context, processor *rule.Processor) error {
	b.processor = processor
	b.topics = processor.GetTopics()
	return b.subscribe()
}

func (b *Broker) handleReconnect() {
	b.mu.RLock()
	needsSubscribe := !b.subscribed && b.processor != nil
	b.mu.RUnlock()

	if needsSubscribe {
		if err := b.subscribe(); err != nil {
			b.logger.Error("failed to resubscribe after reconnection", "error", err)
		}
	}
}

func (b *Broker) subscribe() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.subscribed {
		return nil
	}

	b.logger.Info("subscribing to topics", "count", len(b.topics))
	
	for _, topic := range b.topics {
		if token := b.client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
			b.handleMessage(b.processor, msg)
		}); token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
		}
		b.logger.Debug("subscribed to topic", "topic", topic)
	}

	b.subscribed = true
	return nil
}

func (b *Broker) handleMessage(processor *rule.Processor, msg mqtt.Message) {
	b.logger.Debug("processing message", 
		"topic", msg.Topic(),
		"payload", string(msg.Payload()))

	actions, err := processor.Process(msg.Topic(), msg.Payload())
	if err != nil {
		b.logger.Error("failed to process message",
			"error", err,
			"topic", msg.Topic())
		return
	}

	for _, action := range actions {
		if token := b.client.Publish(action.Topic, 0, false, action.Payload); token.Wait() && token.Error() != nil {
			b.logger.Error("failed to publish message",
				"error", token.Error(),
				"topic", action.Topic)
		} else {
			b.logger.Debug("published message",
				"from", msg.Topic(),
				"to", action.Topic,
				"payload", string(action.Payload))
		}
	}
}

func (b *Broker) Close() {
	b.logger.Info("shutting down mqtt client")
	b.client.Disconnect(250)
}

func newTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, err
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, err
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM(caCert) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:     caCertPool,
	}, nil
}
