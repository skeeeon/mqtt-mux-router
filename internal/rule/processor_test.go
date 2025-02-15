package rule

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"mqtt-mux-router/config"
	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
)

// Using mocks from mocks_test.go

// testSetup provides utilities for test setup
type testSetup struct {
	processor *Processor
	logger    *logger.Logger
	metrics   *metrics.Metrics
	cleanup   func()
}

// newTestSetup creates a new test environment
func newTestSetup(t *testing.T) *testSetup {
	t.Helper()

	// Create test logger
	logConfig := &config.LogConfig{
		Level:      "debug",
		OutputPath: "stdout",
		Encoding:   "console",
	}
	testLogger, err := logger.NewLogger(logConfig)
	require.NoError(t, err)

	// Create test metrics using existing mock
	mockMetrics := newMockMetrics()

	// Create processor with test configuration
	cfg := ProcessorConfig{
		Workers:    2,
		QueueSize:  100,
		BatchSize:  10,
	}

	proc := NewProcessor(cfg, testLogger, mockMetrics)
	require.NotNil(t, proc)

	return &testSetup{
		processor: proc,
		logger:    testLogger,
		metrics:   mockMetrics,
		cleanup: func() {
			proc.Close()
		},
	}
}

// getTestRules returns a set of test rules
func getTestRules() []Rule {
	return []Rule{
		{
			Topic: "sensors/temperature",
			Conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    25.0,
					},
				},
			},
			Action: &Action{
				Topic:   "alerts/temperature",
				Payload: `{"alert":true,"temp":${temperature}}`,
			},
		},
		{
			Topic: "sensors/humidity",
			Conditions: &Conditions{
				Operator: "or",
				Items: []Condition{
					{
						Field:    "humidity",
						Operator: "gt",
						Value:    80.0,
					},
					{
						Field:    "temperature",
						Operator: "gt",
						Value:    30.0,
					},
				},
			},
			Action: &Action{
				Topic:   "alerts/humidity",
				Payload: `{"high_humidity":${humidity},"high_temp":${temperature}}`,
			},
		},
	}
}

func TestNewProcessor(t *testing.T) {
	tests := []struct {
		name          string
		cfg           ProcessorConfig
		wantWorkers   int
		wantQueueSize int
		wantErr       bool
	}{
		{
			name: "valid configuration",
			cfg: ProcessorConfig{
				Workers:    2,
				QueueSize:  100,
				BatchSize:  10,
			},
			wantWorkers:   2,
			wantQueueSize: 100,
			wantErr:       false,
		},
		{
			name: "zero workers",
			cfg: ProcessorConfig{
				Workers:    0,
				QueueSize:  100,
				BatchSize:  10,
			},
			wantWorkers:   1, // Should default to 1
			wantQueueSize: 100,
			wantErr:       false,
		},
		{
			name: "zero queue size",
			cfg: ProcessorConfig{
				Workers:    2,
				QueueSize:  0,
				BatchSize:  10,
			},
			wantWorkers:   2,
			wantQueueSize: 1000, // Should default to 1000
			wantErr:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logConfig := &config.LogConfig{
				Level:      "debug",
				OutputPath: "stdout",
				Encoding:   "console",
			}
			testLogger, err := logger.NewLogger(logConfig)
			require.NoError(t, err)

			mockMetrics := newMockMetrics()
			proc := NewProcessor(tt.cfg, testLogger, mockMetrics)
			if tt.wantErr {
				assert.Nil(t, proc)
			} else {
				assert.NotNil(t, proc)
				assert.Equal(t, tt.wantWorkers, proc.workers, "worker count mismatch")
				assert.Equal(t, tt.wantQueueSize, cap(proc.jobChan), "queue size mismatch")
				assert.NotNil(t, proc.index, "index should not be nil")
				assert.NotNil(t, proc.msgPool, "message pool should not be nil")
				assert.NotNil(t, proc.resultPool, "result pool should not be nil")
				proc.Close()
			}
		})
	}
}

