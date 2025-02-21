package mqtt

import (
    "crypto/tls"
    "crypto/x509"
    "fmt"
    "os"
    "sync/atomic"
    "time"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "mqtt-mux-router/internal/metrics"
)

// ConnectionManagerImpl handles MQTT connection lifecycle
type ConnectionManagerImpl struct {
    broker    *MQTTBroker
    client    mqtt.Client
    connected atomic.Bool
}

// NewConnectionManager creates a new MQTT connection manager
func NewConnectionManager(broker *MQTTBroker) (ConnectionManager, error) {
    cm := &ConnectionManagerImpl{
        broker: broker,
    }

    // Create client options
    opts := mqtt.NewClientOptions().
        AddBroker(broker.config.MQTT.Broker).
        SetClientID(broker.config.MQTT.ClientID).
        SetUsername(broker.config.MQTT.Username).
        SetPassword(broker.config.MQTT.Password).
        SetCleanSession(true).
        SetAutoReconnect(true).
        SetMaxReconnectInterval(time.Minute) // Prevent exponential backoff from growing too large

    // Set up connection handlers
    opts.OnConnect = cm.handleConnect
    opts.OnConnectionLost = cm.handleDisconnect
    opts.OnReconnecting = cm.handleReconnecting

    // Configure TLS if enabled
    if broker.config.MQTT.TLS.Enable {
        tlsConfig, err := cm.newTLSConfig(
            broker.config.MQTT.TLS.CertFile,
            broker.config.MQTT.TLS.KeyFile,
            broker.config.MQTT.TLS.CAFile,
        )
        if err != nil {
            return nil, fmt.Errorf("failed to create TLS config: %w", err)
        }
        opts.SetTLSConfig(tlsConfig)
    }

    cm.client = mqtt.NewClient(opts)
    
    // Establish initial connection
    if token := cm.client.Connect(); token.Wait() && token.Error() != nil {
        return nil, fmt.Errorf("failed to connect to broker: %w", token.Error())
    }

    return cm, nil
}

// NewConnectionManagerWithClient creates a connection manager with a provided client (for testing)
func NewConnectionManagerWithClient(broker *MQTTBroker, client mqtt.Client) ConnectionManager {
    cm := &ConnectionManagerImpl{
        broker: broker,
        client: client,
    }
    cm.connected.Store(true)
    return cm
}

// Connect establishes connection to the MQTT broker
func (cm *ConnectionManagerImpl) Connect() error {
    if token := cm.client.Connect(); token.Wait() && token.Error() != nil {
        return fmt.Errorf("failed to connect to broker: %w", token.Error())
    }
    return nil
}

// Disconnect cleanly disconnects from the MQTT broker
func (cm *ConnectionManagerImpl) Disconnect() {
    cm.broker.logger.Info("disconnecting from mqtt broker")
    cm.client.Disconnect(250)
}

// IsConnected returns current connection status
func (cm *ConnectionManagerImpl) IsConnected() bool {
    return cm.connected.Load()
}

// HandleReconnect manages the reconnection process
func (cm *ConnectionManagerImpl) HandleReconnect() error {
    if !cm.IsConnected() {
        return cm.Connect()
    }
    return nil
}

// GetClient returns the MQTT client instance
func (cm *ConnectionManagerImpl) GetClient() mqtt.Client {
    return cm.client
}

// handleConnect processes successful connections and resubscribes to topics
func (cm *ConnectionManagerImpl) handleConnect(client mqtt.Client) {
    cm.broker.logger.Info("mqtt client connected", "broker", cm.broker.config.MQTT.Broker)
    cm.connected.Store(true)
    cm.broker.stats.LastReconnect = time.Now()

    cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.SetMQTTConnectionStatus(true)
    })

    // Restore rules first
    if err := cm.broker.RestoreRules(); err != nil {
        cm.broker.logger.Error("failed to restore rules after reconnect",
            "error", err)
        cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
            m.IncMQTTReconnects() // Track restoration failures
        })
        return
    }
    
    // Resubscribe to topics if needed
    if cm.broker.sub != nil {
        if err := cm.broker.sub.ResubscribeAll(); err != nil {
            cm.broker.logger.Error("failed to resubscribe to topics after reconnect",
                "error", err)
            cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
                m.IncMQTTReconnects() // Track resubscription failures
            })
            return
        }
        cm.broker.logger.Info("successfully restored rules and resubscribed to topics",
            "topics", cm.broker.sub.GetSubscribedTopics())
    }
}

// handleDisconnect processes connection loss
func (cm *ConnectionManagerImpl) handleDisconnect(client mqtt.Client, err error) {
    cm.broker.logger.Error("mqtt connection lost", "error", err)
    cm.connected.Store(false)

    cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.SetMQTTConnectionStatus(false)
    })
}

// handleReconnecting processes reconnection attempts
func (cm *ConnectionManagerImpl) handleReconnecting(client mqtt.Client, opts *mqtt.ClientOptions) {
    cm.broker.logger.Info("mqtt client reconnecting", 
        "broker", opts.Servers[0],
        "attempt", time.Since(cm.broker.stats.LastReconnect))
    
    cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
        m.IncMQTTReconnects()
    })
}

// newTLSConfig creates a new TLS configuration
func (cm *ConnectionManagerImpl) newTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
    cert, err := tls.LoadX509KeyPair(certFile, keyFile)
    if err != nil {
        return nil, fmt.Errorf("failed to load client certificate: %w", err)
    }

    caCert, err := os.ReadFile(caFile)
    if err != nil {
        return nil, fmt.Errorf("failed to read CA certificate: %w", err)
    }

    caCertPool := x509.NewCertPool()
    if !caCertPool.AppendCertsFromPEM(caCert) {
        return nil, fmt.Errorf("failed to parse CA certificate")
    }

    return &tls.Config{
        Certificates: []tls.Certificate{cert},
        RootCAs:     caCertPool,
        MinVersion:  tls.VersionTLS12,
    }, nil
}
