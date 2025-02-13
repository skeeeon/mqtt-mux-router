//file: internal/rule/types_test.go

package rule

import (
	"encoding/json"
	"testing"
	"time"
)

func TestRuleValidation(t *testing.T) {
	tests := []struct {
		name    string
		rule    Rule
		wantErr bool
		errField string
	}{
		{
			name: "Valid rule with source broker",
			rule: Rule{
				Topic:        "sensors/+/temperature",
				SourceBroker: "source1",
				Enabled:      true,
				Action: &Action{
					Topic:        "alerts/${topic}/temp",
					TargetBroker: "target1",
					QoS:          1,
				},
			},
			wantErr: false,
		},
		{
			name: "Valid rule without source broker",
			rule: Rule{
				Topic:   "sensors/#",
				Enabled: true,
				Action: &Action{
					Topic:        "processed/${topic}",
					TargetBroker: "target1",
					QoS:          0,
				},
			},
			wantErr: false,
		},
		{
			name: "Missing target broker",
			rule: Rule{
				Topic:   "sensors/temp",
				Enabled: true,
				Action: &Action{
					Topic: "alerts/temp",
					QoS:   1,
				},
			},
			wantErr: true,
			errField: "action.targetBroker",
		},
		{
			name: "Invalid QoS level",
			rule: Rule{
				Topic:   "sensors/temp",
				Enabled: true,
				Action: &Action{
					Topic:        "alerts/temp",
					TargetBroker: "target1",
					QoS:          3,
				},
			},
			wantErr: true,
			errField: "action.qos",
		},
		{
			name: "Missing topic",
			rule: Rule{
				SourceBroker: "source1",
				Enabled:      true,
				Action: &Action{
					Topic:        "alerts/temp",
					TargetBroker: "target1",
				},
			},
			wantErr: true,
			errField: "topic",
		},
		{
			name: "Missing action",
			rule: Rule{
				Topic:        "sensors/temp",
				SourceBroker: "source1",
				Enabled:      true,
			},
			wantErr: true,
			errField: "action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRule(&tt.rule)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRule() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				if validErr, ok := err.(*RuleValidationError); ok {
					if validErr.Field != tt.errField {
						t.Errorf("validateRule() error field = %v, want %v", validErr.Field, tt.errField)
					}
				} else {
					t.Errorf("validateRule() error is not RuleValidationError")
				}
			}
		})
	}
}

