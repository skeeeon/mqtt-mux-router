# MQTT Mux Router

A high-performance MQTT message router that processes messages from subscribed topics, evaluates conditions, and triggers actions based on configurable rules. The router is designed for reliability, performance, and easy monitoring in production environments.

## Features

- 🚀 High-performance message processing with worker pools
- 🔐 TLS support with client certificates
- 📝 Dynamic message templating with nested path support
- 📋 Configurable logging with multiple outputs
- 🔄 Automatic reconnection handling with subscription recovery
- 🎯 Complex condition evaluation with AND/OR logic
- 📊 Optional Prometheus metrics integration
- 💾 Efficient memory usage with object pooling
- 🔍 Fast rule matching with indexed lookups
- ⚙️ Comprehensive configuration system

## Quick Start

1. Clone the repository:
```bash
git clone https://github.com/skeeeon/mqtt-mux-router
cd mqtt-mux-router
```

2. Copy the example configuration:
```bash
cp config/config.example.json config/config.json
```

3. Build the binary:
```bash
go build -o mqtt-mux-router ./cmd/mqtt-mux-router
```

4. Start the router:
```bash
./mqtt-mux-router -config config/config.json
```

## Project Structure

```
mqtt-mux-router/
├── cmd/
│   └── mqtt-mux-router/
│       └── main.go                   # Application entry point
├── config/
│   ├── config.go                     # Configuration structures
│   └── config.example.json           # Example configuration
├── internal/
│   ├── broker/
|   │   │   ├── broker.go
|   │   │   └── mqtt/
|   │   │       ├── broker.go         
|   │   │       ├── connection.go    
|   │   │       ├── interfaces.go
|   │   │       ├── publisher.go
|   │   │       └── subscription.go
│   ├── rule/
│   │   ├── types.go                  # Rule data structures
│   │   ├── processor.go              # Rule processing and worker pool
│   │   ├── index.go                  # Rule indexing and lookup
│   │   ├── pool.go                   # Object pooling
│   │   └── loader.go                 # Rule file loading
│   ├── logger/
│   │   └── logger.go                 # Logging implementation
│   ├── metrics/
│   │   ├── metrics.go                # Prometheus metrics definitions
│   │   └── collector.go              # Metrics collection
│   └── stats/
│       └── stats.go                  # Performance metrics
├── rules/                            # Directory for rule files
│   └── example.json
├── go.mod
└── README.md
```

## Prerequisites

- Go 1.21 or higher
- MQTT Broker (e.g., Mosquitto, EMQ X)
- SSL certificates (if using TLS)
- Prometheus (optional, for metrics collection)

## Configuration

The application uses a comprehensive configuration file with optional command-line overrides for operational flexibility.

### Configuration File Structure

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
        "level": "info",
        "outputPath": "/var/log/mqtt-mux-router/mqtt-mux-router.log",
        "encoding": "json"
    },
    "metrics": {
        "enabled": true,
        "address": ":2112",
        "path": "/metrics",
        "updateInterval": "15s"
    },
    "processing": {
        "workers": 4,
        "queueSize": 1000,
        "batchSize": 100
    }
}
```

### Configuration Sections

#### MQTT Settings
- `broker`: MQTT broker address (required)
- `clientId`: Client identifier (required)
- `username`: Authentication username (optional)
- `password`: Authentication password (optional)
- `tls`: TLS configuration
  - `enable`: Enable TLS (true/false)
  - `certFile`: Client certificate path
  - `keyFile`: Client key path
  - `caFile`: CA certificate path

#### Logging Configuration
- `level`: Log level (debug, info, warn, error)
- `outputPath`: Log output destination (file path or "stdout")
- `encoding`: Log format (json or console)

#### Metrics Configuration
- `enabled`: Enable Prometheus metrics (true/false)
- `address`: Metrics server address (e.g., ":2112")
- `path`: Metrics endpoint path (e.g., "/metrics")
- `updateInterval`: Metrics collection interval (e.g., "15s")

#### Processing Configuration
- `workers`: Number of worker threads
- `queueSize`: Processing queue size
- `batchSize`: Message batch size

### Command Line Flags

Configuration options can be overridden via command line flags:

```bash
Usage of mqtt-mux-router:
  -config string
        path to config file (default "config/config.json")
  -rules string
        path to rules directory (default "rules")
  
  # Optional overrides
  -workers int
        override number of worker threads (0 = use config)
  -queue-size int
        override size of processing queue (0 = use config)
  -batch-size int
        override message batch size (0 = use config)
  -metrics-addr string
        override metrics server address (empty = use config)
  -metrics-path string
        override metrics endpoint path (empty = use config)
  -metrics-interval duration
        override metrics collection interval (0 = use config)
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
        "payload": "{\"alert\":\"High temperature!\",\"value\":${temperature},\"message_id\":\"${uuid7()}\"}"
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

## Metrics

The router exposes Prometheus metrics for monitoring system health and performance when metrics are enabled.

### Available Metrics

1. Message Processing:
- `messages_total` (counter) - Total messages by status (received/processed/error)
- `message_queue_depth` (gauge) - Current processing queue depth
- `message_processing_backlog` (gauge) - Difference between received and processed messages

2. Rule Engine:
- `rule_matches_total` (counter) - Total number of rule matches
- `rules_active` (gauge) - Current number of active rules

3. MQTT Connection:
- `mqtt_connection_status` (gauge) - Current connection status (0/1)
- `mqtt_reconnects_total` (counter) - Total number of reconnection attempts

4. Actions:
- `actions_total` (counter) - Total actions executed by status (success/error)

5. Template Processing:
- `template_operations_total` (counter) - Template processing operations by status

6. System:
- `process_goroutines` (gauge) - Current number of goroutines
- `process_memory_bytes` (gauge) - Current memory usage
- `worker_pool_active` (gauge) - Number of active workers

### Prometheus Configuration

Example Prometheus configuration:
```yaml
scrape_configs:
  - job_name: 'mqtt-mux-router'
    static_configs:
      - targets: ['localhost:2112']
    metrics_path: '/metrics'
    scrape_interval: 15s
```

## Performance Characteristics

Typical throughput on modern hardware (4 cores, 8GB RAM):
- Simple Rules: ~2,000-4,000 messages/second
- Complex Rules: ~600-1,000 messages/second

Memory usage is optimized through:
- Object pooling for messages and results
- Efficient rule indexing
- Controlled worker pools
- Configurable batch processing

### Performance Tuning

#### Worker Pool Configuration
- `workers`: Set based on available CPU cores and message complexity
- Recommended: CPU cores × 1.5 for compute-heavy rules
- Recommended: CPU cores × 2-4 for I/O-heavy rules

#### Queue Size Tuning
- `queueSize`: Buffer size for message spikes
- Increase for high-throughput scenarios
- Monitor memory usage when increasing
- Recommended: 1000-5000 for most use cases

#### Batch Processing
- `batchSize`: Number of messages processed in batch
- Larger batches improve throughput but increase latency
- Smaller batches reduce latency but may lower throughput
- Recommended: 100-500 for balanced performance

#### Memory Management
- Monitor `process_memory_bytes` metric
- Adjust queue sizes if memory pressure is high
- Consider reducing batch sizes if GC pressure is high

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
