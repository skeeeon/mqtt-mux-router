package stats

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewStatsCollector verifies the initialization of a new StatsCollector
func TestNewStatsCollector(t *testing.T) {
	collector := NewStatsCollector()

	// Check initial values
	assert.NotNil(t, collector, "StatsCollector should be created")
	assert.WithinDuration(t, time.Now(), collector.StartTime, 100*time.Millisecond, "StartTime should be close to current time")
	assert.WithinDuration(t, time.Now(), collector.LastUpdate, 100*time.Millisecond, "LastUpdate should be close to current time")
	
	// Check initial stat values are zero
	assert.Zero(t, collector.MessagesReceived, "MessagesReceived should be zero")
	assert.Zero(t, collector.MessagesProcessed, "MessagesProcessed should be zero")
	assert.Zero(t, collector.RulesMatched, "RulesMatched should be zero")
	assert.Zero(t, collector.ActionsExecuted, "ActionsExecuted should be zero")
	assert.Zero(t, collector.Errors, "Errors should be zero")
}

// TestUpdate verifies the Update method of StatsCollector
func TestUpdate(t *testing.T) {
	collector := NewStatsCollector()

	// Test updating with specific values
	testValues := []struct {
		received  uint64
		processed uint64
		matched   uint64
		executed  uint64
		errors    uint64
	}{
		{10, 8, 5, 3, 2},
		{20, 18, 10, 7, 3},
		{0, 0, 0, 0, 0},
	}

	for _, testCase := range testValues {
		t.Run("Update Stats", func(t *testing.T) {
			beforeUpdate := collector.LastUpdate

			// Update stats
			collector.Update(
				testCase.received, 
				testCase.processed, 
				testCase.matched, 
				testCase.executed, 
				testCase.errors,
			)

			// Verify atomic updates
			assert.Equal(t, testCase.received, collector.MessagesReceived, "MessagesReceived should match")
			assert.Equal(t, testCase.processed, collector.MessagesProcessed, "MessagesProcessed should match")
			assert.Equal(t, testCase.matched, collector.RulesMatched, "RulesMatched should match")
			assert.Equal(t, testCase.executed, collector.ActionsExecuted, "ActionsExecuted should match")
			assert.Equal(t, testCase.errors, collector.Errors, "Errors should match")

			// Verify LastUpdate is updated
			assert.True(t, collector.LastUpdate.After(beforeUpdate), "LastUpdate should be more recent")
		})
	}
}

// TestGetStats verifies the GetStats method
func TestGetStats(t *testing.T) {
	collector := NewStatsCollector()

	// Set some test values
	collector.Update(100, 90, 50, 30, 5)

	// Get stats
	stats := collector.GetStats()

	// Verify stats map contents
	assert.Contains(t, stats, "uptime", "Should have uptime")
	assert.Contains(t, stats, "messages_received", "Should have messages_received")
	assert.Contains(t, stats, "messages_processed", "Should have messages_processed")
	assert.Contains(t, stats, "rules_matched", "Should have rules_matched")
	assert.Contains(t, stats, "actions_executed", "Should have actions_executed")
	assert.Contains(t, stats, "errors", "Should have errors")
	assert.Contains(t, stats, "last_update", "Should have last_update")

	// Verify specific values
	assert.Equal(t, uint64(100), stats["messages_received"], "messages_received should match")
	assert.Equal(t, uint64(90), stats["messages_processed"], "messages_processed should match")
	assert.Equal(t, uint64(50), stats["rules_matched"], "rules_matched should match")
	assert.Equal(t, uint64(30), stats["actions_executed"], "actions_executed should match")
	assert.Equal(t, uint64(5), stats["errors"], "errors should match")
}

// TestGetStatsJSON verifies JSON marshaling of stats
func TestGetStatsJSON(t *testing.T) {
	// Inline stats update and JSON conversion
	jsonStats, err := func() ([]byte, error) {
		c := NewStatsCollector()
		c.Update(100, 90, 50, 30, 5)
		return c.GetStatsJSON()
	}()

	require.NoError(t, err, "GetStatsJSON should not return an error")

	// Unmarshal JSON to verify contents
	var statsMap map[string]interface{}
	err = json.Unmarshal(jsonStats, &statsMap)
	require.NoError(t, err, "Should be able to unmarshal JSON")

	// Verify key stats
	assert.Equal(t, float64(100), statsMap["messages_received"], "messages_received should match")
	assert.Equal(t, float64(90), statsMap["messages_processed"], "messages_processed should match")
	assert.Equal(t, float64(50), statsMap["rules_matched"], "rules_matched should match")
	assert.Equal(t, float64(30), statsMap["actions_executed"], "actions_executed should match")
	assert.Equal(t, float64(5), statsMap["errors"], "errors should match")
}

// TestCalculateRate verifies message processing rate calculation
func TestCalculateRate(t *testing.T) {
	// Test scenarios
	testCases := []struct {
		name            string
		received        uint64
		processingTime  time.Duration
		expectedRange   struct {
			min float64
			max float64
		}
	}{
		{
			name:            "Zero processing",
			received:        0,
			processingTime:  1 * time.Second,
			expectedRange:   struct{ min, max float64 }{0, 0.001},
		},
		{
			name:            "Normal processing",
			received:        100,
			processingTime:  10 * time.Second,
			expectedRange:   struct{ min, max float64 }{9.9, 10.1},
		},
		{
			name:            "Low time processing",
			received:        50,
			processingTime:  100 * time.Millisecond,
			expectedRange:   struct{ min, max float64 }{490, 510},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create a new collector with a fixed start time
			fixedStartTime := time.Now().Add(-tc.processingTime)
			collector := &StatsCollector{
				StartTime:         fixedStartTime,
				MessagesProcessed: tc.received,
			}

			// Calculate rate
			rate := collector.CalculateRate()

			// Check if rate is within expected range
			assert.GreaterOrEqual(t, rate, tc.expectedRange.min, "Rate should be greater than or equal to minimum")
			assert.LessOrEqual(t, rate, tc.expectedRange.max, "Rate should be less than or equal to maximum")
		})
	}
}
