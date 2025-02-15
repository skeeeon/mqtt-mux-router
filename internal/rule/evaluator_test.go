package rule

import (
	"testing"

	"go.uber.org/zap"
	"mqtt-mux-router/internal/logger"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestProcessor(t *testing.T) *Processor {
	// Create test logger using zap directly
	zapLogger, err := zap.NewDevelopment()
	require.NoError(t, err)
	log := &logger.Logger{Logger: zapLogger}

	// Create mock metrics
	mockMetrics := newMockMetrics()

	// Create processor with minimal config
	cfg := ProcessorConfig{
		Workers:    1,
		QueueSize:  10,
		BatchSize:  1,
	}
	return NewProcessor(cfg, log, mockMetrics)
}

func TestEvaluateConditions(t *testing.T) {
	processor := setupTestProcessor(t)

	tests := []struct {
		name       string
		conditions *Conditions
		message    map[string]interface{}
		want       bool
	}{
		{
			name: "simple equals condition",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{Field: "temperature", Operator: "eq", Value: 25.0},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
			},
			want: true,
		},
		{
			name: "multiple AND conditions",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{Field: "temperature", Operator: "gt", Value: 20.0},
					{Field: "humidity", Operator: "lt", Value: 80.0},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
				"humidity":    70.0,
			},
			want: true,
		},
		{
			name: "multiple OR conditions",
			conditions: &Conditions{
				Operator: "or",
				Items: []Condition{
					{Field: "temperature", Operator: "gt", Value: 30.0},
					{Field: "humidity", Operator: "gt", Value: 60.0},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
				"humidity":    70.0,
			},
			want: true,
		},
		{
			name: "nested conditions",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{Field: "temperature", Operator: "gt", Value: 20.0},
				},
				Groups: []Conditions{
					{
						Operator: "or",
						Items: []Condition{
							{Field: "humidity", Operator: "lt", Value: 50.0},
							{Field: "pressure", Operator: "gt", Value: 1000.0},
						},
					},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
				"humidity":    60.0,
				"pressure":    1010.0,
			},
			want: true,
		},
		{
			name:       "nil conditions",
			conditions: nil,
			message: map[string]interface{}{
				"temperature": 25.0,
			},
			want: true,
		},
		{
			name: "empty conditions",
			conditions: &Conditions{
				Operator: "and",
			},
			message: map[string]interface{}{
				"temperature": 25.0,
			},
			want: true,
		},
		{
			name: "field not found",
			conditions: &Conditions{
				Operator: "and",
				Items: []Condition{
					{Field: "nonexistent", Operator: "eq", Value: 25.0},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
			},
			want: false,
		},
		{
			name: "invalid operator",
			conditions: &Conditions{
				Operator: "invalid",
				Items: []Condition{
					{Field: "temperature", Operator: "eq", Value: 25.0},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processor.evaluateConditions(tt.conditions, tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestEvaluateCondition(t *testing.T) {
	processor := setupTestProcessor(t)

	tests := []struct {
		name     string
		cond     *Condition
		message  map[string]interface{}
		want     bool
	}{
		{
			name: "equals - numeric",
			cond: &Condition{Field: "value", Operator: "eq", Value: 25.0},
			message: map[string]interface{}{"value": 25.0},
			want: true,
		},
		{
			name: "equals - string",
			cond: &Condition{Field: "status", Operator: "eq", Value: "active"},
			message: map[string]interface{}{"status": "active"},
			want: true,
		},
		{
			name: "not equals",
			cond: &Condition{Field: "value", Operator: "neq", Value: 25.0},
			message: map[string]interface{}{"value": 30.0},
			want: true,
		},
		{
			name: "greater than",
			cond: &Condition{Field: "value", Operator: "gt", Value: 25.0},
			message: map[string]interface{}{"value": 30.0},
			want: true,
		},
		{
			name: "less than",
			cond: &Condition{Field: "value", Operator: "lt", Value: 25.0},
			message: map[string]interface{}{"value": 20.0},
			want: true,
		},
		{
			name: "greater than or equal",
			cond: &Condition{Field: "value", Operator: "gte", Value: 25.0},
			message: map[string]interface{}{"value": 25.0},
			want: true,
		},
		{
			name: "less than or equal",
			cond: &Condition{Field: "value", Operator: "lte", Value: 25.0},
			message: map[string]interface{}{"value": 25.0},
			want: true,
		},
		{
			name: "exists",
			cond: &Condition{Field: "value", Operator: "exists"},
			message: map[string]interface{}{"value": 25.0},
			want: true,
		},
		{
			name: "contains",
			cond: &Condition{Field: "text", Operator: "contains", Value: "test"},
			message: map[string]interface{}{"text": "this is a test message"},
			want: true,
		},
		{
			name: "invalid operator",
			cond: &Condition{Field: "value", Operator: "invalid", Value: 25.0},
			message: map[string]interface{}{"value": 25.0},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processor.evaluateCondition(tt.cond, tt.message)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestCompareNumeric(t *testing.T) {
	processor := setupTestProcessor(t)

	tests := []struct {
		name string
		a    interface{}
		b    interface{}
		op   string
		want bool
	}{
		{
			name: "float64 comparison",
			a:    25.5,
			b:    20.0,
			op:   "gt",
			want: true,
		},
		{
			name: "string number comparison",
			a:    "25.5",
			b:    "20.0",
			op:   "gt",
			want: true,
		},
		{
			name: "mixed type comparison",
			a:    25.5,
			b:    "20.0",
			op:   "gt",
			want: true,
		},
		{
			name: "invalid first value",
			a:    "not a number",
			b:    20.0,
			op:   "gt",
			want: false,
		},
		{
			name: "invalid second value",
			a:    25.5,
			b:    "not a number",
			op:   "gt",
			want: false,
		},
		{
			name: "invalid operator",
			a:    25.5,
			b:    20.0,
			op:   "invalid",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := processor.compareNumeric(tt.a, tt.b, tt.op)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetMapKeys(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want []string
	}{
		{
			name: "empty map",
			m:    map[string]interface{}{},
			want: []string{},
		},
		{
			name: "single key",
			m: map[string]interface{}{
				"key1": "value1",
			},
			want: []string{"key1"},
		},
		{
			name: "multiple keys",
			m: map[string]interface{}{
				"key1": "value1",
				"key2": 2,
				"key3": true,
			},
			want: []string{"key1", "key2", "key3"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getMapKeys(tt.m)
			assert.ElementsMatch(t, tt.want, got)
		})
	}
}