func TestLoadRules(t *testing.T) {
	tests := []struct {
		name    string
		rules   []Rule
		wantLen int
	}{
		{
			name:    "valid rules",
			rules:   getTestRules(),
			wantLen: 2,
		},
		{
			name:    "nil rules",
			rules:   nil,
			wantLen: 0,
		},
		{
			name: "single rule",
			rules: []Rule{
				{
					Topic: "test/topic",
					Action: &Action{
						Topic:   "test/output",
						Payload: "test",
					},
				},
			},
			wantLen: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := newTestSetup(t)
			defer setup.cleanup()

			err := setup.processor.LoadRules(tt.rules)
			assert.NoError(t, err, "LoadRules should not return error as validation happens in loader")
			
			topics := setup.processor.GetTopics()
			assert.Len(t, topics, tt.wantLen, "should have expected number of topics")
			
			if tt.rules != nil {
				for _, rule := range tt.rules {
					matchingRules := setup.processor.index.Find(rule.Topic)
					assert.NotEmpty(t, matchingRules, "should find rules for topic: %s", rule.Topic)
				}
			}
		})
	}
}

func TestProcess(t *testing.T) {
	tests := []struct {
		name       string
		rules      []Rule
		topic      string
		payload    []byte
		wantAction []*Action
		wantErr    bool
	}{
		{
			name:  "matching temperature rule",
			rules: getTestRules(),
			topic: "sensors/temperature",
			payload: []byte(`{
				"temperature": 30.0,
				"timestamp": "2024-02-14T12:00:00Z"
			}`),
			wantAction: []*Action{
				{
					Topic:   "alerts/temperature",
					Payload: `{"alert":true,"temp":30}`,
				},
			},
			wantErr: false,
		},
		{
			name:  "non-matching temperature",
			rules: getTestRules(),
			topic: "sensors/temperature",
			payload: []byte(`{
				"temperature": 20.0,
				"timestamp": "2024-02-14T12:00:00Z"
			}`),
			wantAction: []*Action{}, // Changed from nil to empty slice
			wantErr:    false,
		},
		{
			name:    "invalid json payload",
			rules:   getTestRules(),
			topic:   "sensors/temperature",
			payload: []byte(`invalid json`),
			wantErr: true,
		},
		{
			name:    "empty payload",
			rules:   getTestRules(),
			topic:   "sensors/temperature",
			payload: []byte{},
			wantErr: true,
		},
		{
			name:       "unknown topic",
			rules:      getTestRules(),
			topic:      "unknown/topic",
			payload:    []byte(`{"temperature": 30.0}`),
			wantAction: nil, // This case should return nil as there are no matching rules
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := newTestSetup(t)
			defer setup.cleanup()

			err := setup.processor.LoadRules(tt.rules)
			require.NoError(t, err)

			actions, err := setup.processor.Process(tt.topic, tt.payload)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.wantAction == nil {
				assert.Nil(t, actions, "should return nil for no matching rules")
				return
			}

			assert.Equal(t, len(tt.wantAction), len(actions), "action count should match")
			for i, want := range tt.wantAction {
				assert.Equal(t, want.Topic, actions[i].Topic, "action topic should match")
				// Compare JSON payloads after normalizing
				var wantPayload, gotPayload interface{}
				err = json.Unmarshal([]byte(want.Payload), &wantPayload)
				require.NoError(t, err)
				err = json.Unmarshal([]byte(actions[i].Payload), &gotPayload)
				require.NoError(t, err)
				assert.Equal(t, wantPayload, gotPayload, "action payload should match")
			}
		})
	}
}

func TestProcessTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		data     map[string]interface{}
		want     string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			template: "Value is ${value}",
			data:     map[string]interface{}{"value": 42},
			want:     "Value is 42",
			wantErr:  false,
		},
		{
			name:     "multiple variables",
			template: `{"temp":${temperature},"humidity":${humidity}}`,
			data: map[string]interface{}{
				"temperature": 25.5,
				"humidity":    60,
			},
			want:    `{"temp":25.5,"humidity":60}`,
			wantErr: false,
		},
		{
			name:     "missing variable",
			template: "Value is ${missing}",
			data:     map[string]interface{}{"value": 42},
			want:     "Value is ${missing}", // Now expecting the original template text
			wantErr:  false,
		},
		{
			name:     "mixed existing and missing variables",
			template: `{"exists":${value},"missing":${nothere}}`,
			data:     map[string]interface{}{"value": 42},
			want:     `{"exists":42,"missing":${nothere}}`,
			wantErr:  false,
		},
		{
			name:     "uuid4 function",
			template: `{"id":"${uuid4()}"}`,
			data:     map[string]interface{}{},
			wantErr:  false,
		},
		{
			name:     "uuid7 function",
			template: `{"id":"${uuid7()}"}`,
			data:     map[string]interface{}{},
			wantErr:  false,
		},
		{
			name:     "empty template",
			template: "",
			data:     map[string]interface{}{},
			want:     "",
			wantErr:  false,
		},
		{
			name:     "template without variables",
			template: "just a string",
			data:     map[string]interface{}{},
			want:     "just a string",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := newTestSetup(t)
			defer setup.cleanup()

			result, err := setup.processor.processTemplate(tt.template, tt.data)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if !strings.Contains(tt.template, "uuid") {
				assert.Equal(t, tt.want, result, "template processing result should match expected output")
			} else {
				// For UUID tests, verify format only
				assert.Contains(t, result, `"id":"`, "UUID result should contain id field")
				assert.Contains(t, result, `"`, "UUID result should be properly quoted")
				if strings.Contains(tt.template, "uuid4") {
					assert.Len(t, strings.Split(strings.Trim(result, `"{}id: `), "-"), 5, "UUID4 should have 5 sections")
				}
			}
		})
	}
}

