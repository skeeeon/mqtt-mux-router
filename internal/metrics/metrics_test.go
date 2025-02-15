package metrics

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/assert"
)

func TestNewMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	assert.NoError(t, err)
	assert.NotNil(t, m)
}

func TestMetricsSetConnectionStatus(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	assert.NoError(t, err)

	// Test setting connection status
	m.SetMQTTConnectionStatus(true)
	m.SetMQTTConnectionStatus(false)
	
	// Note: In a real integration test, we'd use prometheus's test utilities
	// to verify the actual metric values
}

func TestMetricsIncrementCounters(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := NewMetrics(reg)
	assert.NoError(t, err)

	// Test various counter increments
	m.IncMessagesTotal("received")
	m.IncMessagesTotal("processed")
	m.IncMessagesTotal("error")
	m.IncRuleMatches()
	m.IncMQTTReconnects()
	m.IncActionsTotal("success")
	m.IncActionsTotal("error")
}
