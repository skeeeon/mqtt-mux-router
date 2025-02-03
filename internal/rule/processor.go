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

type ProcessorConfig struct {
    Workers    int
    QueueSize  int
    BatchSize  int
}

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

type ProcessorStats struct {
    Processed uint64
    Matched   uint64
    Errors    uint64
}

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

    p.startWorkers()
    return p
}

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

func (p *Processor) GetTopics() []string {
    return p.index.GetTopics()
}

func (p *Processor) Process(topic string, payload []byte) ([]*Action, error) {
    msg := p.msgPool.Get()
    msg.Topic = topic
    msg.Payload = payload

    msg.Rules = p.index.Find(topic)
    if len(msg.Rules) == 0 {
        p.msgPool.Put(msg)
        return nil, nil
    }

    if err := json.Unmarshal(payload, &msg.Values); err != nil {
        p.msgPool.Put(msg)
        atomic.AddUint64(&p.stats.Errors, 1)
        return nil, fmt.Errorf("failed to unmarshal message: %w", err)
    }

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

    actions := make([]*Action, len(msg.Actions))
    copy(actions, msg.Actions)

    atomic.AddUint64(&p.stats.Processed, 1)
    if len(actions) > 0 {
        atomic.AddUint64(&p.stats.Matched, 1)
    }

    p.msgPool.Put(msg)
    return actions, nil
}

func (p *Processor) processActionTemplate(action *Action, msg map[string]interface{}) (*Action, error) {
    processedAction := &Action{
        Topic:   action.Topic,
        Payload: action.Payload,
    }

    if strings.Contains(action.Topic, "${") {
        topic, err := p.processTemplate(action.Topic, msg)
        if err != nil {
            return nil, fmt.Errorf("failed to process topic template: %w", err)
        }
        processedAction.Topic = topic
    }

    payload, err := p.processTemplate(action.Payload, msg)
    if err != nil {
        return nil, fmt.Errorf("failed to process payload template: %w", err)
    }
    processedAction.Payload = payload

    return processedAction, nil
}

func (p *Processor) processTemplate(template string, data map[string]interface{}) (string, error) {
    p.logger.Debug("processing template",
        "template", template,
        "dataKeys", getMapKeys(data))

    re := regexp.MustCompile(`\${([^}]+)}`)
    result := template

    matches := re.FindAllStringSubmatch(template, -1)
    for _, match := range matches {
        if len(match) != 2 {
            continue
        }

        placeholder := match[0]
        path := strings.Split(match[1], ".")

        p.logger.Debug("processing template variable",
            "placeholder", placeholder,
            "path", path)

        value, err := p.getValueFromPath(data, path)
        if err != nil {
            p.logger.Debug("template value not found",
                "path", match[1],
                "error", err)
            continue
        }

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

func (p *Processor) GetStats() ProcessorStats {
    return ProcessorStats{
        Processed: atomic.LoadUint64(&p.stats.Processed),
        Matched:   atomic.LoadUint64(&p.stats.Matched),
        Errors:    atomic.LoadUint64(&p.stats.Errors),
    }
}

func (p *Processor) Close() {
    close(p.jobChan)
    p.wg.Wait()
}
