# MQTT Mux Router

A powerful and flexible MQTT message router that processes messages from subscribed topics, evaluates conditions, and triggers actions based on configurable rules.

## Features

- 🔐 TLS support with client certificates
- 📝 Dynamic message templating with nested path support
- 📋 Configurable logging with rotation and multiple outputs
- 🔄 Automatic reconnection handling
- 🎯 Complex condition evaluation with AND/OR logic
- 📊 Structured JSON logging with detailed debug information

## Project Structure

```
mqtt-mux-router/
├── cmd/
│   └── mqtt-mux-router/
│       └── main.go              # Application entry point
├── config/
│   └── config.go               # Configuration structures and loading
├── internal/
│   ├── broker/
│   │   └── broker.go           # MQTT broker implementation
│   ├── rule/
│   │   ├── types.go            # Rule data structures
│   │   ├── processor.go        # Rule processing and management
│   │   ├── evaluator.go        # Rule condition evaluation
│   │   └── template.go         # Template processing
│   └── logger/
│       └── logger.go           # Logging implementation
├── rules/                      # Directory for rule files
│   └── example.json
├── go.mod
└── README.md
```

## Table of Contents
- [Installation](#installation)
- [Configuration](#configuration)
  - [MQTT Settings](#mqtt-settings)
  - [Logging Configuration](#logging-configuration)
- [Rules](#rules)
  - [Rule Structure](#rule-structure)
  - [Condition Operators](#condition-operators)
  - [Message Templating](#message-templating)
- [Monitoring and Debugging](#monitoring-and-debugging)
- [Deployment](#deployment)

## Key Components

### Broker (internal/broker/broker.go)
- MQTT client management
- Connection handling
- Message routing
- TLS support

### Rule Processing (internal/rule/)
- **types.go**: Core data structures for rules
- **processor.go**: Rule loading and management
- **evaluator.go**: Condition evaluation logic
- **template.go**: Dynamic message templating

### Configuration (config/config.go)
- MQTT broker settings
- TLS configuration
- Logging settings

### Logger (internal/logger/logger.go)
- Structured JSON logging
- File rotation support
- Multiple output targets
- Configurable log levels

## Installation

### Prerequisites

- Go 1.21 or higher
- MQTT Broker (e.g., Mosquitto, EMQ X)
- SSL certificates (if using TLS)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/yourusername/mqtt-mux-router
cd mqtt-mux-router

# Build the binary
go build -o mqtt-mux-router ./cmd/mqtt-mux-router
```

## Configuration

### MQTT Settings

Configure MQTT connection settings in `config.json`:

```json
{
    "mqtt": {
        "broker": "ssl://mqtt.example.com:8883",
        "clientId": "mqtt-mux-router",
        "username": "user",
        "password": "pass",
        "tls": {
            "enable": true,
            "certFile": "certs/client-cert.pem",
            "keyFile": "certs/client-key.pem",
            "caFile": "certs/ca.pem"
        }
    }
}
```

### Logging Configuration

Configure logging behavior in the same `config.json`:

```json
{
    "logging": {
        "directory": "/var/log/mqtt-mux-router",
        "maxSize": 10,
        "maxAge": 7,
        "maxBackups": 5,
        "compress": true,
        "logToFile": true,
        "logToStdout": true,
        "level": "info"
    }
}
```

#### Logging Options

| Option | Description | Default |
|--------|-------------|---------|
| `directory` | Directory for log files | - |
| `maxSize` | Maximum size in megabytes before rotation | 10 |
| `maxAge` | Days to retain old log files | 7 |
| `maxBackups` | Number of old log files to retain | 5 |
| `compress` | Compress rotated files | false |
| `logToFile` | Enable file logging | false |
| `logToStdout` | Enable stdout logging | true |
| `level` | Log level (debug, info, warn, error) | info |

## Rules

Rules define how messages are processed and routed. Each rule consists of a subscription topic, conditions for evaluation, and an action to perform when conditions are met.

### Rule Structure

Basic rule structure:
```json
{
    "topic": "topic/to/subscribe",
    "conditions": {
        "operator": "and|or",
        "items": [...conditions...],
        "groups": [...nested condition groups...]
    },
    "action": {
        "topic": "topic/to/publish",
        "payload": "message template"
    }
}
```

### Rule Examples

#### 1. Simple Single Condition
Monitor temperature and send alert when too high:
```json
{
    "topic": "sensors/temperature",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "temperature",
                "operator": "gt",
                "value": 30
            }
        ]
    },
    "action": {
        "topic": "alerts/temperature",
        "payload": "{\"alert\":\"High temperature detected\",\"value\":${temperature}}"
    }
}
```

#### 2. Multiple Conditions with AND
Monitor environmental conditions:
```json
{
    "topic": "sensors/environment",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "temperature",
                "operator": "gt",
                "value": 30
            },
            {
                "field": "humidity",
                "operator": "gt",
                "value": 70
            },
            {
                "field": "status",
                "operator": "eq",
                "value": "active"
            }
        ]
    },
    "action": {
        "topic": "alerts/environment/${device.id}",
        "payload": "{\"alert\":\"Environmental conditions critical\",\"temp\":${temperature},\"humidity\":${humidity}}"
    }
}
```

#### 3. Multiple Conditions with OR
Monitor device status:
```json
{
    "topic": "devices/+/status",
    "conditions": {
        "operator": "or",
        "items": [
            {
                "field": "battery",
                "operator": "lt",
                "value": 10
            },
            {
                "field": "signal",
                "operator": "lt",
                "value": 20
            },
            {
                "field": "error",
                "operator": "exists",
                "value": true
            }
        ]
    },
    "action": {
        "topic": "maintenance/required/${device.id}",
        "payload": "{\"device\":\"${device.id}\",\"issues\":{\"battery\":${battery},\"signal\":${signal},\"error\":${error}}}"
    }
}
```

#### 4. Nested Conditions with Groups
Complex system monitoring:
```json
{
    "topic": "system/metrics",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "environment",
                "operator": "eq",
                "value": "production"
            }
        ],
        "groups": [
            {
                "operator": "or",
                "items": [
                    {
                        "field": "cpu.usage",
                        "operator": "gt",
                        "value": 90
                    },
                    {
                        "field": "memory.usage",
                        "operator": "gt",
                        "value": 85
                    }
                ]
            },
            {
                "operator": "and",
                "items": [
                    {
                        "field": "disk.available",
                        "operator": "lt",
                        "value": 10
                    },
                    {
                        "field": "disk.write_rate",
                        "operator": "gt",
                        "value": 100
                    }
                ]
            }
        ]
    },
    "action": {
        "topic": "alerts/system/${system.id}",
        "payload": "{\"alert\":\"System resources critical\",\"metrics\":{\"cpu\":${cpu.usage},\"memory\":${memory.usage},\"disk\":{\"available\":${disk.available},\"write_rate\":${disk.write_rate}}}}"
    }
}
```

#### 5. String Operations and Pattern Matching
Log message filtering:
```json
{
    "topic": "logs/application",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "level",
                "operator": "eq",
                "value": "error"
            },
            {
                "field": "message",
                "operator": "contains",
                "value": "database"
            }
        ],
        "groups": [
            {
                "operator": "or",
                "items": [
                    {
                        "field": "component",
                        "operator": "contains",
                        "value": "auth"
                    },
                    {
                        "field": "component",
                        "operator": "contains",
                        "value": "api"
                    }
                ]
            }
        ]
    },
    "action": {
        "topic": "notifications/critical/${application.id}",
        "payload": "{\"source\":\"${component}\",\"error\":\"${message}\",\"timestamp\":${timestamp}}"
    }
}
```

#### 6. Multi-Stage Processing
Temperature trend analysis:
```json
{
    "topic": "sensors/+/temperature",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "current",
                "operator": "gt",
                "value": 25
            }
        ],
        "groups": [
            {
                "operator": "and",
                "items": [
                    {
                        "field": "trend.direction",
                        "operator": "eq",
                        "value": "increasing"
                    },
                    {
                        "field": "trend.rate",
                        "operator": "gt",
                        "value": 2
                    }
                ]
            }
        ]
    },
    "action": {
        "topic": "analytics/temperature/${sensor.id}",
        "payload": "{\"alert\":\"Rapid temperature increase\",\"current\":${current},\"trend\":{\"direction\":\"${trend.direction}\",\"rate\":${trend.rate}},\"location\":\"${sensor.location}\"}"
    }
}
```

### Rule Best Practices

1. Topic Structure:
   - Use descriptive topic hierarchies
   - Use wildcards (+, #) carefully
   - Group related devices/sensors under common prefixes

2. Condition Design:
   - Start with simple conditions and add complexity as needed
   - Use groups to organize related conditions
   - Consider performance impact of complex nested conditions

3. Action Payloads:
   - Include relevant context in messages
   - Use consistent payload structures
   - Include timestamps and identifiers
   - Use nested JSON objects for complex data

4. Error Handling:
   - Include existence checks for optional fields
   - Provide default values in templates
   - Consider edge cases in numeric comparisons

5. Maintenance:
   - Document rule purposes and dependencies
   - Use consistent naming conventions
   - Regular review and cleanup of unused rules

### Condition Operators

#### Comparison Operators
- `eq`: Equal to
- `neq`: Not equal to
- `gt`: Greater than
- `lt`: Less than
- `gte`: Greater than or equal to
- `lte`: Less than or equal to
- `exists`: Check if field exists
- `contains`: Check if string contains value

#### Logical Operators
- `and`: All conditions must be true
- `or`: At least one condition must be true

### Message Templating

Template syntax supports both simple and nested values:

```json
{
    "payload": "{\"location\": \"${device.location}\", \"reading\": ${sensors.temperature.value}}"
}
```

#### Template Features
- Simple value: `${key}`
- Nested path: `${path.to.value}`
- Dynamic topics: `alerts/${device.id}/temperature`
- Automatic type conversion for:
  - Strings
  - Numbers
  - Booleans
  - Objects
  - Arrays
  - Null values

## Monitoring and Debugging

### Debug Level Logging

When `level` is set to `debug`, the router provides detailed logging for:

1. MQTT Operations:
```json
{
    "level": "debug",
    "msg": "mqtt client connected successfully",
    "broker": "ssl://mqtt.example.com:8883",
    "clientId": "mqtt-mux-router",
    "time": "2024-01-12T10:00:00Z"
}
```

2. Message Processing:
```json
{
    "level": "debug",
    "msg": "received message",
    "topic": "sensors/temperature",
    "payload": "{\"temperature\":32.5}",
    "qos": 0,
    "messageId": 12345,
    "time": "2024-01-12T10:00:01Z"
}
```

3. Rule Evaluation:
```json
{
    "level": "debug",
    "msg": "evaluating condition",
    "field": "temperature",
    "operator": "gt",
    "expectedValue": 30,
    "actualValue": 32.5,
    "result": true,
    "time": "2024-01-12T10:00:01Z"
}
```

### Viewing Logs

```bash
# View latest logs
tail -f /var/log/mqtt-mux-router/mqtt-mux-router.log

