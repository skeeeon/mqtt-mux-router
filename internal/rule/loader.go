//file: internal/rule/loader.go
package rule

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"mqtt-mux-router/internal/logger"
)

// RulesLoader handles loading rules from files
type RulesLoader struct {
	logger *logger.Logger
}

// NewRulesLoader creates a new rules loader
func NewRulesLoader(log *logger.Logger) *RulesLoader {
	return &RulesLoader{
		logger: log,
	}
}

// LoadFromDirectory loads and validates all rule files from a directory
func (l *RulesLoader) LoadFromDirectory(path string) ([]Rule, error) {
	var rules []Rule

	// Verify directory exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, fmt.Errorf("directory does not exist: %s", path)
	}

	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || filepath.Ext(path) != ".json" {
			return nil
		}

		l.logger.Debug("loading rule file", "path", path)

		data, err := os.ReadFile(path)
		if err != nil {
			l.logger.Error("failed to read rule file",
				"path", path,
				"error", err)
			return err
		}

		// Try to parse as a rule set first
		var ruleSet RuleSet
		if err := json.Unmarshal(data, &ruleSet); err == nil && ruleSet.Name != "" {
			// Valid rule set
			if err := l.validateRuleSet(&ruleSet); err != nil {
				return fmt.Errorf("invalid rule set in %s: %w", path, err)
			}
			rules = append(rules, ruleSet.Rules...)
		} else {
			// Try single rule format
			var singleRule Rule
			if err := json.Unmarshal(data, &singleRule); err != nil {
				l.logger.Error("failed to parse rule file",
					"path", path,
					"error", err)
				return fmt.Errorf("failed to parse rule file %s: %w", path, err)
			}
			// Validate single rule
			if err := validateRule(&singleRule); err != nil {
				return fmt.Errorf("invalid rule in %s: %w", path, err)
			}
			rules = append(rules, singleRule)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}

	// Set creation timestamp for rules that don't have one
	now := time.Now()
	for i := range rules {
		if rules[i].CreatedAt.IsZero() {
			rules[i].CreatedAt = now
		}
	}

	// Check for duplicate topics across all rules
	topics := make(map[string]struct{})
	for _, rule := range rules {
		if _, exists := topics[rule.Topic]; exists {
			return nil, fmt.Errorf("duplicate topic pattern found: %s", rule.Topic)
		}
		topics[rule.Topic] = struct{}{}
	}

	l.logger.Info("rules loaded successfully",
		"count", len(rules))

	return rules, nil
}

// validateRuleSet validates a complete rule set
func (l *RulesLoader) validateRuleSet(ruleSet *RuleSet) error {
	if ruleSet.Name == "" {
		return fmt.Errorf("rule set name is required")
	}

	if ruleSet.Version == "" {
		return fmt.Errorf("rule set version is required")
	}

	if len(ruleSet.Rules) == 0 {
		return fmt.Errorf("rule set must contain at least one rule")
	}

	if ruleSet.CreatedAt.IsZero() {
		ruleSet.CreatedAt = time.Now()
	}

	// Check for duplicate topics within the rule set
	topics := make(map[string]struct{})
	for _, rule := range ruleSet.Rules {
		if _, exists := topics[rule.Topic]; exists {
			return fmt.Errorf("duplicate topic pattern found: %s", rule.Topic)
		}
		topics[rule.Topic] = struct{}{}

		// Validate each rule in the set
		if err := validateRule(&rule); err != nil {
			return fmt.Errorf("invalid rule with topic %s: %w", rule.Topic, err)
		}
	}

	return nil
}
