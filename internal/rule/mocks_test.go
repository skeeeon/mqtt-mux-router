package rule

import (
	"github.com/prometheus/client_golang/prometheus"
	"mqtt-mux-router/internal/metrics"
)

// newMockMetrics creates a new metrics instance for testing
func newMockMetrics() *metrics.Metrics {
	// Create a test registry that we can throw away
	reg := prometheus.NewRegistry()
	m, _ := metrics.NewMetrics(reg)
	return m
}
