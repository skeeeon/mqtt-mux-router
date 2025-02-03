# MQTT Mux Router

A high-performance MQTT message router that processes messages from subscribed topics, evaluates conditions, and triggers actions based on configurable rules.

## Features

- 🚀 High-performance message processing with worker pools
- 🔐 TLS support with client certificates
- 📝 Dynamic message templating with nested path support
- 📋 Configurable logging with multiple outputs
- 🔄 Automatic reconnection handling with subscription recovery
- 🎯 Complex condition evaluation with AND/OR logic
- 📊 Comprehensive performance metrics and monitoring
- 💾 Efficient memory usage with object pooling
- 🔍 Fast rule matching with indexed lookups

## Performance

Typical throughput on modern hardware (4 cores, 8GB RAM):
- Simple Rules: ~2,000-4,000 messages/second
- Complex Rules: ~600-1,000 messages/second

Memory usage is optimized through:
- Object pooling for messages and results
- Efficient rule indexing
- Controlled worker pools
- Configurable batch processing

## Project Structure

```
mqtt-mux-router/
├── cmd/
│   └── mqtt-mux-router/
│       └── main.go                # Application entry point
├── config/
│   └── config.go                  # Configuration structures
├── internal/
│   ├── broker/
│   │   └── broker.go             # MQTT broker implementation
│   ├── rule/
│   │   ├── types.go              # Rule data structures
│   │   ├── processor.go          # Rule processing and worker pool
│   │   ├── index.go              # Rule indexing and lookup
│   │   ├── pool.go               # Object pooling
│   │   └── loader.go             # Rule file loading
│   ├── logger/
│   │   └── logger.go             # Logging implementation
│   └── stats/
│       └── stats.go              # Performance metrics
├── rules/                        # Directory for rule files
│   └── example.json
├── go.mod
└── README.md
```

## Installation

### Prerequisites

- Go 1.21 or higher
- MQTT Broker (e.g., Mosquitto, EMQ X)
- SSL certificates (if using TLS)

### Building from Source

```bash
# Clone the repository
git clone https://github.com/skeeeon/mqtt-mux-router
cd mqtt-mux-router

# Build the binary
go build -o mqtt-mux-router ./cmd/mqtt-mux-router
```

## Configuration

### Application Settings

```bash
Usage of mqtt-mux-router:
  -config string
        path to config file (default "config/config.json")
  -rules string
        path to rules directory (default "rules")
  -workers int
        number of worker threads (default: number of CPU cores)
  -queue-size int
        size of processing queue (default 1000)
  -batch-size int
        message batch size (default 100)
```

### MQTT Settings
Configure MQTT connection in `config.json`:

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

```json
{
    "logging": {
        "level": "info",
        "outputPath": "/var/log/mqtt-mux-router/mqtt-mux-router.log",
        "encoding": "json"
    }
}
```

## Rule Configuration

Rules define message routing and transformation logic:

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
        "payload": "{\"alert\":\"High temperature!\",\"value\":${temperature},\"message_id\":${uuid7()}}"
    }
}
```
### Template Functions

The router supports the following template functions:

- `${uuid4()}`: Generates a random UUID v4
  - Use for random identifiers
  - Example: `550e8400-e29b-41d4-a716-446655440000`

- `${uuid7()}`: Generates a time-ordered UUID v7
  - Use for event tracking and time-ordered identifiers
  - Includes millisecond precision timestamp
  - Example: `0188c57c-e1f1-7c63-b4f6-b9c2e4712fb1`

### Condition Operators

- `eq`: Equal to
- `neq`: Not equal to
- `gt`: Greater than
- `lt`: Less than
- `gte`: Greater than or equal to
- `lte`: Less than or equal to
- `exists`: Check if field exists
- `contains`: Check if string contains value

### Logical Operators
- `and`: All conditions must be true
- `or`: At least one condition must be true

## Performance Tuning

1. Worker Pool:
   - Set workers based on CPU cores (`-workers`)
   - Adjust for message processing complexity
   - Monitor CPU utilization

2. Queue Size:
   - Buffer for message spikes (`-queue-size`)
   - Memory vs latency trade-off
   - Monitor queue utilization

3. Batch Processing:
   - Optimize throughput (`-batch-size`)
   - Balance latency vs throughput
   - Adjust based on message patterns

4. Memory Management:
   - Object pooling reduces GC pressure
   - Configurable pool sizes
   - Automatic cleanup

## Monitoring

### Performance Metrics

The application tracks:
- Messages received/second
- Processing throughput
- Rule match rates
- Error counts
- Memory usage
- Queue depths

### Log Levels

- **INFO**: Important state changes and operations
- **DEBUG**: Detailed processing information
- **ERROR**: Issues requiring attention

### Viewing Logs

```bash
# View latest logs
tail -f /var/log/mqtt-mux-router/mqtt-mux-router.log

# Follow specific log levels
tail -f /var/log/mqtt-mux-router/mqtt-mux-router.log | grep "level\":\"error\""

# View performance metrics
tail -f /var/log/mqtt-mux-router/mqtt-mux-router.log | grep "stats"
```

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
