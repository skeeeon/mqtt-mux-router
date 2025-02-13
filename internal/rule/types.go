//file: internal/rule/types.go
package rule

import (
	"time"
	"fmt"
)

// Rule defines a message routing rule
type Rule struct {
	Topic        string      `json:"topic"`      // Source topic pattern
	SourceBroker string      `json:"sourceBroker,omitempty"` // Optional, defaults to first source broker
	Description  string      `json:"description,omitempty"`   // Optional rule description
	Enabled      bool        `json:"enabled"`     // Whether the rule is active
	Conditions   *Conditions `json:"conditions"`  // Optional conditions for rule matching
	Action       *Action     `json:"action"`      // Required action to take on match
	Priority     int         `json:"priority"`    // Optional priority for rule ordering
	CreatedAt    time.Time   `json:"createdAt"`  // When the rule was created
	UpdatedAt    time.Time   `json:"updatedAt"`  // When the rule was last updated
}

// Conditions represents a group of conditions with a logical operator
type Conditions struct {
	Operator string      `json:"operator"` // "and" or "or"
	Items    []Condition `json:"items"`    // Individual conditions
	Groups   []Conditions `json:"groups,omitempty"` // Nested condition groups
}

// Condition represents a single condition to evaluate
type Condition struct {
	Field    string      `json:"field"`    // Field name to evaluate
	Operator string      `json:"operator"` // Comparison operator
	Value    interface{} `json:"value"`    // Value to compare against
}

// Action represents an action to take when a rule matches
type Action struct {
	Topic        string            `json:"topic"`        // Target topic pattern
	TargetBroker string            `json:"targetBroker"` // Required, must be a target/both broker
	Payload      string            `json:"payload"`      // Template for the target payload
	QoS          byte             `json:"qos"`          // MQTT QoS level (0, 1, or 2)
	Retain       bool             `json:"retain"`       // Whether to retain the message
	Headers      map[string]string `json:"headers,omitempty"` // Optional headers to add
}

// RuleValidationError represents a rule validation error
type RuleValidationError struct {
	Field   string
	Message string
}

// Error implements the error interface
func (e *RuleValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ProcessedMessage represents a message being processed by rules
type ProcessedMessage struct {
	Topic        string
	SourceBroker string
	Payload      []byte
	Values       map[string]interface{}
	Timestamp    time.Time
	Headers      map[string]string
}

// MatchResult represents the result of a rule match
type MatchResult struct {
	Rule      *Rule
	Action    *Action
	Variables map[string]interface{}
	Headers   map[string]string
}

// RuleSet represents a collection of rules with metadata
type RuleSet struct {
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Version     string    `json:"version"`
	Rules       []Rule    `json:"rules"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Constants for condition operators
const (
	OperatorAnd = "and"
	OperatorOr  = "or"

	// Comparison operators
	OperatorEquals              = "eq"
	OperatorNotEquals          = "neq"
	OperatorGreaterThan        = "gt"
	OperatorLessThan           = "lt"
	OperatorGreaterThanOrEqual = "gte"
	OperatorLessThanOrEqual    = "lte"
	OperatorExists             = "exists"
	OperatorContains           = "contains"
	OperatorMatches            = "matches" // Regex matching
)

// ValidOperators contains all valid comparison operators
var ValidOperators = map[string]bool{
	OperatorEquals:              true,
	OperatorNotEquals:          true,
	OperatorGreaterThan:        true,
	OperatorLessThan:           true,
	OperatorGreaterThanOrEqual: true,
	OperatorLessThanOrEqual:    true,
	OperatorExists:             true,
	OperatorContains:           true,
	OperatorMatches:            true,
}
