// File: internal/broker/broker.go
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
	client  mqtt.Client
	logger  *logger.Logger
	config  *config.Config
	wg      sync.WaitGroup
}

func NewBroker(cfg *config.Config, log *logger.Logger) (*Broker, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.MQTT.Broker).
		SetClientID(cfg.MQTT.ClientID).
		SetUsername(cfg.MQTT.Username).
		SetPassword(cfg.MQTT.Password).
		SetCleanSession(true).
		SetAutoReconnect(true)

	// Add connect and disconnect handlers for debugging
	opts.OnConnect = func(client mqtt.Client) {
		log.Debug("mqtt client connected successfully",
			"broker", cfg.MQTT.Broker,
			"clientId", cfg.MQTT.ClientID)
	}

	opts.OnConnectionLost = func(client mqtt.Client, err error) {
		log.Debug("mqtt connection lost",
			"broker", cfg.MQTT.Broker,
			"clientId", cfg.MQTT.ClientID,
			"error", err)
	}

	opts.OnReconnecting = func(client mqtt.Client, opts *mqtt.ClientOptions) {
		log.Debug("mqtt client attempting to reconnect",
			"broker", cfg.MQTT.Broker,
			"clientId", cfg.MQTT.ClientID)
	}

	if cfg.MQTT.TLS.Enable {
		log.Debug("configuring tls for mqtt connection",
			"certFile", cfg.MQTT.TLS.CertFile,
			"keyFile", cfg.MQTT.TLS.KeyFile,
			"caFile", cfg.MQTT.TLS.CAFile)

		tlsConfig, err := newTLSConfig(cfg.MQTT.TLS.CertFile, cfg.MQTT.TLS.KeyFile, cfg.MQTT.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	log.Debug("creating mqtt client",
		"broker", cfg.MQTT.Broker,
		"clientId", cfg.MQTT.ClientID,
		"username", cfg.MQTT.Username,
		"tlsEnabled", cfg.MQTT.TLS.Enable)

	client := mqtt.NewClient(opts)

	log.Debug("attempting mqtt connection", "broker", cfg.MQTT.Broker)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to broker: %w", token.Error())
	}

	return &Broker{
		client: client,
		logger: log,
		config: cfg,
	}, nil
}

func (b *Broker) Start(ctx context.Context, processor *rule.Processor) error {
	b.logger.Debug("starting mqtt broker service")
	
	// Subscribe to all topics from rules
	topics := processor.GetTopics()
	b.logger.Debug("subscribing to topics", "count", len(topics), "topics", topics)
	
	for _, topic := range topics {
		b.logger.Debug("subscribing to topic", "topic", topic)
		if token := b.client.Subscribe(topic, 0, func(client mqtt.Client, msg mqtt.Message) {
			b.handleMessage(processor, msg)
		}); token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
		}
		b.logger.Info("subscribed to topic", "topic", topic)
	}

	return nil
}

func (b *Broker) handleMessage(processor *rule.Processor, msg mqtt.Message) {
	b.logger.Debug("received message",
		"topic", msg.Topic(),
		"payload", string(msg.Payload()),
		"qos", msg.Qos(),
		"retained", msg.Retained(),
		"messageId", msg.MessageID())

	actions, err := processor.Process(msg.Topic(), msg.Payload())
	if err != nil {
		b.logger.Error("failed to process message",
			"error", err,
			"topic", msg.Topic(),
			"payload", string(msg.Payload()))
		return
	}

	for _, action := range actions {
		b.logger.Debug("publishing action",
			"sourceTopic", msg.Topic(),
			"targetTopic", action.Topic,
			"payload", string(action.Payload))

		if token := b.client.Publish(action.Topic, 0, false, action.Payload); token.Wait() && token.Error() != nil {
			b.logger.Error("failed to publish message",
				"error", token.Error(),
				"topic", action.Topic)
		} else {
			b.logger.Debug("successfully published message",
				"topic", action.Topic,
				"payload", string(action.Payload))
		}
	}
}

func (b *Broker) Close() {
	b.logger.Debug("disconnecting mqtt client")
	b.client.Disconnect(250)
	b.logger.Info("mqtt client disconnected")
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
