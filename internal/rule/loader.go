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

		var ruleSet RuleSet
		if err := json.Unmarshal(data, &ruleSet); err != nil {
			// Try single rule format
			var singleRule Rule
			if err := json.Unmarshal(data, &singleRule); err != nil {
				l.logger.Error("failed to parse rule file",
					"path", path,
					"error", err)
				return fmt.Errorf("failed to parse rule file %s: %w", path, err)
			}
			rules = append(rules, singleRule)
		} else {
			rules = append(rules, ruleSet.Rules...)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to load rules: %w", err)
	}

	// Set timestamps for rules that don't have them
	now := time.Now()
	for i := range rules {
		if rules[i].CreatedAt.IsZero() {
			rules[i].CreatedAt = now
		}
		if rules[i].UpdatedAt.IsZero() {
			rules[i].UpdatedAt = now
		}
	}

	// Validate all rules
	for i, rule := range rules {
		if err := validateRule(&rule); err != nil {
			return nil, fmt.Errorf("invalid rule at index %d: %w", i, err)
		}
	}

	l.logger.Info("rules loaded successfully",
		"count", len(rules))

	return rules, nil
}
