package rule

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "mqtt-mux-router/internal/logger"
)

// RulesLoader handles loading rules from the filesystem
type RulesLoader struct {
    logger *logger.Logger
}

// NewRulesLoader creates a new rules loader
func NewRulesLoader(log *logger.Logger) *RulesLoader {
    return &RulesLoader{
        logger: log,
    }
}

// LoadFromDirectory loads all rules from a directory and its subdirectories
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
            return err
        }

        l.logger.Debug("successfully loaded rules",
            "path", path,
            "count", len(ruleSet))

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
