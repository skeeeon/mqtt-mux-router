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

	if cfg.MQTT.TLS.Enable {
		tlsConfig, err := newTLSConfig(cfg.MQTT.TLS.CertFile, cfg.MQTT.TLS.KeyFile, cfg.MQTT.TLS.CAFile)
		if err != nil {
			return nil, fmt.Errorf("failed to create TLS config: %w", err)
		}
		opts.SetTLSConfig(tlsConfig)
	}

	client := mqtt.NewClient(opts)
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
	// Subscribe to all topics from rules
	topics := processor.GetTopics()
	for _, topic := range topics {
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
	b.logger.Debug("received message", "topic", msg.Topic(), "payload", string(msg.Payload()))

	actions, err := processor.Process(msg.Topic(), msg.Payload())
	if err != nil {
		b.logger.Error("failed to process message", "error", err)
		return
	}

	for _, action := range actions {
		if token := b.client.Publish(action.Topic, 0, false, action.Payload); token.Wait() && token.Error() != nil {
			b.logger.Error("failed to publish message", "error", token.Error())
		}
		b.logger.Debug("published message", "topic", action.Topic, "payload", string(action.Payload))
	}
}

func (b *Broker) Close() {
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
