// File: internal/rule/processor.go
package rule

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"mqtt-mux-router/internal/logger"
)

type Processor struct {
	rules  []Rule
	logger *logger.Logger
}

func NewProcessor(rulesPath string, log *logger.Logger) (*Processor, error) {
	rules, err := LoadRules(rulesPath)
	if err != nil {
		return nil, err
	}

	return &Processor{
		rules:  rules,
		logger: log,
	}, nil
}

func (p *Processor) GetTopics() []string {
	topics := make([]string, len(p.rules))
	for i, rule := range p.rules {
		topics[i] = rule.Topic
	}
	return topics
}

func (p *Processor) Process(topic string, payload []byte) ([]*Action, error) {
	var actions []*Action
	var msg map[string]interface{}

	if err := json.Unmarshal(payload, &msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	for _, rule := range p.rules {
		if rule.Topic != topic {
			continue
		}

		if rule.Conditions == nil || p.evaluateConditions(rule.Conditions, msg) {
			// Process action template with message values
			processedAction, err := p.processActionTemplate(rule.Action, msg)
			if err != nil {
				p.logger.Error("failed to process action template", "error", err)
				continue
			}
			actions = append(actions, processedAction)
		}
	}

	return actions, nil
}

// processActionTemplate creates a copy of the action with processed template values
func (p *Processor) processActionTemplate(action *Action, msg map[string]interface{}) (*Action, error) {
	// Create a new action to avoid modifying the original
	processedAction := &Action{
		Topic:   action.Topic,
		Payload: action.Payload,
	}

	// Process the payload template
	payload, err := p.processTemplate(action.Payload, msg)
	if err != nil {
		return nil, fmt.Errorf("failed to process payload template: %w", err)
	}
	processedAction.Payload = payload

	return processedAction, nil
}

// processTemplate handles the template substitution for both simple and nested values
func (p *Processor) processTemplate(template string, data map[string]interface{}) (string, error) {
	// Find all template variables in the format ${path.to.value}
	re := regexp.MustCompile(`\${([^}]+)}`)
	result := template

	matches := re.FindAllStringSubmatch(template, -1)
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}

		placeholder := match[0]    // The full placeholder (e.g., ${path.to.value})
		path := strings.Split(match[1], ".") // Split the path into parts

		// Get the value from the data using the path
		value, err := p.getValueFromPath(data, path)
		if err != nil {
			p.logger.Debug("template value not found", "path", match[1])
			continue
		}

		// Convert value to string based on type
		strValue := p.convertToString(value)
		result = strings.ReplaceAll(result, placeholder, strValue)
	}

	return result, nil
}

// getValueFromPath retrieves a value from nested maps using a path
func (p *Processor) getValueFromPath(data map[string]interface{}, path []string) (interface{}, error) {
	var current interface{} = data

	for _, key := range path {
		switch v := current.(type) {
		case map[string]interface{}:
			var ok bool
			current, ok = v[key]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", key)
			}
		case map[interface{}]interface{}:
			var ok bool
			current, ok = v[key]
			if !ok {
				return nil, fmt.Errorf("key not found: %s", key)
			}
		default:
			return nil, fmt.Errorf("invalid path: %s is not a map", key)
		}
	}

	return current, nil
}

// convertToString converts an interface value to its string representation
func (p *Processor) convertToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	case int:
		return strconv.Itoa(v)
	case bool:
		return strconv.FormatBool(v)
	case nil:
		return "null"
	default:
		// For complex types, convert to JSON
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", v)
	}
}

func (p *Processor) evaluateConditions(conditions *Conditions, msg map[string]interface{}) bool {
	if conditions == nil || (len(conditions.Items) == 0 && len(conditions.Groups) == 0) {
		return true
	}

	results := make([]bool, 0, len(conditions.Items)+len(conditions.Groups))

	// Evaluate individual conditions
	for _, condition := range conditions.Items {
		results = append(results, p.evaluateCondition(&condition, msg))
	}

	// Evaluate nested condition groups
	for _, group := range conditions.Groups {
		results = append(results, p.evaluateConditions(&group, msg))
	}

	// Apply logical operator
	switch conditions.Operator {
	case "and":
		for _, result := range results {
			if !result {
				return false
			}
		}
		return true
	case "or":
		for _, result := range results {
			if result {
				return true
			}
		}
		return false
	default:
		p.logger.Error("unknown logical operator", "operator", conditions.Operator)
		return false
	}
}

func (p *Processor) evaluateCondition(cond *Condition, msg map[string]interface{}) bool {
	value, ok := msg[cond.Field]
	if !ok {
		return false
	}

	switch cond.Operator {
	case "eq":
		return value == cond.Value
	case "neq":
		return value != cond.Value
	case "gt", "lt", "gte", "lte":
		return p.compareNumeric(value, cond.Value, cond.Operator)
	default:
		p.logger.Error("unknown operator", "operator", cond.Operator)
		return false
	}
}

func (p *Processor) compareNumeric(a, b interface{}, op string) bool {
	var numA, numB float64
	var err error

	switch v := a.(type) {
	case float64:
		numA = v
	case string:
		numA, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
	default:
		return false
	}

	switch v := b.(type) {
	case float64:
		numB = v
	case string:
		numB, err = strconv.ParseFloat(v, 64)
		if err != nil {
			return false
		}
	default:
		return false
	}

	switch op {
	case "gt":
		return numA > numB
	case "lt":
		return numA < numB
	case "gte":
		return numA >= numB
	case "lte":
		return numA <= numB
	default:
		return false
	}
}

