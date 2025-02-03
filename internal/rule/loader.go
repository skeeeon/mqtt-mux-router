package rule

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "mqtt-mux-router/internal/logger"
)

type RulesLoader struct {
    logger *logger.Logger
}

func NewRulesLoader(log *logger.Logger) *RulesLoader {
    return &RulesLoader{
        logger: log,
    }
}

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

        var ruleSet []Rule
        if err := json.Unmarshal(data, &ruleSet); err != nil {
            l.logger.Error("failed to parse rule file",
                "path", path,
                "error", err)
            return fmt.Errorf("failed to parse rule file %s: %w", path, err)
        }

        l.logger.Debug("successfully loaded rules",
            "path", path,
            "count", len(ruleSet))

        // Validate rules before adding them
        for i, rule := range ruleSet {
            if err := validateRule(&rule); err != nil {
                l.logger.Error("invalid rule configuration",
                    "path", path,
                    "ruleIndex", i,
                    "error", err)
                return fmt.Errorf("invalid rule in file %s at index %d: %w", path, i, err)
            }
        }

        rules = append(rules, ruleSet...)
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to load rules: %w", err)
    }

    l.logger.Info("rules loaded successfully",
        "totalRules", len(rules))

    return rules, nil
}

// validateRule performs basic validation of rule configuration
func validateRule(rule *Rule) error {
    if rule.Topic == "" {
        return fmt.Errorf("rule topic cannot be empty")
    }

    if rule.Action == nil {
        return fmt.Errorf("rule action cannot be nil")
    }

    if rule.Action.Topic == "" {
        return fmt.Errorf("action topic cannot be empty")
    }

    if rule.Conditions != nil {
        if err := validateConditions(rule.Conditions); err != nil {
            return fmt.Errorf("invalid conditions: %w", err)
        }
    }

    return nil
}

// validateConditions recursively validates condition groups
func validateConditions(conditions *Conditions) error {
    if conditions.Operator != "and" && conditions.Operator != "or" {
        return fmt.Errorf("invalid operator: %s", conditions.Operator)
    }

    // Validate individual conditions
    for _, condition := range conditions.Items {
        if condition.Field == "" {
            return fmt.Errorf("condition field cannot be empty")
        }
        if !isValidOperator(condition.Operator) {
            return fmt.Errorf("invalid condition operator: %s", condition.Operator)
        }
    }

    // Recursively validate nested condition groups
    for _, group := range conditions.Groups {
        if err := validateConditions(&group); err != nil {
            return err
        }
    }

    return nil
}

// isValidOperator checks if the operator is supported
func isValidOperator(op string) bool {
    validOperators := map[string]bool{
        "eq":       true,
        "neq":      true,
        "gt":       true,
        "lt":       true,
        "gte":      true,
        "lte":      true,
        "exists":   true,
        "contains": true,
    }
    return validOperators[op]
}
