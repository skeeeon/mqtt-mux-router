//file: internal/rule/evaluator.go

package rule

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// evaluateConditions evaluates a condition group against a message
func evaluateConditions(conditions *Conditions, msg *ProcessedMessage) bool {
	if conditions == nil || (len(conditions.Items) == 0 && len(conditions.Groups) == 0) {
		return true // No conditions means automatic match
	}

	results := make([]bool, 0, len(conditions.Items)+len(conditions.Groups))

	// Evaluate individual conditions
	for _, condition := range conditions.Items {
		result := evaluateCondition(&condition, msg.Values)
		results = append(results, result)
	}

	// Evaluate nested groups
	for _, group := range conditions.Groups {
		result := evaluateConditions(&group, msg)
		results = append(results, result)
	}

	// Apply logical operator
	switch conditions.Operator {
	case OperatorAnd:
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	case OperatorOr:
		for _, result := range results {
			if result {
				return true
			}
		}
		return false
	default:
		return false
	}
}

// evaluateCondition evaluates a single condition against message values
func evaluateCondition(condition *Condition, values map[string]interface{}) bool {
	value, exists := values[condition.Field]
	if !exists {
		return condition.Operator == OperatorNotEquals // Missing field only matches "not equals"
	}

	switch condition.Operator {
	case OperatorEquals:
		return compareValues(value, condition.Value) == 0
	case OperatorNotEquals:
		return compareValues(value, condition.Value) != 0
	case OperatorGreaterThan:
		return compareValues(value, condition.Value) > 0
	case OperatorLessThan:
		return compareValues(value, condition.Value) < 0
	case OperatorGreaterThanOrEqual:
		return compareValues(value, condition.Value) >= 0
	case OperatorLessThanOrEqual:
		return compareValues(value, condition.Value) <= 0
	case OperatorExists:
		return exists
	case OperatorContains:
		return containsValue(value, condition.Value)
	case OperatorMatches:
		return matchesPattern(value, condition.Value)
	default:
		return false
	}
}

// compareValues compares two values of potentially different types
func compareValues(a, b interface{}) int {
	// Handle nil cases
	if a == nil && b == nil {
		return 0
	}
	if a == nil {
		return -1
	}
	if b == nil {
		return 1
	}

	// Convert to comparable types
	switch va := a.(type) {
	case float64:
		if vb, ok := toFloat64(b); ok {
			if va < vb {
				return -1
			}
			if va > vb {
				return 1
			}
			return 0
		}
	case string:
		if vb, ok := toString(b); ok {
			return strings.Compare(va, vb)
		}
	case bool:
		if vb, ok := toBool(b); ok {
			if va == vb {
				return 0
			}
			if va {
				return 1
			}
			return -1
		}
	}

	// If types are incompatible, compare their string representations
	return strings.Compare(fmt.Sprintf("%v", a), fmt.Sprintf("%v", b))
}

// containsValue checks if one value contains another
func containsValue(value, substr interface{}) bool {
	str, ok := toString(value)
	if !ok {
		return false
	}

	subStr, ok := toString(substr)
	if !ok {
		return false
	}

	return strings.Contains(str, subStr)
}

// matchesPattern checks if a value matches a regular expression pattern
func matchesPattern(value, pattern interface{}) bool {
	str, ok := toString(value)
	if !ok {
		return false
	}

	patternStr, ok := toString(pattern)
	if !ok {
		return false
	}

	re, err := regexp.Compile(patternStr)
	if err != nil {
		return false
	}

	return re.MatchString(str)
}

// Type conversion helper functions
func toFloat64(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

func toString(v interface{}) (string, bool) {
	switch val := v.(type) {
	case string:
		return val, true
	case []byte:
		return string(val), true
	case fmt.Stringer:
		return val.String(), true
	default:
		return fmt.Sprintf("%v", val), true
	}
}

func toBool(v interface{}) (bool, bool) {
	switch val := v.(type) {
	case bool:
		return val, true
	case string:
		if b, err := strconv.ParseBool(val); err == nil {
			return b, true
		}
	case int:
		return val != 0, true
	case float64:
		return val != 0, true
	}
	return false, false
}
