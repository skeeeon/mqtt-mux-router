package rule

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strconv"
    "strings"
    "sync"
    "sync/atomic"

    "mqtt-mux-router/internal/logger"
)

// ProcessorConfig holds processor configuration
type ProcessorConfig struct {
    Workers    int
    QueueSize  int
    BatchSize  int
}

// Processor provides rule processing
type Processor struct {
    index      *RuleIndex
    msgPool    *MessagePool
    resultPool *ResultPool
    workers    int
    jobChan    chan *ProcessingMessage
    logger     *logger.Logger
    stats      ProcessorStats
    wg         sync.WaitGroup
}

// ProcessorStats tracks processing metrics
type ProcessorStats struct {
    Processed uint64
    Matched   uint64
    Errors    uint64
}

// NewProcessor creates a new processor
func NewProcessor(cfg ProcessorConfig, log *logger.Logger) *Processor {
    if cfg.Workers <= 0 {
        cfg.Workers = 1
    }
    if cfg.QueueSize <= 0 {
        cfg.QueueSize = 1000
    }
    
    p := &Processor{
        index:      NewRuleIndex(),
        msgPool:    NewMessagePool(),
        resultPool: NewResultPool(),
        workers:    cfg.Workers,
        jobChan:    make(chan *ProcessingMessage, cfg.QueueSize),
        logger:     log,
    }

    // Start worker pool
    p.startWorkers()

    return p
}

// LoadRules loads rules into the index
func (p *Processor) LoadRules(rules []Rule) error {
    p.index.Clear()
    
    for i := range rules {
        rule := &rules[i]
        p.index.Add(rule)
    }

    p.logger.Info("rules loaded into index",
        "count", len(rules))

    return nil
}

// GetTopics returns a list of all topics from the loaded rules
func (p *Processor) GetTopics() []string {
    return p.index.GetTopics()
}

// Process handles a message
func (p *Processor) Process(topic string, payload []byte) ([]*Action, error) {
    // Get message object from pool
    msg := p.msgPool.Get()
    msg.Topic = topic
    msg.Payload = payload

    // Find matching rules
    msg.Rules = p.index.Find(topic)
    if len(msg.Rules) == 0 {
        p.msgPool.Put(msg)
        return nil, nil
    }

    // Parse payload
    if err := json.Unmarshal(payload, &msg.Values); err != nil {
        p.msgPool.Put(msg)
        atomic.AddUint64(&p.stats.Errors, 1)
        return nil, fmt.Errorf("failed to unmarshal message: %w", err)
    }

    // Process rules
    for _, rule := range msg.Rules {
        if rule.Conditions == nil || p.evaluateConditions(rule.Conditions, msg.Values) {
            action, err := p.processActionTemplate(rule.Action, msg.Values)
            if err != nil {
                p.logger.Error("failed to process action template",
                    "error", err,
                    "topic", rule.Topic)
                continue
            }
            msg.Actions = append(msg.Actions, action)
        }
    }

    // Copy actions before returning message to pool
    actions := make([]*Action, len(msg.Actions))
    copy(actions, msg.Actions)

    atomic.AddUint64(&p.stats.Processed, 1)
    if len(actions) > 0 {
        atomic.AddUint64(&p.stats.Matched, 1)
    }

    // Return message to pool
    p.msgPool.Put(msg)

    return actions, nil
}

// processActionTemplate processes the action template with values from the message
func (p *Processor) processActionTemplate(action *Action, msg map[string]interface{}) (*Action, error) {
    // Create a new action to avoid modifying the original
    processedAction := &Action{
        Topic:   action.Topic,
        Payload: action.Payload,
    }

    // Process the topic template if it contains variables
    if strings.Contains(action.Topic, "${") {
        topic, err := p.processTemplate(action.Topic, msg)
        if err != nil {
            return nil, fmt.Errorf("failed to process topic template: %w", err)
        }
        processedAction.Topic = topic
    }

    // Process the payload template
    payload, err := p.processTemplate(action.Payload, msg)
    if err != nil {
        return nil, fmt.Errorf("failed to process payload template: %w", err)
    }
    processedAction.Payload = payload

    return processedAction, nil
}

// Template processing functions
func (p *Processor) processTemplate(template string, data map[string]interface{}) (string, error) {
    p.logger.Debug("processing template",
        "template", template,
        "dataKeys", getMapKeys(data))

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

        p.logger.Debug("processing template variable",
            "placeholder", placeholder,
            "path", path)

        // Get the value from the data using the path
        value, err := p.getValueFromPath(data, path)
        if err != nil {
            p.logger.Debug("template value not found",
                "path", match[1],
                "error", err)
            continue
        }

        // Convert value to string and replace in template
        strValue := p.convertToString(value)
        result = strings.ReplaceAll(result, placeholder, strValue)
    }

    return result, nil
}

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
    case map[string]interface{}, []interface{}:
        // For complex types, convert to JSON
        jsonBytes, err := json.Marshal(v)
        if err != nil {
            p.logger.Debug("failed to marshal complex value to JSON",
                "error", err)
            return fmt.Sprintf("%v", v)
        }
        return string(jsonBytes)
    default:
        return fmt.Sprintf("%v", v)
    }
}

// Internal worker pool functions
func (p *Processor) startWorkers() {
    for i := 0; i < p.workers; i++ {
        p.wg.Add(1)
        go p.worker()
    }
}

func (p *Processor) worker() {
    defer p.wg.Done()

    for msg := range p.jobChan {
        p.processMessage(msg)
    }
}

func (p *Processor) processMessage(msg *ProcessingMessage) {
    defer p.msgPool.Put(msg)

    if err := json.Unmarshal(msg.Payload, &msg.Values); err != nil {
        atomic.AddUint64(&p.stats.Errors, 1)
        p.logger.Error("failed to unmarshal message",
            "error", err,
            "topic", msg.Topic)
        return
    }

    atomic.AddUint64(&p.stats.Processed, 1)
    if len(msg.Actions) > 0 {
        atomic.AddUint64(&p.stats.Matched, 1)
    }
}

// GetStats returns current processing statistics
func (p *Processor) GetStats() ProcessorStats {
    return ProcessorStats{
        Processed: atomic.LoadUint64(&p.stats.Processed),
        Matched:   atomic.LoadUint64(&p.stats.Matched),
        Errors:    atomic.LoadUint64(&p.stats.Errors),
    }
}

// Close shuts down the processor
func (p *Processor) Close() {
    close(p.jobChan)
    p.wg.Wait()
}
