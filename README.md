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

## Configuration

The application uses a comprehensive configuration file with optional command-line overrides for operational flexibility.

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

## Rule Configuration

Rules define message routing and transformation logic between brokers. Rules can be defined individually or grouped in rule sets.

### Rule Structure

```json
{
    "topic": "sensors/+/temperature",
    "sourceBroker": "source1",            // Optional: defaults to any source broker
    "description": "Temperature alerts",   // Optional: rule description
    "enabled": true,                      // Required: rule status
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
        "retain": false
    },
    "createdAt": "2024-02-12T12:00:00Z"  // Managed by system
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
    "createdAt": "2024-02-12T12:00:00Z"
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

## Performance Characteristics

### Message Processing Pipeline

1. Message Reception
   - Efficient MQTT client connection handling
   - Queue-based message buffering
   - Automatic backpressure handling

2. Rule Matching
   - Optimized topic tree for fast pattern matching
   - In-memory rule index
   - Parallel rule evaluation

3. Action Execution
   - Efficient template processing
   - Worker pool for parallel action execution
   - Message batching for improved throughput

### Typical Performance

On modern hardware (4 cores, 8GB RAM):
- Simple Rules: ~2,000-4,000 messages/second/broker
- Complex Rules: ~600-1,000 messages/second/broker
- Multi-broker Setup: Scales linearly with CPU cores

### Memory Usage

Memory usage is optimized through:
- Object pooling for messages and results
- Efficient rule indexing
- Controlled worker pools
- Configurable batch processing

### Tuning Guidelines

1. Worker Pool Configuration
   - Set workers based on available CPU cores
   - Recommended: CPU cores × 1.5 for compute-heavy rules
   - Recommended: CPU cores × 2-4 for I/O-heavy rules

2. Queue Size
   - Buffer size for message spikes
   - Default: 1000 messages
   - Increase for high-throughput scenarios
   - Monitor memory usage when increasing

3. Batch Processing
   - Controls message processing batch size
   - Default: 100 messages
   - Larger batches = higher throughput, higher latency
   - Smaller batches = lower latency, lower throughput

4. Memory Management
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