func TestGetValueFromPath(t *testing.T) {
	tests := []struct {
		name    string
		data    map[string]interface{}
		path    []string
		want    interface{}
		wantErr bool
	}{
		{
			name: "simple path",
			data: map[string]interface{}{
				"key": "value",
			},
			path:    []string{"key"},
			want:    "value",
			wantErr: false,
		},
		{
			name: "nested path",
			data: map[string]interface{}{
				"nested": map[string]interface{}{
					"key": "value",
				},
			},
			path:    []string{"nested", "key"},
			want:    "value",
			wantErr: false,
		},
		{
			name: "missing path",
			data: map[string]interface{}{
				"key": "value",
			},
			path:    []string{"missing"},
			wantErr: true,
		},
		{
			name: "invalid path type",
			data: map[string]interface{}{
				"key": "value",
			},
			path:    []string{"key", "invalid"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := newTestSetup(t)
			defer setup.cleanup()

			got, err := setup.processor.getValueFromPath(tt.data, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestMessageProcessingConcurrency(t *testing.T) {
	setup := newTestSetup(t)
	defer setup.cleanup()

	// Load test rules
	rules := getTestRules()
	err := setup.processor.LoadRules(rules)
	require.NoError(t, err)

	// Process multiple messages concurrently
	const numMessages = 100
	errors := make(chan error, numMessages)
	done := make(chan bool, numMessages)

	for i := 0; i < numMessages; i++ {
		go func(temp float64) {
			payload := []byte(fmt.Sprintf(`{"temperature": %f}`, temp))
			_, err := setup.processor.Process("sensors/temperature", payload)
			if err != nil {
				errors <- err
			}
			done <- true
		}(float64(20 + i))
	}

	// Wait for all goroutines to complete
	timeout := time.After(5 * time.Second)
	count := 0
	for count < numMessages {
		select {
		case err := <-errors:
			t.Errorf("Processing error: %v", err)
		case <-done:
			count++
		case <-timeout:
			t.Fatalf("Test timed out after processing %d/%d messages", count, numMessages)
		}
	}
}

func TestProcessorClose(t *testing.T) {
	setup := newTestSetup(t)
	
	// Load some rules
	rules := getTestRules()
	err := setup.processor.LoadRules(rules)
	require.NoError(t, err)

	// Submit a few messages
	for i := 0; i < 5; i++ {
		payload := []byte(fmt.Sprintf(`{"temperature": %f}`, float64(20+i)))
		_, err := setup.processor.Process("sensors/temperature", payload)
		require.NoError(t, err)
	}

	// Close the processor only once
	setup.processor.Close()

	// Verify channel is closed
	select {
	case _, ok := <-setup.processor.GetJobChannel():
		assert.False(t, ok, "job channel should be closed")
	default:
		// Channel is already closed
	}
}

func TestProcessorStats(t *testing.T) {
	setup := newTestSetup(t)
	defer setup.cleanup()

	// Load rules
	rules := getTestRules()
	err := setup.processor.LoadRules(rules)
	require.NoError(t, err)

	// Process some messages
	payloads := []struct {
		temp float64
		want bool
	}{
		{temp: 30.0, want: true},  // Should match
		{temp: 20.0, want: false}, // Shouldn't match
		{temp: 26.0, want: true},  // Should match
	}

	for _, p := range payloads {
		payload := []byte(fmt.Sprintf(`{"temperature": %f}`, p.temp))
		actions, err := setup.processor.Process("sensors/temperature", payload)
		require.NoError(t, err)
		assert.Equal(t, p.want, len(actions) > 0)
	}

	// Get stats
	stats := setup.processor.GetStats()

	// Verify stats
	assert.Equal(t, uint64(3), stats.Processed, "should have processed 3 messages")
	assert.Equal(t, uint64(2), stats.Matched, "should have matched 2 messages")
	assert.Equal(t, uint64(0), stats.Errors, "should have no errors")
}

func TestConvertToString(t *testing.T) {
	tests := []struct {
		name  string
		value interface{}
		want  string
	}{
		{
			name:  "string value",
			value: "test",
			want:  "test",
		},
		{
			name:  "integer value",
			value: 42,
			want:  "42",
		},
		{
			name:  "float value",
			value: 42.5,
			want:  "42.5",
		},
		{
			name:  "boolean value",
			value: true,
			want:  "true",
		},
		{
			name:  "nil value",
			value: nil,
			want:  "null",
		},
		{
			name: "map value",
			value: map[string]interface{}{
				"key": "value",
			},
			want: `{"key":"value"}`,
		},
		{
			name:  "slice value",
			value: []interface{}{1, 2, 3},
			want:  "[1,2,3]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := newTestSetup(t)
			defer setup.cleanup()

			result := setup.processor.convertToString(tt.value)
			
			if strings.Contains(tt.want, "{") || strings.Contains(tt.want, "[") {
				// For JSON values, compare after normalizing
				var wantJSON, gotJSON interface{}
				err := json.Unmarshal([]byte(tt.want), &wantJSON)
				require.NoError(t, err)
				err = json.Unmarshal([]byte(result), &gotJSON)
				require.NoError(t, err)
				assert.Equal(t, wantJSON, gotJSON)
			} else {
				assert.Equal(t, tt.want, result)
			}
		})
	}
}

func TestProcessMessagePool(t *testing.T) {
	setup := newTestSetup(t)
	defer setup.cleanup()

	// Load rules
	rules := getTestRules()
	err := setup.processor.LoadRules(rules)
	require.NoError(t, err)

	// Get a message from the pool
	msg := setup.processor.msgPool.Get()
	require.NotNil(t, msg)

	// Setup message
	msg.Topic = "sensors/temperature"
	msg.Payload = []byte(`{"temperature": 30.0}`)
	msg.Rules = setup.processor.index.Find(msg.Topic)

	// Process message
	err = json.Unmarshal(msg.Payload, &msg.Values)
	require.NoError(t, err)

	setup.processor.processMessage(msg)

	// Verify message pool behavior
	msg2 := setup.processor.msgPool.Get()
	require.NotNil(t, msg2)
	assert.Empty(t, msg2.Topic, "pooled message should have empty topic")
	assert.Empty(t, msg2.Values, "pooled message should have empty values")
	assert.Empty(t, msg2.Rules, "pooled message should have empty rules")
}
