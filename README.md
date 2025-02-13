# MQTT Mux Router

A high-performance MQTT message router that processes messages from multiple brokers, evaluates conditions, and triggers actions based on configurable rules. The router supports flexible source and target broker configurations, making it ideal for complex MQTT routing scenarios and multi-broker environments.

## Features

- 🌟 Multi-broker support with role-based routing (source/target/both)
- 🚀 High-performance message processing with worker pools
- 🔐 TLS support with client certificates
- 📝 Dynamic message templating with variable substitution
- 🎯 Complex condition evaluation with AND/OR logic
- 📋 Configurable logging with multiple outputs
- 🔄 Automatic reconnection handling with subscription recovery
- 📊 Comprehensive Prometheus metrics for each broker
- 💾 Efficient memory usage with object pooling
- 🔍 Fast rule matching with indexed lookups
- ⚙️ Comprehensive configuration system
- 🔀 Flexible broker-to-broker routing

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
│       └── main.go                # Application entry point
├── config/
│   ├── config.go                  # Configuration structures
│   └── config.example.json        # Example configuration
├── internal/
│   ├── broker/
│   │   ├── types.go              # Broker types and interfaces
│   │   ├── manager.go            # Multi-broker management
│   │   └── subscription.go       # Topic subscription handling
│   ├── rule/
│   │   ├── types.go              # Rule data structures
│   │   ├── processor.go          # Rule processing and worker pool
│   │   ├── index.go              # Rule indexing and lookup
│   │   ├── evaluator.go          # Condition evaluation
│   │   ├── pool.go               # Object pooling
│   │   ├── loader.go             # Rule file loading
│   │   └── validator.go          # Rule validation
│   ├── logger/
│   │   └── logger.go             # Logging implementation
│   ├── metrics/
│   │   ├── metrics.go            # Prometheus metrics definitions
│   │   └── collector.go          # Metrics collection
│   └── stats/
│       └── stats.go              # Performance metrics
├── rules/                        # Directory for rule files
│   └── example.json
├── go.mod
└── README.md
```

## Prerequisites

- Go 1.21 or higher
- MQTT Broker(s) (e.g., Mosquitto, EMQ X)
- SSL certificates (if using TLS)
- Prometheus (optional, for metrics collection)

## Configuration

The application uses a comprehensive configuration file with optional command-line overrides for operational flexibility. Backward compatibility is maintained for single-broker configurations.

### Configuration File Structure

```json
{
    "brokers": {
        "source1": {
            "id": "source1",
            "role": "source",            # Required: "source", "target", or "both"
            "address": "ssl://source.mqtt.example.com:8883",
            "clientId": "mux-source1",
            "username": "user",          # Optional
            "password": "pass",          # Optional
            "tls": {                     # Optional
                "enable": true,
                "certFile": "certs/client-cert.pem",  # Required if TLS enabled
                "keyFile": "certs/client-key.pem",    # Required if TLS enabled
                "caFile": "certs/ca.pem"              # Required if TLS enabled
            }
        },
        "target1": {
            "id": "target1",
            "role": "target",
            "address": "mqtt://target.mqtt.example.com:1883",
            "clientId": "mux-target1"
        }
    },
    "logging": {
        "level": "info",                # Optional: "debug", "info", "warn", "error" (default: "info")
        "outputPath": "stdout",         # Optional: file path or "stdout" (default: "stdout")
        "encoding": "json"              # Optional: "json" or "console" (default: "json")
    },
    "metrics": {
        "enabled": true,                # Optional (default: false)
        "address": ":2112",            # Optional (default: ":2112")
        "path": "/metrics",            # Optional (default: "/metrics")
        "updateInterval": "15s"        # Optional (default: "15s"), valid time.Duration format
    },
    "processing": {
        "workers": 4,                  # Optional (default: number of CPU cores)
        "queueSize": 1000,            # Optional (default: 1000)
        "batchSize": 100              # Optional (default: 100)
    }
}
```

### Configuration Requirements

#### Broker Configuration
- At least one source broker (role: "source" or "both") is required
- At least one target broker (role: "target" or "both") is required
- Each broker must have a unique ID that matches its map key
- `address` and `clientId` are required for each broker
- When TLS is enabled, all TLS fields (certFile, keyFile, caFile) are required

#### Logging Configuration
Default values if not specified:
- `level`: "info"
- `outputPath`: "stdout"
- `encoding`: "json"

Valid log levels: "debug", "info", "warn", "error"

#### Metrics Configuration
Default values if not specified:
- `enabled`: false
- `address`: ":2112"
- `path`: "/metrics"
- `updateInterval`: "15s"

The updateInterval must be a valid Go duration string (e.g., "10s", "1m", "500ms")

#### Processing Configuration
Default values if not specified:
- `workers`: number of CPU cores
- `queueSize`: 1000
- `batchSize`: 100

### Backward Compatibility

The configuration supports backward compatibility for single-broker setups. The following legacy format is automatically converted:

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
        "outputPath": "stdout",
        "encoding": "json"
    },
    "metrics": {
        "enabled": true,
        "address": ":2112",
        "path": "/metrics"
    },
    "processing": {
        "workers": 4,
        "queueSize": 1000,
        "batchSize": 100
    }
}
```

