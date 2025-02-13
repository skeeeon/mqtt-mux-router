//file: internal/rule/processor.go
package rule

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"

	"mqtt-mux-router/internal/logger"
	"mqtt-mux-router/internal/metrics"
)

// Processor handles rule processing and message routing
type Processor struct {
	index      *RuleIndex
	logger     *logger.Logger
	metrics    *metrics.Metrics
	jobChan    chan *ProcessedMessage
	workers    int
	wg         sync.WaitGroup
	stats      ProcessorStats
}

// ProcessorStats tracks processing statistics
type ProcessorStats struct {
	Processed uint64
	Matched   uint64
	Errors    uint64
}

// NewProcessor creates a new rule processor
func NewProcessor(workers int, logger *logger.Logger, metrics *metrics.Metrics) *Processor {
	if workers <= 0 {
		workers = 1
	}

	return &Processor{
		index:   NewRuleIndex(logger, metrics),
		logger:  logger,
		metrics: metrics,
		workers: workers,
		jobChan: make(chan *ProcessedMessage, 1000),
	}
}

// Start begins processing messages
func (p *Processor) Start() {
	for i := 0; i < p.workers; i++ {
		p.wg.Add(1)
		go p.worker()
	}
}

// Stop gracefully shuts down the processor
func (p *Processor) Stop() {
	close(p.jobChan)
	p.wg.Wait()
}

// Process processes a message against loaded rules
func (p *Processor) Process(msg *ProcessedMessage) ([]*MatchResult, error) {
	if msg == nil {
		return nil, fmt.Errorf("message is nil")
	}

	if msg.Topic == "" {
		return nil, fmt.Errorf("message topic is empty")
	}

	atomic.AddUint64(&p.stats.Processed, 1)

	// Validate message values
	if msg.Values == nil {
		msg.Values = make(map[string]interface{})
	}

	// If payload is present but Values is empty, try to unmarshal
	if len(msg.Payload) > 0 && len(msg.Values) == 0 {
		if err := json.Unmarshal(msg.Payload, &msg.Values); err != nil {
			atomic.AddUint64(&p.stats.Errors, 1)
			p.logger.Error("failed to unmarshal payload",
				"error", err,
				"topic", msg.Topic)
			return nil, fmt.Errorf("failed to unmarshal payload: %w", err)
		}
	}

	// Find matching rules
	rules := p.index.Find(msg.Topic)
	if len(rules) == 0 {
		return nil, nil
	}

	var lastErr error
	results := make([]*MatchResult, 0, len(rules))

	// Process each matching rule
	for _, rule := range rules {
		// Skip disabled rules
		if !rule.Enabled {
			continue
		}

		// Check source broker if specified
		if rule.SourceBroker != "" && rule.SourceBroker != msg.SourceBroker {
			continue
		}

		// Evaluate conditions
		if rule.Conditions != nil && !evaluateConditions(rule.Conditions, msg) {
			continue
		}

		// Process action template
		action, err := p.processActionTemplate(rule.Action, msg)
		if err != nil {
			atomic.AddUint64(&p.stats.Errors, 1)
			p.logger.Error("failed to process action template",
				"error", err,
				"topic", msg.Topic,
				"rule", rule.Topic)
			lastErr = err
			continue
		}

		results = append(results, &MatchResult{
			Rule:      rule,
			Action:    action,
			Variables: msg.Values,
		})

		atomic.AddUint64(&p.stats.Matched, 1)

		if p.metrics != nil {
			p.metrics.IncRuleMatches()
		}
	}

	// If we have processed rules but encountered errors, return the error
	if len(results) == 0 && lastErr != nil {
		return nil, lastErr
	}

	return results, nil
}

// processActionTemplate processes template variables in the action
func (p *Processor) processActionTemplate(action *Action, msg *ProcessedMessage) (*Action, error) {
	result := &Action{
		TargetBroker: action.TargetBroker,
		QoS:          action.QoS,
		Retain:       action.Retain,
	}

	p.logger.Debug("processing action template",
		"sourceTopic", msg.Topic,
		"targetTopic", action.Topic,
		"variables", getMapKeys(msg.Values))

	// Process topic template - topic variables are required
	topic, err := p.processTemplate(action.Topic, msg.Values, true)
	if err != nil {
		return nil, fmt.Errorf("failed to process topic template: %w", err)
	}
	result.Topic = topic

	// Process payload template - variables are optional
	if action.Payload != "" {
		payload, err := p.processTemplate(action.Payload, msg.Values, false)
		if err != nil {
			return nil, fmt.Errorf("failed to process payload template: %w", err)
		}
		result.Payload = payload
	}

	p.logger.Debug("action template processing complete",
		"topic", result.Topic)

	return result, nil
}

// worker processes messages from the job channel
func (p *Processor) worker() {
	defer p.wg.Done()

	for msg := range p.jobChan {
		if _, err := p.Process(msg); err != nil {
			p.logger.Error("failed to process message",
				"error", err,
				"topic", msg.Topic)
		}
	}
}

// GetStats returns current processor statistics
func (p *Processor) GetStats() ProcessorStats {
	return ProcessorStats{
		Processed: atomic.LoadUint64(&p.stats.Processed),
		Matched:   atomic.LoadUint64(&p.stats.Matched),
		Errors:    atomic.LoadUint64(&p.stats.Errors),
	}
}

// processTemplate processes a template string with message values
func (p *Processor) processTemplate(template string, values map[string]interface{}, isTopicTemplate bool) (string, error) {
	if template == "" {
		return "", nil
	}

	p.logger.Debug("processing template",
		"template", template,
		"isTopicTemplate", isTopicTemplate,
		"availableVariables", getMapKeys(values))

	// Find all ${...} references
	re := regexp.MustCompile(`\${([^}]+)}`)
	matches := re.FindAllStringSubmatch(template, -1)

	result := template
	for _, match := range matches {
		if len(match) != 2 {
			continue
		}

		placeholder := match[0]    // ${varname}
		varName := match[1]        // varname

		// Check if variable exists
		value, exists := values[varName]
		if !exists {
			// Topic templates require all variables
			if isTopicTemplate {
				p.logger.Error("required template variable not found",
					"variable", varName,
					"template", template)
				return "", fmt.Errorf("required template variable '%s' not found", varName)
			}
			// Use empty string for missing optional variables
			p.logger.Debug("optional template variable not found, using empty string",
				"variable", varName)
			value = ""
		}

		// Convert value to string
		strValue := p.convertValueToString(value)

		// Replace variable in template
		result = strings.Replace(result, placeholder, strValue, -1)

		p.logger.Debug("processed template variable",
			"variable", varName,
			"value", strValue)
	}

	return result, nil
}

// convertValueToString converts a value to its string representation
func (p *Processor) convertValueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	case nil:
		return ""
	case bool:
		if v {
			return "true"
		}
		return "false"
	case float64:
		// Use exact representation for integers
		if float64(int64(v)) == v {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	default:
		// For other types, use JSON marshaling
		if bytes, err := json.Marshal(v); err == nil {
			str := string(bytes)
			// Remove quotes from simple string values
			if len(str) > 0 && str[0] == '"' && str[len(str)-1] == '"' {
				str = str[1 : len(str)-1]
			}
			return str
		}
		// Fallback to simple string conversion
		return fmt.Sprintf("%v", v)
	}
}

// getMapKeys returns a sorted list of map keys
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
