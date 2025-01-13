package rule

import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"

    "mqtt-mux-router/internal/logger"
)

type Processor struct {
    rules  []Rule
    logger *logger.Logger
}

// NewProcessor creates a new rule processor and loads rules from the specified path
func NewProcessor(rulesPath string, log *logger.Logger) (*Processor, error) {
    rules, err := LoadRules(rulesPath, log)
    if err != nil {
        return nil, err
    }

    processor := &Processor{
        rules:  rules,
        logger: log,
    }

    // Log loaded rules for verification
    processor.logger.Debug("processor initialized", 
        "rulesCount", len(rules),
        "path", rulesPath)

    return processor, nil
}

// LoadRules loads and validates all rules from the specified directory
func LoadRules(path string, log *logger.Logger) ([]Rule, error) {
    var rules []Rule

    err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        if info.IsDir() || filepath.Ext(path) != ".json" {
            return nil
        }

        log.Debug("loading rule file", "path", path)

        data, err := os.ReadFile(path)
        if err != nil {
            log.Error("failed to read rule file",
                "path", path,
                "error", err)
            return err
        }

        var ruleSet []Rule
        if err := json.Unmarshal(data, &ruleSet); err != nil {
            log.Error("failed to parse rule file",
                "path", path,
                "error", err)
            return err
        }

        log.Debug("successfully loaded rules",
            "path", path,
            "count", len(ruleSet))

        rules = append(rules, ruleSet...)
        return nil
    })

    if err != nil {
        return nil, fmt.Errorf("failed to load rules: %w", err)
    }

    log.Info("rules loaded successfully",
        "totalRules", len(rules))

    return rules, nil
}

// Process processes an incoming message against all rules
func (p *Processor) Process(topic string, payload []byte) ([]*Action, error) {
    var actions []*Action
    var msg map[string]interface{}

    if err := json.Unmarshal(payload, &msg); err != nil {
        return nil, fmt.Errorf("failed to unmarshal message: %w", err)
    }

    p.logger.Debug("processing message for rules evaluation", 
        "topic", topic,
        "payload", string(payload))

    for _, rule := range p.rules {
        if rule.Topic != topic {
            continue
        }

        p.logger.Debug("found matching rule", 
            "topic", rule.Topic,
            "hasConditions", rule.Conditions != nil)

        if rule.Conditions == nil || p.evaluateConditions(rule.Conditions, msg) {
            processedAction, err := p.processActionTemplate(rule.Action, msg)
            if err != nil {
                p.logger.Error("failed to process action template",
                    "error", err,
                    "topic", rule.Topic)
                continue
            }
            actions = append(actions, processedAction)
            p.logger.Debug("rule matched and action processed",
                "topic", rule.Topic,
                "actionTopic", processedAction.Topic)
        } else {
            p.logger.Debug("rule conditions not met",
                "topic", rule.Topic)
        }
    }

    return actions, nil
}

// GetTopics returns a list of all topics from the loaded rules
func (p *Processor) GetTopics() []string {
    topics := make([]string, len(p.rules))
    for i, rule := range p.rules {
        topics[i] = rule.Topic
    }
    return topics
}
