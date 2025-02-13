package rule

import (
	"encoding/json"
	"testing"
	"strings"
	"fmt"

	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
	"github.com/prometheus/client_golang/prometheus"
)

func setupTestProcessor(t *testing.T) (*Processor, *logger.Logger) {
	t.Helper()

	// Create logger
	log, err := logger.NewLogger(&config.LogConfig{
		Level:      "debug",
		OutputPath: "stdout",
		Encoding:   "console",
	})
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	// Create metrics
	reg := prometheus.NewRegistry()
	metrics, err := metrics.NewMetrics(reg)
	if err != nil {
		t.Fatalf("Failed to create metrics: %v", err)
	}

	// Create processor with test configuration
	processor := NewProcessor(2, log, metrics)
	if processor == nil {
		t.Fatal("Failed to create processor")
	}

	return processor, log
}

func TestProcessorInitialization(t *testing.T) {
	processor, _ := setupTestProcessor(t)

	tests := []struct {
		name    string
		workers int
		want    int
	}{
		{"Default workers", 0, 1},
		{"Multiple workers", 4, 4},
		{"Single worker", 1, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := NewProcessor(tt.workers, processor.logger, processor.metrics)
			if p.workers != tt.want {
				t.Errorf("NewProcessor() workers = %v, want %v", p.workers, tt.want)
			}
			if p.jobChan == nil {
				t.Error("NewProcessor() jobChan is nil")
			}
			if p.index == nil {
				t.Error("NewProcessor() index is nil")
			}
		})
	}
}

func TestProcessorMessageProcessing(t *testing.T) {
	processor, _ := setupTestProcessor(t)

	// Create test rules
	rules := []Rule{
		{
			Topic:        "sensors/temperature",
			SourceBroker: "source1",
			Enabled:      true,
			Action: &Action{
				Topic:        "alerts/temperature",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
		{
			Topic:        "sensors/+/temperature",
			SourceBroker: "source1",
			Enabled:      true,
			Conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    30.0,
					},
				},
			},
			Action: &Action{
				Topic:        "alerts/high-temp/${device_id}",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
	}

	// Add rules to processor
	for i := range rules {
		if err := processor.index.Add(&rules[i]); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	tests := []struct {
		name       string
		message    *ProcessedMessage
		wantMatch  bool
		wantCount  int
		wantError  bool
	}{
		{
			name: "Exact match",
			message: &ProcessedMessage{
				Topic:        "sensors/temperature",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"temperature": 25.0,
				},
			},
			wantMatch: true,
			wantCount: 1,
			wantError: false,
		},
		{
			name: "Wildcard match with condition",
			message: &ProcessedMessage{
				Topic:        "sensors/device1/temperature",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"temperature": 35.0,
					"device_id":   "device1",
				},
			},
			wantMatch: true,
			wantCount: 1,
			wantError: false,
		},
		{
			name: "No match",
			message: &ProcessedMessage{
				Topic:        "sensors/humidity",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"humidity": 60.0,
				},
			},
			wantMatch: false,
			wantCount: 0,
			wantError: false,
		},
		{
			name:       "Nil message",
			message:    nil,
			wantMatch: false,
			wantCount: 0,
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := processor.Process(tt.message)
			
			if (err != nil) != tt.wantError {
				t.Errorf("Process() error = %v, wantError %v", err, tt.wantError)
				return
			}

			if !tt.wantError {
				matchCount := len(results)
				if (matchCount > 0) != tt.wantMatch {
					t.Errorf("Process() match = %v, want %v", matchCount > 0, tt.wantMatch)
				}
				if matchCount != tt.wantCount {
					t.Errorf("Process() got %v matches, want %v", matchCount, tt.wantCount)
				}
			}
		})
	}
}

