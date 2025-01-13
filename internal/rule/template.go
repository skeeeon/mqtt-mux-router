package rule

import (
    "encoding/json"
    "fmt"
    "regexp"
    "strconv"
    "strings"
)

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

// processTemplate handles the template substitution for both simple and nested values
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
        p.logger.Debug("template variable processed",
            "placeholder", placeholder,
            "value", strValue)

        result = strings.ReplaceAll(result, placeholder, strValue)
    }

    return result, nil
}

// getValueFromPath retrieves a value from nested maps using a path
func (p *Processor) getValueFromPath(data map[string]interface{}, path []string) (interface{}, error) {
    p.logger.Debug("getting value from path",
        "path", path,
        "availableKeys", getMapKeys(data))

    var current interface{} = data

    for i, key := range path {
        p.logger.Debug("accessing path segment",
            "segment", key,
            "index", i,
            "currentType", fmt.Sprintf("%T", current))

        switch v := current.(type) {
        case map[string]interface{}:
            var ok bool
            current, ok = v[key]
            if !ok {
                err := fmt.Errorf("key not found: %s", key)
                p.logger.Debug("key not found in map",
                    "key", key,
                    "availableKeys", getMapKeys(v),
                    "error", err)
                return nil, err
            }
        case map[interface{}]interface{}:
            var ok bool
            current, ok = v[key]
            if !ok {
                err := fmt.Errorf("key not found: %s", key)
                p.logger.Debug("key not found in interface map",
                    "key", key,
                    "error", err)
                return nil, err
            }
        default:
            err := fmt.Errorf("invalid path: %s is not a map", key)
            p.logger.Debug("invalid path segment",
                "segment", key,
                "currentType", fmt.Sprintf("%T", current),
                "error", err)
            return nil, err
        }
    }

    return current, nil
}

// convertToString converts a value to its string representation
func (p *Processor) convertToString(value interface{}) string {
    p.logger.Debug("converting value to string",
        "value", value,
        "type", fmt.Sprintf("%T", value))

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
        p.logger.Debug("using default string conversion",
            "type", fmt.Sprintf("%T", v))
        return fmt.Sprintf("%v", v)
    }
}
