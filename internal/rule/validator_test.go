//file: internal/rule/validator_test.go
package rule

import (
	"testing"
	"time"
)

func TestValidateRule(t *testing.T) {
	now := time.Now()
	tests := []struct {
		name     string
		rule     *Rule
		wantErr  bool
		errField string
	}{
		{
			name: "Valid complete rule",
			rule: &Rule{
				Topic:        "sensors/+/temperature",
				SourceBroker: "source1",
				Description:  "Temperature monitoring",
				Enabled:      true,
				CreatedAt:    now,
				Action: &Action{
					Topic:        "alerts/${device_id}",
					TargetBroker: "target1",
					Payload:      "${temperature}",
					QoS:          1,
					Retain:       false,
				},
			},
			wantErr: false,
		},
		{
			name:     "Nil rule",
			rule:     nil,
			wantErr:  true,
			errField: "rule",
		},
		{
			name: "Empty topic",
			rule: &Rule{
				Topic:   "",
				Enabled: true,
				Action: &Action{
					Topic:        "alerts",
					TargetBroker: "target1",
				},
			},
			wantErr:  true,
			errField: "topic",
		},
		{
			name: "Invalid wildcard placement",
			rule: &Rule{
				Topic:   "sensors/#/temperature",
				Enabled: true,
				Action: &Action{
					Topic:        "alerts",
					TargetBroker: "target1",
				},
			},
			wantErr:  true,
			errField: "topic",
		},
		{
			name: "Invalid + wildcard usage",
			rule: &Rule{
				Topic:   "sensors/dev+ice/temp",
				Enabled: true,
				Action: &Action{
					Topic:        "alerts",
					TargetBroker: "target1",
				},
			},
			wantErr:  true,
			errField: "topic",
		},
		{
			name: "Missing action",
			rule: &Rule{
				Topic:        "sensors/temperature",
				SourceBroker: "source1",
				Enabled:      true,
			},
			wantErr:  true,
			errField: "action",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRule(tt.rule)
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

func TestValidateAction(t *testing.T) {
	tests := []struct {
		name     string
		action   *Action
		wantErr  bool
		errField string
	}{
		{
			name: "Valid action",
			action: &Action{
				Topic:        "alerts/${device_id}",
				TargetBroker: "target1",
				Payload:      "${temperature}",
				QoS:          1,
				Retain:       false,
			},
			wantErr: false,
		},
		{
			name:     "Nil action",
			action:   nil,
			wantErr:  true,
			errField: "action",
		},
		{
			name: "Empty topic",
			action: &Action{
				Topic:        "",
				TargetBroker: "target1",
				QoS:          0,
			},
			wantErr:  true,
			errField: "action.topic",
		},
		{
			name: "Missing target broker",
			action: &Action{
				Topic: "alerts",
				QoS:   0,
			},
			wantErr:  true,
			errField: "action.targetBroker",
		},
		{
			name: "Invalid QoS",
			action: &Action{
				Topic:        "alerts",
				TargetBroker: "target1",
				QoS:          3,
			},
			wantErr:  true,
			errField: "action.qos",
		},
		{
			name: "Invalid template variable",
			action: &Action{
				Topic:        "alerts/${invalid-var}",
				TargetBroker: "target1",
				QoS:          0,
			},
			wantErr:  true,
			errField: "action.topic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateAction(tt.action)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateAction() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				if validErr, ok := err.(*RuleValidationError); ok {
					if validErr.Field != tt.errField {
						t.Errorf("validateAction() error field = %v, want %v", validErr.Field, tt.errField)
					}
				} else {
					t.Errorf("validateAction() error is not RuleValidationError")
				}
			}
		})
	}
}

func TestValidateConditions(t *testing.T) {
	tests := []struct {
		name       string
		conditions *Conditions
		wantErr    bool
		errField   string
	}{
		{
			name: "Valid AND conditions",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: OperatorGreaterThan,
						Value:    25.0,
					},
					{
						Field:    "humidity",
						Operator: OperatorLessThan,
						Value:    80.0,
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid nested conditions",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "type",
						Operator: OperatorEquals,
						Value:    "sensor",
					},
				},
				Groups: []Conditions{
					{
						Operator: OperatorOr,
						Items: []Condition{
							{
								Field:    "temperature",
								Operator: OperatorGreaterThan,
								Value:    30.0,
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid operator",
			conditions: &Conditions{
				Operator: "invalid",
				Items: []Condition{
					{
						Field:    "temperature",
						Operator: OperatorEquals,
						Value:    25.0,
					},
				},
			},
			wantErr:  true,
			errField: "conditions.operator",
		},
		{
			name: "Empty field name",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "",
						Operator: OperatorEquals,
						Value:    25.0,
					},
				},
			},
			wantErr:  true,
			errField: "conditions.items[0]",
		},
		{
			name: "Invalid regex pattern",
			conditions: &Conditions{
				Operator: OperatorAnd,
				Items: []Condition{
					{
						Field:    "status",
						Operator: OperatorMatches,
						Value:    "[invalid(regex",
					},
				},
			},
			wantErr:  true,
			errField: "conditions.items[0]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConditions(tt.conditions)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConditions() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil {
				if validErr, ok := err.(*RuleValidationError); ok {
					if validErr.Field != tt.errField {
						t.Errorf("validateConditions() error field = %v, want %v", validErr.Field, tt.errField)
					}
				} else {
					t.Errorf("validateConditions() error is not RuleValidationError")
				}
			}
		})
	}
}

func TestValidateTemplate(t *testing.T) {
	tests := []struct {
		name     string
		template string
		wantErr  bool
	}{
		{
			name:     "Valid empty template",
			template: "",
			wantErr:  false,
		},
		{
			name:     "Valid simple variable",
			template: "Hello ${name}",
			wantErr:  false,
		},
		{
			name:     "Valid multiple variables",
			template: "${greeting} ${name}! Temperature: ${temp}",
			wantErr:  false,
		},
		{
			name:     "Valid nested path",
			template: "${sensor.temperature.value}",
			wantErr:  false,
		},
		{
			name:     "Invalid variable name",
			template: "${invalid-name}",
			wantErr:  true,
		},
		{
			name:     "Invalid character in path",
			template: "${sensor.#.value}",
			wantErr:  true,
		},
		{
			name:     "Unclosed variable",
			template: "${name",
			wantErr:  false, // The regex won't match unclosed variables
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTemplate(tt.template)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateTemplate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestIsValidVariableName(t *testing.T) {
	tests := []struct {
		name string
		variable string
		want bool
	}{
		{
			name: "Simple variable",
			variable: "name",
			want: true,
		},
		{
			name: "Variable with underscore",
			variable: "device_id",
			want: true,
		},
		{
			name: "Variable with number",
			variable: "sensor1",
			want: true,
		},
		{
			name: "Nested path",
			variable: "sensor.temperature.value",
			want: true,
		},
		{
			name: "Invalid character",
			variable: "device-id",
			want: false,
		},
		{
			name: "Invalid start character",
			variable: "1sensor",
			want: false,
		},
		{
			name: "Empty string",
			variable: "",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidVariableName(tt.variable); got != tt.want {
				t.Errorf("isValidVariableName() = %v, want %v", got, tt.want)
			}
		})
	}
}