func TestConditionEvaluation(t *testing.T) {
	tests := []struct {
		name       string
		conditions *Conditions
		message    map[string]interface{}
		want       bool
	}{
		{
			name: "Simple equals condition",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: OperatorEquals,
						Value:    25.0,
					},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
			},
			want: true,
		},
		{
			name: "AND conditions",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: OperatorGreaterThan,
						Value:    20.0,
					},
					{
						Field:    "humidity",
						Operator: OperatorLessThan,
						Value:    80.0,
					},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
				"humidity":    70.0,
			},
			want: true,
		},
		{
			name: "OR conditions",
			conditions: &Conditions{
				Operator: OperatorOr,
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: OperatorGreaterThan,
						Value:    30.0,
					},
					{
						Field:    "humidity",
						Operator: OperatorGreaterThan,
						Value:    60.0,
					},
				},
			},
			message: map[string]interface{}{
				"temperature": 25.0,
				"humidity":    70.0,
			},
			want: true,
		},
		{
			name: "Nested conditions",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "sensor_type",
						Operator: OperatorEquals,
						Value:    "temperature",
					},
				},
				Groups: []Conditions{
					{
						Operator: OperatorOr,
						Items: []Condition{
							{
								Field:    "value",
								Operator: OperatorGreaterThan,
								Value:    30.0,
							},
							{
								Field:    "alert",
								Operator: OperatorEquals,
								Value:    true,
							},
						},
					},
				},
			},
			message: map[string]interface{}{
				"sensor_type": "temperature",
				"value":       25.0,
				"alert":      true,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := &ProcessedMessage{
				Values: tt.message,
			}
			got := evaluateConditions(tt.conditions, msg)
			if got != tt.want {
				t.Errorf("evaluateConditions() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRuleSetSerialization(t *testing.T) {
	now := time.Now().UTC().Truncate(time.Second)
	ruleSet := RuleSet{
		Name:        "Test Rules",
		Description: "Test rule set",
		Version:     "1.0",
		Rules: []Rule{
			{
				Topic:        "sensors/+/temperature",
				SourceBroker: "source1",
				Description:  "Temperature alerts",
				Enabled:      true,
				Priority:     1,
				CreatedAt:    now,
				UpdatedAt:    now,
				Action: &Action{
					Topic:        "alerts/${topic}",
					TargetBroker: "target1",
					QoS:          1,
					Headers: map[string]string{
						"type": "temperature",
					},
				},
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
	}

	// Test marshaling
	data, err := json.Marshal(ruleSet)
	if err != nil {
		t.Fatalf("Failed to marshal RuleSet: %v", err)
	}

	// Test unmarshaling
	var decoded RuleSet
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal RuleSet: %v", err)
	}

	// Compare fields
	if decoded.Name != ruleSet.Name {
		t.Errorf("Name = %v, want %v", decoded.Name, ruleSet.Name)
	}
	if decoded.Version != ruleSet.Version {
		t.Errorf("Version = %v, want %v", decoded.Version, ruleSet.Version)
	}
	if len(decoded.Rules) != len(ruleSet.Rules) {
		t.Errorf("Rules length = %v, want %v", len(decoded.Rules), len(ruleSet.Rules))
	}

	// Compare first rule
	if len(decoded.Rules) > 0 && len(ruleSet.Rules) > 0 {
		got := decoded.Rules[0]
		want := ruleSet.Rules[0]

		if got.Topic != want.Topic {
			t.Errorf("Rule Topic = %v, want %v", got.Topic, want.Topic)
		}
		if got.SourceBroker != want.SourceBroker {
			t.Errorf("Rule SourceBroker = %v, want %v", got.SourceBroker, want.SourceBroker)
		}
		if got.Action.TargetBroker != want.Action.TargetBroker {
			t.Errorf("Rule TargetBroker = %v, want %v", got.Action.TargetBroker, want.Action.TargetBroker)
		}
		if got.Action.QoS != want.Action.QoS {
			t.Errorf("Rule QoS = %v, want %v", got.Action.QoS, want.Action.QoS)
		}
	}
}

func TestMatchResult(t *testing.T) {
	now := time.Now()
	rule := &Rule{
		Topic:        "sensors/+/temperature",
		SourceBroker: "source1",
		Action: &Action{
			Topic:        "alerts/${device_id}/temp",
			TargetBroker: "target1",
			QoS:          1,
		},
	}

	result := &MatchResult{
		Rule: rule,
		Action: &Action{
			Topic:        "alerts/device123/temp",
			TargetBroker: "target1",
			QoS:          1,
		},
		Variables: map[string]interface{}{
			"device_id": "device123",
			"temperature": 25.5,
		},
		Headers: map[string]string{
			"timestamp": now.Format(time.RFC3339),
		},
	}

	// Test result fields
	if result.Rule != rule {
		t.Errorf("Rule = %v, want %v", result.Rule, rule)
	}

	if result.Action.Topic != "alerts/device123/temp" {
		t.Errorf("Action Topic = %v, want alerts/device123/temp", result.Action.Topic)
	}

	if val, ok := result.Variables["device_id"]; !ok || val != "device123" {
		t.Errorf("Variables[device_id] = %v, want device123", val)
	}

	if val, ok := result.Headers["timestamp"]; !ok || val == "" {
		t.Errorf("Headers[timestamp] missing or empty")
	}
}

func TestRuleValidationError(t *testing.T) {
	err := &RuleValidationError{
		Field:   "topic",
		Message: "cannot be empty",
	}

	expected := "topic: cannot be empty"
	if err.Error() != expected {
		t.Errorf("Error() = %v, want %v", err.Error(), expected)
	}
}

func TestProcessedMessage(t *testing.T) {
	msg := &ProcessedMessage{
		Topic:        "sensors/device123/temperature",
		SourceBroker: "source1",
		Payload:      []byte(`{"temperature": 25.5, "humidity": 60}`),
		Values: map[string]interface{}{
			"temperature": 25.5,
			"humidity":    60.0,
		},
		Timestamp: time.Now(),
		Headers: map[string]string{
			"device_id": "device123",
		},
	}

	// Test message fields
	if msg.Topic == "" {
		t.Error("Topic is empty")
	}
	if msg.SourceBroker == "" {
		t.Error("SourceBroker is empty")
	}
	if len(msg.Payload) == 0 {
		t.Error("Payload is empty")
	}
	if len(msg.Values) == 0 {
		t.Error("Values is empty")
	}
	if msg.Timestamp.IsZero() {
		t.Error("Timestamp is zero")
	}
	if len(msg.Headers) == 0 {
		t.Error("Headers is empty")
	}

	// Test values
	if temp, ok := msg.Values["temperature"].(float64); !ok || temp != 25.5 {
		t.Errorf("Values[temperature] = %v, want 25.5", temp)
	}
	if hum, ok := msg.Values["humidity"].(float64); !ok || hum != 60.0 {
		t.Errorf("Values[humidity] = %v, want 60.0", hum)
	}
}
