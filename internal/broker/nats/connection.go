package nats

import (
	"fmt"
	"sync/atomic"
	"time"

	"github.com/nats-io/nats.go"
	"mqtt-mux-router/internal/metrics"
)

// ConnectionManagerImpl implements ConnectionManager for NATS
type ConnectionManagerImpl struct {
	broker    *NATSBroker
	conn      *nats.Conn
	connected atomic.Bool
}

// NewConnectionManager creates a new NATS connection manager
func NewConnectionManager(broker *NATSBroker) (ConnectionManager, error) {
	cm := &ConnectionManagerImpl{
		broker: broker,
	}

	// Establish initial connection
	if err := cm.Connect(); err != nil {
		return nil, err
	}

	return cm, nil
}

// Connect establishes connection to the NATS server
func (cm *ConnectionManagerImpl) Connect() error {
	if len(cm.broker.config.NATS.URLs) == 0 {
		return fmt.Errorf("no NATS server URLs provided")
	}

	// Create connection options
	opts := []nats.Option{
		nats.Name(cm.broker.config.NATS.ClientID),
		nats.ReconnectWait(time.Second * 2),
		nats.MaxReconnects(-1), // Unlimited reconnects
		nats.DisconnectErrHandler(cm.handleDisconnect),
		nats.ReconnectHandler(cm.handleReconnect),
		nats.ClosedHandler(cm.handleClosed),
	}

	// Add authentication if configured
	if cm.broker.config.NATS.Username != "" {
		opts = append(opts, nats.UserInfo(
			cm.broker.config.NATS.Username, 
			cm.broker.config.NATS.Password))
	}

	// Configure TLS if enabled
	if cm.broker.config.NATS.TLS.Enable {
		// ClientCert function only returns a single Option, not an error
		certOpt := nats.ClientCert(
			cm.broker.config.NATS.TLS.CertFile,
			cm.broker.config.NATS.TLS.KeyFile,
		)
		opts = append(opts, certOpt)

		// Add CA certificates if provided
		if cm.broker.config.NATS.TLS.CAFile != "" {
			rootCAOpt := nats.RootCAs(cm.broker.config.NATS.TLS.CAFile)
			opts = append(opts, rootCAOpt)
		}
	}

	// Connect to the NATS server
	cm.broker.logger.Info("connecting to NATS server", "urls", cm.broker.config.NATS.URLs)

	var err error
	cm.conn, err = nats.Connect(cm.broker.config.NATS.URLs[0], opts...)
	if err != nil {
		return fmt.Errorf("failed to connect to NATS server: %w", err)
	}

	// Update connection status
	cm.connected.Store(true)

	// Update metrics
	cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
		m.SetMQTTConnectionStatus(true)
	})

	cm.broker.logger.Info("connected to NATS server", "url", cm.conn.ConnectedUrl())

	return nil
}

// Disconnect cleanly disconnects from the NATS server
func (cm *ConnectionManagerImpl) Disconnect() {
	if cm.conn != nil {
		cm.broker.logger.Info("disconnecting from NATS server")
		cm.conn.Close()
		cm.connected.Store(false)
	}
}

// IsConnected returns the current connection status
func (cm *ConnectionManagerImpl) IsConnected() bool {
	return cm.conn != nil && cm.conn.IsConnected() && cm.connected.Load()
}

// GetConnection returns the NATS connection
func (cm *ConnectionManagerImpl) GetConnection() *nats.Conn {
	return cm.conn
}

// NATS connection event handlers

func (cm *ConnectionManagerImpl) handleDisconnect(conn *nats.Conn, err error) {
	cm.broker.logger.Error("disconnected from NATS server", "error", err)
	cm.connected.Store(false)

	cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
		m.SetMQTTConnectionStatus(false)
	})
}

func (cm *ConnectionManagerImpl) handleReconnect(conn *nats.Conn) {
	cm.broker.logger.Info("reconnected to NATS server", "url", conn.ConnectedUrl())
	cm.connected.Store(true)
	cm.broker.stats.LastReconnect = time.Now()

	cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
		m.SetMQTTConnectionStatus(true)
		m.IncMQTTReconnects()
	})

	// Restore rules and subscriptions
	cm.broker.RestoreState()
}

func (cm *ConnectionManagerImpl) handleClosed(conn *nats.Conn) {
	cm.broker.logger.Warn("NATS connection closed")
	cm.connected.Store(false)

	cm.broker.safeMetricsUpdate(func(m *metrics.Metrics) {
		m.SetMQTTConnectionStatus(false)
	})
}
