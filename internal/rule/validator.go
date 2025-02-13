//file: internal/rule/validator.go
package rule

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	// validVariablePattern matches valid variable names:
	// - Must start with a letter or underscore
	// - Can contain letters, numbers, underscores
	// - Can have dot notation for nested fields
	validVariablePattern = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*)*$`)
)

// validateRule performs comprehensive validation of a rule
func validateRule(rule *Rule) error {
	if rule == nil {
		return &RuleValidationError{
			Field:   "rule",
			Message: "rule cannot be nil",
		}
	}

	// Validate topic
	if err := validateTopic(rule.Topic); err != nil {
		return &RuleValidationError{
			Field:   "topic",
			Message: err.Error(),
		}
	}

	// Validate action
	if err := validateAction(rule.Action); err != nil {
		return err
	}

	// Validate conditions if present
	if rule.Conditions != nil {
		if err := validateConditions(rule.Conditions); err != nil {
			return err
		}
	}

	return nil
}

// validateTopic checks if a topic pattern is valid
func validateTopic(topic string) error {
	if topic == "" {
		return fmt.Errorf("topic cannot be empty")
	}

	segments := strings.Split(topic, "/")
	for i, segment := range segments {
		// Allow empty segments for leading/trailing slashes
		if segment == "" && i != 0 && i != len(segments)-1 {
			return fmt.Errorf("empty segment not allowed in middle of topic")
		}

		if strings.Contains(segment, "#") {
			if segment != "#" {
				return fmt.Errorf("# wildcard must occupy entire segment")
			}
			if i != len(segments)-1 {
				return fmt.Errorf("# wildcard must be the last segment")
			}
		}

		if strings.Contains(segment, "+") {
			if segment != "+" {
				return fmt.Errorf("+ wildcard must occupy entire segment")
			}
		}
	}

	return nil
}

// validateAction checks if an action configuration is valid
func validateAction(action *Action) error {
	if action == nil {
		return &RuleValidationError{
			Field:   "action",
			Message: "action cannot be nil",
		}
	}

	// Validate target topic
	if action.Topic == "" {
		return &RuleValidationError{
			Field:   "action.topic",
			Message: "action topic cannot be empty",
		}
	}

	// Validate target broker
	if action.TargetBroker == "" {
		return &RuleValidationError{
			Field:   "action.targetBroker",
			Message: "target broker cannot be empty",
		}
	}

	// Validate QoS
	if action.QoS > 2 {
		return &RuleValidationError{
			Field:   "action.qos",
			Message: "QoS must be 0, 1, or 2",
		}
	}

	// Validate topic template
	if err := validateTemplate(action.Topic); err != nil {
		return &RuleValidationError{
			Field:   "action.topic",
			Message: err.Error(),
		}
	}

	// Validate payload template
	if err := validateTemplate(action.Payload); err != nil {
		return &RuleValidationError{
			Field:   "action.payload",
			Message: err.Error(),
		}
	}

	return nil
}

// validateConditions validates a condition group recursively
func validateConditions(conditions *Conditions) error {
	if conditions == nil {
		return nil
	}

	// Validate operator
	switch conditions.Operator {
	case OperatorAnd, OperatorOr:
		// Valid operators
	default:
		return &RuleValidationError{
			Field:   "conditions.operator",
			Message: fmt.Sprintf("invalid operator: %s", conditions.Operator),
		}
	}

	// Validate individual conditions
	for i, condition := range conditions.Items {
		if err := validateCondition(&condition); err != nil {
			return &RuleValidationError{
				Field:   fmt.Sprintf("conditions.items[%d]", i),
				Message: err.Error(),
			}
		}
	}

	// Validate nested groups
	for i, group := range conditions.Groups {
		if err := validateConditions(&group); err != nil {
			return &RuleValidationError{
				Field:   fmt.Sprintf("conditions.groups[%d]", i),
				Message: err.Error(),
			}
		}
	}

	return nil
}

// validateCondition validates a single condition
func validateCondition(condition *Condition) error {
	if condition.Field == "" {
		return fmt.Errorf("field cannot be empty")
	}

	if !ValidOperators[condition.Operator] {
		return fmt.Errorf("invalid operator: %s", condition.Operator)
	}

	// Validate pattern for regex operator
	if condition.Operator == OperatorMatches {
		if pattern, ok := condition.Value.(string); ok {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("invalid regex pattern: %s", err)
			}
		} else {
			return fmt.Errorf("regex pattern must be a string")
		}
	}

	return nil
}

// validateTemplate checks if a template string has valid variable references
func validateTemplate(template string) error {
	if template == "" {
		return nil
	}

	// Find all ${...} references
	re := regexp.MustCompile(`\${([^}]+)}`)
	matches := re.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		varName := match[1]

		// Validate variable name format
		if !isValidVariableName(varName) {
			return fmt.Errorf("invalid variable name: %s", varName)
		}
	}

	return nil
}

// isValidVariableName checks if a variable name is valid
func isValidVariableName(name string) bool {
	return validVariablePattern.MatchString(name)
}