func TestProcessorTemplateProcessing(t *testing.T) {
	processor, _ := setupTestProcessor(t)

	rule := &Rule{
		Topic:        "devices/+/status",
		SourceBroker: "source1",
		Enabled:      true,
		Action: &Action{
			Topic:        "processed/${device_id}/status/${status}",
			TargetBroker: "target1",
			Payload:      `{"device":"${device_id}","status":"${status}","timestamp":"${timestamp}"}`,
			Headers: map[string]string{
				"device": "${device_id}",
				"type":   "status",
			},
		},
	}

	if err := processor.index.Add(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	tests := []struct {
		name       string
		message    *ProcessedMessage
		wantTopic  string
		wantValues map[string]interface{}
		wantError  bool
	}{
		{
			name: "Complete template substitution",
			message: &ProcessedMessage{
				Topic:        "devices/dev123/status",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"device_id": "dev123",
					"status":    "online",
					"timestamp": "2024-02-12T12:00:00Z",
				},
			},
			wantTopic: "processed/dev123/status/online",
			wantValues: map[string]interface{}{
				"device":    "dev123",
				"status":    "online",
				"timestamp": "2024-02-12T12:00:00Z",
			},
			wantError: false,
		},
		{
			name: "Missing optional value",
			message: &ProcessedMessage{
				Topic:        "devices/dev123/status",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"device_id": "dev123",
					"status":    "online",
				},
			},
			wantTopic: "processed/dev123/status/online",
			wantValues: map[string]interface{}{
				"device": "dev123",
				"status": "online",
				"timestamp": "",
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := processor.Process(tt.message)
			
			if tt.wantError && err == nil {
				t.Error("Process() expected error but got none")
				return
			}
			
			if !tt.wantError && err != nil {
				t.Fatalf("Process() unexpected error = %v", err)
			}

			if len(results) != 1 {
				t.Fatalf("Process() got %v results, want 1", len(results))
			}

			result := results[0]

			// Check topic template processing
			if result.Action.Topic != tt.wantTopic {
				t.Errorf("Process() topic = %q, want %q", result.Action.Topic, tt.wantTopic)
			}

			// Check payload template processing
			var payload map[string]interface{}
			if err := json.Unmarshal([]byte(result.Action.Payload), &payload); err != nil {
				t.Fatalf("Failed to unmarshal payload: %v, payload: %s", err, result.Action.Payload)
			}

			// Compare payload values
			for key, want := range tt.wantValues {
				got, exists := payload[key]
				if !exists {
					t.Errorf("Payload missing key %q", key)
					continue
				}
				
				// Convert values to strings for comparison
				wantStr := fmt.Sprintf("%v", want)
				gotStr := fmt.Sprintf("%v", got)
				if gotStr != wantStr {
					t.Errorf("Payload[%q] = %q, want %q", key, gotStr, wantStr)
				}
			}

			// Ensure no extra keys in payload
			for key := range payload {
				if _, exists := tt.wantValues[key]; !exists {
					t.Errorf("Payload contains unexpected key %q", key)
				}
			}

			// Check header template processing
			deviceID, exists := result.Headers["device"]
			if !exists {
				t.Error("Headers missing 'device' key")
			} else {
				expectedDeviceID := fmt.Sprintf("%v", tt.message.Values["device_id"])
				if deviceID != expectedDeviceID {
					t.Errorf("Headers[device] = %q, want %q (original device_id: %v)", 
						deviceID, 
						expectedDeviceID, 
						tt.message.Values["device_id"])
				}
			}
		})
	}
}

func TestProcessorConcurrentProcessing(t *testing.T) {
	processor, _ := setupTestProcessor(t)
	processor.Start()
	defer processor.Stop()

	// Add test rules
	rule := &Rule{
		Topic:        "test/+/data",
		SourceBroker: "source1",
		Enabled:      true,
		Action: &Action{
			Topic:        "processed/${device_id}",
			TargetBroker: "target1",
			QoS:          1,
		},
	}

	if err := processor.index.Add(rule); err != nil {
		t.Fatalf("Failed to add rule: %v", err)
	}

	// Number of concurrent messages to process
	const numMessages = 1000
	processed := make(chan bool, numMessages)

	// Send messages concurrently
	for i := 0; i < numMessages; i++ {
		go func(id int) {
			msg := &ProcessedMessage{
				Topic:        "test/device/data",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"device_id": id,
					"value":     float64(id),
				},
			}

			results, err := processor.Process(msg)
			if err != nil {
				t.Errorf("Process() error = %v", err)
				processed <- false
				return
			}

			if len(results) != 1 {
				t.Errorf("Process() got %v results, want 1", len(results))
				processed <- false
				return
			}

			processed <- true
		}(i)
	}

	// Wait for all messages to be processed
	successCount := 0
	for i := 0; i < numMessages; i++ {
		if <-processed {
			successCount++
		}
	}

	if successCount != numMessages {
		t.Errorf("Processed %v messages successfully, want %v", successCount, numMessages)
	}

	// Check processor stats
	stats := processor.GetStats()
	if stats.Processed != uint64(numMessages) {
		t.Errorf("Stats.Processed = %v, want %v", stats.Processed, numMessages)
	}
	if stats.Matched != uint64(numMessages) {
		t.Errorf("Stats.Matched = %v, want %v", stats.Matched, numMessages)
	}
}

