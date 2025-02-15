package mqtt

import (
    "sync"
    "sync/atomic"
    "time"

    mqtt "github.com/eclipse/paho.mqtt.golang"
    "mqtt-mux-router/internal/logger"
    "mqtt-mux-router/internal/metrics"
    "mqtt-mux-router/config"
)

// MockToken implements mqtt.Token for testing
type MockToken struct {
    err error
    done chan struct{}
}

func NewMockToken() *MockToken {
    return &MockToken{
        done: make(chan struct{}),
    }
}

func (t *MockToken) Wait() bool                 { return true }
func (t *MockToken) WaitTimeout(d time.Duration) bool { return true }
func (t *MockToken) Error() error              { return t.err }
func (t *MockToken) Done() <-chan struct{}     { return t.done }

// MockClient implements mqtt.Client for testing
type MockClient struct {
    connected     atomic.Bool
    publishFunc   func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token
    subscribeFunc func(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token
    mu           sync.RWMutex
}

func NewMockClient() *MockClient {
    return &MockClient{
        publishFunc: func(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
            return NewMockToken()
        },
        subscribeFunc: func(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
            return NewMockToken()
        },
    }
}

func (m *MockClient) Connect() mqtt.Token                 { return NewMockToken() }
func (m *MockClient) Disconnect(quiesce uint)            {}
func (m *MockClient) Publish(topic string, qos byte, retained bool, payload interface{}) mqtt.Token {
    return m.publishFunc(topic, qos, retained, payload)
}
func (m *MockClient) Subscribe(topic string, qos byte, callback mqtt.MessageHandler) mqtt.Token {
    return m.subscribeFunc(topic, qos, callback)
}
func (m *MockClient) SubscribeMultiple(filters map[string]byte, callback mqtt.MessageHandler) mqtt.Token {
    return NewMockToken()
}
func (m *MockClient) Unsubscribe(topics ...string) mqtt.Token { return NewMockToken() }
func (m *MockClient) AddRoute(topic string, callback mqtt.MessageHandler) {}
func (m *MockClient) IsConnected() bool { return m.connected.Load() }
func (m *MockClient) IsConnectionOpen() bool { return true }
func (m *MockClient) OptionsReader() mqtt.ClientOptionsReader { return mqtt.ClientOptionsReader{} }

// MockLogger implements logger.Logger for testing
type MockLogger struct {
    Logger *logger.Logger // Embed the actual logger type
    logs []struct {
        level string
        msg   string
        args  []interface{}
    }
    mu sync.RWMutex
}

func NewMockLogger() *logger.Logger {
    mock := &MockLogger{
        logs: make([]struct {
            level string
            msg   string
            args  []interface{}
        }, 0),
    }
    // Create a new base logger
    baseLogger, _ := logger.NewLogger(&config.LogConfig{
        Level:      "info",
        OutputPath: "stdout",
        Encoding:   "json",
    })
    mock.Logger = baseLogger
    return mock.Logger
}

// MockMetrics implements metrics.Metrics for testing
type MockMetrics struct {
    Metrics *metrics.Metrics // Embed the actual metrics type
    counters map[string]int64
    gauges   map[string]float64
    mu       sync.RWMutex
}

func NewMockMetrics() *metrics.Metrics {
    mock := &MockMetrics{
        counters: make(map[string]int64),
        gauges:   make(map[string]float64),
    }
    // Create a new base metrics
    baseMetrics, _ := metrics.NewMetrics(nil)
    mock.Metrics = baseMetrics
    return mock.Metrics
}
