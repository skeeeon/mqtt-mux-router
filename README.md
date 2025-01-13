# MQTT Mux Router

A powerful and flexible MQTT message router that processes messages from subscribed topics, evaluates conditions, and triggers actions based on configurable rules.

## Features

- 🔐 TLS support with client certificates
- 📝 JSON-based configuration
- 🔄 Automatic reconnection handling
- 📝 Dynamic message templating with nested path support
- 📋 Configurable logging with rotation and multiple outputs
- 📋 Rule-based message processing
- 🎯 Complex condition evaluation (AND/OR logic)
- 📊 Structured JSON logging
- 📁 Recursive rule file loading
- 🚀 High performance message processing

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

### MQTT Broker Settings

Create a `config.json` file:

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
    },
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

### Rule Configuration

Rules are defined in JSON files within the rules directory. Each rule specifies:
- Topic to subscribe to
- Conditions to evaluate
- Action to take when conditions are met

Example rule file (`rules/example.json`):

```json
[
    {
        "topic": "sensors/temperature",
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
                }
            ]
        },
        "action": {
            "topic": "alerts/temperature",
            "payload": "{\"alert\":\"High temperature and humidity!\",\"temp\":${temperature}}"
        }
    }
]
```

### Logging Configuration

The MQTT Mux Router supports comprehensive logging configuration with features like log rotation, multiple outputs, and different log levels. Configure logging in the `config.json` file:

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

#### Log Rotation

The router automatically handles log rotation based on:
- File size (maxSize)
- File age (maxAge)
- Number of backups (maxBackups)

Rotated files can optionally be compressed to save space.

#### Log Levels

- `debug`: Detailed information for debugging
- `info`: General operational information
- `warn`: Warning messages for potential issues
- `error`: Error messages for serious problems

#### Log Format

Logs are output in JSON format for easy parsing:

```json
{
    "time": "2024-01-12T10:00:00Z",
    "level": "INFO",
    "msg": "connected to mqtt broker",
    "broker": "ssl://mqtt.example.com:8883"
}
```

### Message Templating

The MQTT Mux Router supports dynamic message templating, allowing you to use values from input messages in your output messages. The template system supports both simple key-value replacements and nested JSON paths.

### Template Syntax
- Use `${key}` to insert a simple value
- Use `${path.to.value}` for nested values
- Templates support all JSON data types (strings, numbers, booleans, objects, arrays)

### Example Templates

Simple value:
```json
{
    "payload": "{\"temperature\": ${temperature}}"
}
```

Nested values:
```json
{
    "payload": "{\"location\": \"${device.location}\", \"reading\": ${sensors.temperature.value}}"
}
```

Complex objects:
```json
{
    "payload": "{\"device\": ${device}, \"readings\": ${sensors}}"
}
```

### Template Processing

The router automatically:
- Handles type conversion for different data types
- Processes nested JSON paths
- Maintains proper JSON formatting
- Provides error handling for missing values
- Logs template processing issues for debugging

### Example Rule with Templates

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
        "topic": "alerts/${device.id}",
        "payload": "{\"alert\": \"High temperature\", \"reading\": ${temperature}, \"location\": \"${device.location}\"}"
    }
}
```

Input message:
```json
{
    "temperature": 32.5,
    "device": {
        "id": "sensor-001",
        "location": "warehouse"
    }
}
```

Output message:
```json
{
    "alert": "High temperature",
    "reading": 32.5,
    "location": "warehouse"
}
```

## Supported Operators

### Comparison Operators
- `eq`: Equal to
- `neq`: Not equal to
- `gt`: Greater than
- `lt`: Less than
- `gte`: Greater than or equal to
- `lte`: Less than or equal to

### Logical Operators
- `and`: All conditions must be true
- `or`: At least one condition must be true

## Running the Application

```bash
# Run with default configuration paths
./mqtt-mux-router

# Run with custom paths
./mqtt-mux-router -config /path/to/config.json -rules /path/to/rules
```

## Rule Examples

### Simple Condition
```json
{
    "topic": "devices/status",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "status",
                "operator": "eq",
                "value": "offline"
            }
        ]
    },
    "action": {
        "topic": "alerts/device",
        "payload": "{\"alert\":\"Device offline\",\"deviceId\":${deviceId}}"
    }
}
```

### Complex Nested Conditions
```json
{
    "topic": "system/metrics",
    "conditions": {
        "operator": "and",
        "items": [
            {
                "field": "cpu_usage",
                "operator": "gt",
                "value": 80
            }
        ],
        "groups": [
            {
                "operator": "or",
                "items": [
                    {
                        "field": "memory_usage",
                        "operator": "gt",
                        "value": 90
                    },
                    {
                        "field": "swap_usage",
                        "operator": "gt",
                        "value": 50
                    }
                ]
            }
        ]
    },
    "action": {
        "topic": "alerts/system",
        "payload": "{\"alert\":\"System resources critical\"}"
    }
}
```

## Production Deployment

### Systemd Service

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

Enable and start the service:
```bash
sudo systemctl enable mqtt-mux-router
sudo systemctl start mqtt-mux-router
```

### Log Rotation

Create `/etc/logrotate.d/mqtt-mux-router`:

```
/var/log/mqtt-mux-router.log {
    daily
    rotate 7
    compress
    delaycompress
    missingok
    notifempty
}
```

## Monitoring

### Log Monitoring
```bash
# View service logs
journalctl -u mqtt-mux-router -f

# Check service status
systemctl status mqtt-mux-router
```

### Debugging

Enable debug logging by setting the log level in the logger configuration:

```go
handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
    Level: slog.LevelDebug,
})
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

## Acknowledgments

- [Eclipse Paho MQTT Go Client](https://github.com/eclipse/paho.mqtt.golang)
- [Go slog](https://pkg.go.dev/log/slog)