func TestProcessorDisabledRules(t *testing.T) {
	processor, _ := setupTestProcessor(t)

	// Add enabled and disabled rules
	rules := []Rule{
		{
			Topic:        "test/enabled",
			SourceBroker: "source1",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/enabled",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
		{
			Topic:        "test/disabled",
			SourceBroker: "source1",
			Enabled:      false,
			Action: &Action{
				Topic:        "processed/disabled",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
	}

	for i := range rules {
		if err := processor.index.Add(&rules[i]); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	tests := []struct {
		name      string
		topic     string
		wantMatch bool
	}{
		{
			name:      "Enabled rule",
			topic:     "test/enabled",
			wantMatch: true,
		},
		{
			name:      "Disabled rule",
			topic:     "test/disabled",
			wantMatch: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &ProcessedMessage{
				Topic:        tt.topic,
				SourceBroker: "source1",
				Values:       map[string]interface{}{},
			}

			results, err := processor.Process(msg)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			if (len(results) > 0) != tt.wantMatch {
				t.Errorf("Process() match = %v, want %v", len(results) > 0, tt.wantMatch)
			}
		})
	}
}

func TestProcessorSourceBrokerFiltering(t *testing.T) {
	processor, _ := setupTestProcessor(t)

	// Add rules for different source brokers
	rules := []Rule{
		{
			Topic:        "test/broker1",
			SourceBroker: "broker1",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/broker1",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
		{
			Topic:        "test/broker2",
			SourceBroker: "broker2",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/broker2",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
		{
			Topic:        "test/any",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/any",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
	}

	for i := range rules {
		if err := processor.index.Add(&rules[i]); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	tests := []struct {
		name         string
		topic        string
		sourceBroker string
		wantMatches  int
	}{
		{
			name:         "Matching source broker",
			topic:        "test/broker1",
			sourceBroker: "broker1",
			wantMatches:  1,
		},
		{
			name:         "Non-matching source broker",
			topic:        "test/broker1",
			sourceBroker: "broker2",
			wantMatches:  0,
		},
		{
			name:         "Any source broker",
			topic:        "test/any",
			sourceBroker: "broker3",
			wantMatches:  1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &ProcessedMessage{
				Topic:        tt.topic,
				SourceBroker: tt.sourceBroker,
				Values:       map[string]interface{}{},
			}

			results, err := processor.Process(msg)
			if err != nil {
				t.Fatalf("Process() error = %v", err)
			}

			if len(results) != tt.wantMatches {
				t.Errorf("Process() got %d matches, want %d", len(results), tt.wantMatches)
			}
		})
	}
}

func TestProcessorErrorHandling(t *testing.T) {
	processor, _ := setupTestProcessor(t)

	// Add rules for testing error conditions
	rules := []Rule{
		{
			Topic:        "test/template_error",
			SourceBroker: "source1",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/${required_var}/status", // Requires variable
				TargetBroker: "target1",
				QoS:          1,
				Payload:      `{"value": "${required_var}"}`,
			},
		},
		{
			Topic:        "test/json_error",
			SourceBroker: "source1",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/value",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
	}

	for i := range rules {
		if err := processor.index.Add(&rules[i]); err != nil {
			t.Fatalf("Failed to add rule: %v", err)
		}
	}

	tests := []struct {
		name       string
		message    *ProcessedMessage
		expectErr  string
	}{
		{
			name: "Malformed JSON in Values",
			message: &ProcessedMessage{
				Topic:        "test/json_error",
				SourceBroker: "source1",
				Payload:      []byte(`{invalid json}`),
				Values:       nil,
			},
			expectErr: "failed to unmarshal payload",
		},
		{
			name: "Missing required template variable",
			message: &ProcessedMessage{
				Topic:        "test/template_error",
				SourceBroker: "source1",
				Values: map[string]interface{}{
					"other_var": "value", // Missing required_var
				},
			},
			expectErr: "required template variable 'required_var' not found",
		},
		{
			name:    "Nil message",
			message: nil,
			expectErr: "message is nil",
		},
		{
			name: "Empty topic",
			message: &ProcessedMessage{
				Topic:        "",
				SourceBroker: "source1",
				Values:       map[string]interface{}{},
			},
			expectErr: "message topic is empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := processor.Process(tt.message)
			
			if tt.expectErr == "" && err != nil {
				t.Errorf("Process() unexpected error = %v", err)
			}
			
			if tt.expectErr != "" {
				if err == nil {
					t.Errorf("Process() expected error containing %q, got nil", tt.expectErr)
				} else if !strings.Contains(err.Error(), tt.expectErr) {
					t.Errorf("Process() error = %v, want error containing %q", err, tt.expectErr)
				}
			}

			if err != nil && results != nil {
				t.Error("Process() returned non-nil results with error")
			}
		})
	}
}

// Benchmarks
func BenchmarkProcessor(b *testing.B) {
	log, err := logger.NewLogger(&config.LogConfig{
		Level:      "error", // Minimize logging overhead
		OutputPath: "stdout",
		Encoding:   "console",
	})
	if err != nil {
		b.Fatalf("Failed to create logger: %v", err)
	}

	reg := prometheus.NewRegistry()
	metrics, err := metrics.NewMetrics(reg)
	if err != nil {
		b.Fatalf("Failed to create metrics: %v", err)
	}

	processor := NewProcessor(4, log, metrics)

	// Add test rules
	rules := []Rule{
		{
			Topic:        "bench/exact",
			SourceBroker: "source1",
			Enabled:      true,
			Action: &Action{
				Topic:        "processed/exact",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
		{
			Topic:        "bench/+/wildcard",
			SourceBroker: "source1",
			Enabled:      true,
			Conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "value",
						Operator: "gt",
						Value:    50.0,
					},
				},
			},
			Action: &Action{
				Topic:        "processed/${device_id}",
				TargetBroker: "target1",
				QoS:          1,
			},
		},
	}

	for i := range rules {
		if err := processor.index.Add(&rules[i]); err != nil {
			b.Fatalf("Failed to add rule: %v", err)
		}
	}

	b.Run("ExactMatch", func(b *testing.B) {
		msg := &ProcessedMessage{
			Topic:        "bench/exact",
			SourceBroker: "source1",
			Values:       map[string]interface{}{},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := processor.Process(msg); err != nil {
				b.Fatalf("Process() error = %v", err)
			}
		}
	})

	b.Run("WildcardMatch", func(b *testing.B) {
		msg := &ProcessedMessage{
			Topic:        "bench/device1/wildcard",
			SourceBroker: "source1",
			Values: map[string]interface{}{
				"device_id": "device1",
				"value":     75.0,
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := processor.Process(msg); err != nil {
				b.Fatalf("Process() error = %v", err)
			}
		}
	})

	b.Run("NoMatch", func(b *testing.B) {
		msg := &ProcessedMessage{
			Topic:        "bench/nomatch",
			SourceBroker: "source1",
			Values:       map[string]interface{}{},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			if _, err := processor.Process(msg); err != nil {
				b.Fatalf("Process() error = %v", err)
			}
		}
	})

	b.Run("ConcurrentProcessing", func(b *testing.B) {
		processor.Start()
		defer processor.Stop()

		msg := &ProcessedMessage{
			Topic:        "bench/exact",
			SourceBroker: "source1",
			Values:       map[string]interface{}{},
		}

		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				if _, err := processor.Process(msg); err != nil {
					b.Fatalf("Process() error = %v", err)
				}
			}
		})
	})
}