This legacy format is automatically converted to a multi-broker configuration with a single broker having the role "both".

## Rule Configuration

Rules define message routing and transformation logic between brokers. Rules can be defined individually or grouped in rule sets.

### Rule Structure

```json
{
    "topic": "sensors/+/temperature",
    "sourceBroker": "source1",            // Optional: defaults to any source broker
    "description": "Temperature alerts",   // Optional: rule description
    "enabled": true,                      // Required: rule status
    "priority": 1,                        // Optional: processing priority (default: 0)
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
        "topic": "alerts/${device_id}/temperature",
        "targetBroker": "target1",        // Required: must be a target/both broker
        "payload": "{\"alert\":\"High temperature!\",\"value\":${temperature}}",
        "qos": 1,
        "retain": false,
        "headers": {                       // Optional: custom headers
            "type": "temperature_alert",
            "severity": "high"
        }
    },
    "createdAt": "2024-02-12T12:00:00Z",  // Managed by system
    "updatedAt": "2024-02-12T12:00:00Z"   // Managed by system
}
```

### Rule Set Structure

Multiple rules can be grouped in a rule set:

```json
{
    "name": "Temperature Monitoring",
    "description": "Temperature monitoring rules",
    "version": "1.0",
    "rules": [
        {
            "topic": "sensors/+/temperature",
            // ... rule fields as above
        }
    ],
    "createdAt": "2024-02-12T12:00:00Z",
    "updatedAt": "2024-02-12T12:00:00Z"
}
```

### Topic Patterns

Rules support MQTT-style topic patterns with wildcards:
- `+`: Single-level wildcard
  - Example: `sensors/+/temperature` matches `sensors/room1/temperature`
  - Must occupy entire segment
- `#`: Multi-level wildcard
  - Example: `sensors/#` matches any topic starting with `sensors/`
  - Must be the last segment
- No wildcards in published topics
- Empty segments allowed only at start/end of topic

### Template Substitution

The router supports variable substitution in topics and payloads:

- Message field variables:
  - Format: `${field_name}`
  - Example: `${temperature}` substitutes the temperature value
  - Required in topics (error if missing)
  - Optional in payloads (empty string if missing)

- Topic segment variables:
  - Format: `${_topic_N}` where N is the segment index (1-based)
  - Example: With topic `sensors/room1/temperature`:
    - `${_topic_1}` = "sensors"
    - `${_topic_2}` = "room1"

### Condition Operators

Comparison operators for field values:
- `eq`: Equal to
- `neq`: Not equal to
- `gt`: Greater than
- `lt`: Less than
- `gte`: Greater than or equal to
- `lte`: Less than or equal to
- `exists`: Check if field exists
- `contains`: String contains value
- `matches`: Regular expression match

Values are automatically converted for comparison:
- Numbers compared numerically
- Strings compared lexicographically
- Booleans compared as true > false
- Mixed types use string comparison
- Null values handled appropriately

