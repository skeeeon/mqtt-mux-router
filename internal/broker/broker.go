//file: internal/broker/broker.go

package broker

import (
    "context"
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"
    "sync"
    "sync/atomic"
    "time"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "mqtt-mux-router/config"
    "mqtt-mux-router/internal/logger"
    "mqtt-mux-router/internal/metrics"
    "mqtt-mux-router/internal/rule"
)

type Broker struct {
    client     mqtt.Client
    logger     *logger.Logger
    config     *config.Config
    processor  *rule.Processor
    topics     []string
    subscribed bool
    stats      BrokerStats
    metrics    *metrics.Metrics
    mu         sync.RWMutex
    wg         sync.WaitGroup
}

type BrokerStats struct {
    MessagesReceived  uint64
    MessagesPublished uint64
    LastReconnect     time.Time
    Errors            uint64
}

type BrokerConfig struct {
    ProcessorWorkers int
    QueueSize       int
    BatchSize       int
}

func NewBroker(cfg *config.Config, log *logger.Logger, brokerCfg BrokerConfig, metrics *metrics.Metrics) (*Broker, error) {
    processorCfg := rule.ProcessorConfig{
        Workers:    brokerCfg.ProcessorWorkers,
        QueueSize:  brokerCfg.QueueSize,
        BatchSize:  brokerCfg.BatchSize,
    }

    processor := rule.NewProcessor(processorCfg, log, metrics)

    broker := &Broker{
        logger:    log,
        config:    cfg,
        processor: processor,
        metrics:   metrics,
    }

    opts := mqtt.NewClientOptions().
        AddBroker(cfg.MQTT.Broker).
        SetClientID(cfg.MQTT.ClientID).
        SetUsername(cfg.MQTT.Username).
        SetPassword(cfg.MQTT.Password).
        SetCleanSession(true).
        SetAutoReconnect(true)

    opts.OnConnect = func(client mqtt.Client) {
        broker.handleConnect()
    }

    opts.OnConnectionLost = func(client mqtt.Client, err error) {
        broker.handleDisconnect(err)
    }

    opts.OnReconnecting = func(client mqtt.Client, opts *mqtt.ClientOptions) {
        broker.logger.Info("mqtt client reconnecting", "broker", cfg.MQTT.Broker)
        broker.metrics.IncMQTTReconnects()
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

func (b *Broker) handleConnect() {
    b.logger.Info("mqtt client connected", "broker", b.config.MQTT.Broker)
    b.stats.LastReconnect = time.Now()
    b.metrics.SetMQTTConnectionStatus(true)

    b.mu.RLock()
    needsSubscribe := !b.subscribed && len(b.topics) > 0
    b.mu.RUnlock()

    if needsSubscribe {
        if err := b.subscribe(); err != nil {
            b.logger.Error("failed to resubscribe after reconnection", "error", err)
        }
    }
}

func (b *Broker) handleDisconnect(err error) {
    b.logger.Error("mqtt connection lost", "error", err)
    b.metrics.SetMQTTConnectionStatus(false)
    b.mu.Lock()
    b.subscribed = false
    b.mu.Unlock()
}

func (b *Broker) Start(ctx context.Context, rules []rule.Rule) error {
    if err := b.processor.LoadRules(rules); err != nil {
        return fmt.Errorf("failed to load rules: %w", err)
    }

    topics := make(map[string]struct{})
    for _, rule := range rules {
        topics[rule.Topic] = struct{}{}
    }

    b.topics = make([]string, 0, len(topics))
    for topic := range topics {
        b.topics = append(b.topics, topic)
    }

    b.metrics.SetRulesActive(float64(len(rules)))
    return b.subscribe()
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
            b.handleMessage(msg)
        }); token.Wait() && token.Error() != nil {
            return fmt.Errorf("failed to subscribe to topic %s: %w", topic, token.Error())
        }
        b.logger.Debug("subscribed to topic", "topic", topic)
    }

    b.subscribed = true
    return nil
}

func (b *Broker) handleMessage(msg mqtt.Message) {
    atomic.AddUint64(&b.stats.MessagesReceived, 1)
    b.metrics.IncMessagesTotal("received")

    b.logger.Debug("processing message",
        "topic", msg.Topic(),
        "payload", string(msg.Payload()))

    actions, err := b.processor.Process(msg.Topic(), msg.Payload())
    if err != nil {
        atomic.AddUint64(&b.stats.Errors, 1)
        b.metrics.IncMessagesTotal("error")
        b.logger.Error("failed to process message",
            "error", err,
            "topic", msg.Topic())
        return
    }

    b.metrics.IncMessagesTotal("processed")

    for _, action := range actions {
        if token := b.client.Publish(action.Topic, 0, false, action.Payload); token.Wait() && token.Error() != nil {
            atomic.AddUint64(&b.stats.Errors, 1)
            b.metrics.IncActionsTotal("error")
            b.logger.Error("failed to publish message",
                "error", token.Error(),
                "topic", action.Topic)
        } else {
            atomic.AddUint64(&b.stats.MessagesPublished, 1)
            b.metrics.IncActionsTotal("success")
            b.logger.Debug("published message",
                "from", msg.Topic(),
                "to", action.Topic,
                "payload", string(action.Payload))
        }
    }

    // Update queue metrics
    b.metrics.SetMessageQueueDepth(float64(len(b.processor.GetJobChannel())))
    b.metrics.SetProcessingBacklog(float64(
        atomic.LoadUint64(&b.stats.MessagesReceived) -
        atomic.LoadUint64(&b.stats.MessagesPublished)))
}

func (b *Broker) GetStats() BrokerStats {
    return BrokerStats{
        MessagesReceived:  atomic.LoadUint64(&b.stats.MessagesReceived),
        MessagesPublished: atomic.LoadUint64(&b.stats.MessagesPublished),
        LastReconnect:     b.stats.LastReconnect,
        Errors:            atomic.LoadUint64(&b.stats.Errors),
    }
}

func (b *Broker) Close() {
    b.logger.Info("shutting down mqtt client")
    b.client.Disconnect(250)
    b.processor.Close()
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