# Search for specific events
grep "mqtt client connected" /var/log/mqtt-mux-router/mqtt-mux-router.log

# View error logs
grep "level\":\"error\"" /var/log/mqtt-mux-router/mqtt-mux-router.log
```

### Log Management

```bash
# List all log files
ls -l /var/log/mqtt-mux-router/

# Check total log size
du -sh /var/log/mqtt-mux-router/

# Clean up old logs manually
find /var/log/mqtt-mux-router/ -name "mqtt-mux-router.log.*" -mtime +7 -delete
```

## Deployment

### Running the Application

```bash
# Run with default configuration
./mqtt-mux-router

# Run with custom paths
./mqtt-mux-router -config /path/to/config.json -rules /path/to/rules
```

### Service Management

Create a systemd service file `/etc/systemd/system/mqtt-mux-router.service`:

```ini
[Unit]
Description=MQTT Mux Router
After=network.target

[Service]
ExecStart=/usr/local/bin/mqtt-mux-router -config /etc/mqtt-mux-router/config.json -rules /etc/mqtt-mux-router/rules
Restart=always
User=mqtt-mux
Group=mqtt-mux

[Install]
WantedBy=multi-user.target
```

Enable and manage the service:
```bash
sudo systemctl enable mqtt-mux-router
sudo systemctl start mqtt-mux-router
sudo systemctl status mqtt-mux-router
```