### Logical Operators

Conditions can be combined using logical operators:
- `and`: All conditions must be true
- `or`: At least one condition must be true

Conditions can be nested:
```json
{
    "operator": "and",
    "items": [
        {
            "field": "temperature",
            "operator": "gt",
            "value": 30
        }
    ],
    "groups": [
        {
            "operator": "or",
            "items": [
                {
                    "field": "humidity",
                    "operator": "gt",
                    "value": 80
                },
                {
                    "field": "pressure",
                    "operator": "lt",
                    "value": 1000
                }
            ]
        }
    ]
}
```

### Broker Targeting

Rules can specify source and target brokers:

- Source Broker (`sourceBroker`):
  - Optional field
  - When specified, rule only matches messages from that broker
  - Must reference a broker with role "source" or "both"
  - When omitted, matches messages from any source broker

- Target Broker (`targetBroker`):
  - Required field
  - Must reference a broker with role "target" or "both"
  - Error if broker doesn't exist or has incompatible role

### Rule Validation

Rules are validated when loaded:
- Topic patterns must be valid MQTT topics
- Wildcards must follow MQTT rules
- Conditions must use valid operators
- Template variables must be valid
- Broker references must be valid
- Required fields must be present
- QoS must be 0, 1, or 2

### System Fields

Some fields are managed by the system:
- `createdAt`: Set when rule is first loaded
- `updatedAt`: Updated when rule is modified
- Both use RFC3339 format timestamps
- Cannot be set manually in rule files

### Processing Behavior

- Rules are processed in priority order (higher numbers first)
- Within same priority, order is undefined
- Disabled rules are skipped
- All matching rules are processed
- Processing continues after action errors
- Template errors prevent action execution

## Metrics

The router exposes Prometheus metrics for monitoring system health and performance:

### Available Metrics

1. Broker Metrics:
- `mqtt_connection_status` (gauge) - Connection status by broker ID
- `mqtt_reconnects_total` (counter) - Reconnection attempts by broker
- `mqtt_messages_total` (counter) - Messages by broker and direction
- `mqtt_broker_errors_total` (counter) - Errors by broker and type

2. Message Processing:
- `messages_total` (counter) - Total messages by status
- `message_queue_depth` (gauge) - Current processing queue depth
- `message_processing_backlog` (gauge) - Processing backlog

3. Rule Engine:
- `rule_matches_total` (counter) - Rule matches by topic
- `rules_active` (gauge) - Active rules by type
- `routing_success_total` (counter) - Successful message routes
- `routing_errors_total` (counter) - Failed message routes

4. System:
- `process_goroutines` (gauge) - Current goroutines
- `process_memory_bytes` (gauge) - Memory usage
- `worker_pool_active` (gauge) - Active workers

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
- Simple Rules: ~2,000-4,000 messages/second/broker
- Complex Rules: ~600-1,000 messages/second/broker
- Multi-broker Setup: Scales linearly with CPU cores

Memory usage is optimized through:
- Object pooling for messages and results
- Efficient rule indexing
- Controlled worker pools
- Configurable batch processing

### Performance Tuning

#### Worker Pool Configuration
- `workers`: Set based on available CPU cores and number of brokers
- Recommended: CPU cores × 1.5 for compute-heavy rules
- Recommended: CPU cores × 2-4 for I/O-heavy rules

#### Queue Size Tuning
- `queueSize`: Buffer size for message spikes
- Increase for high-throughput scenarios
- Monitor memory usage when increasing
- Recommended: 1000-5000 per broker for most use cases

#### Batch Processing
- `batchSize`: Number of messages processed in batch
- Larger batches improve throughput but increase latency
- Smaller batches reduce latency but may lower throughput
- Recommended: 100-500 for balanced performance

#### Memory Management
- Monitor `process_memory_bytes` metric
- Adjust queue sizes if memory pressure is high
- Consider reducing batch sizes if GC pressure is high
- Set appropriate limits for number of concurrent connections

## Contributing

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License. See the [LICENSE](LICENSE) file for details.
