# Arc OpenTelemetry Exporter

OpenTelemetry Collector exporter for [Arc](https://github.com/basekick-labs/arc), the unified observability database.

## Features

- ✅ **Traces**: Export distributed traces with full span hierarchy
- ✅ **Metrics**: Export all metric types (gauge, counter, histogram, summary)
- ✅ **Logs**: Export structured logs with attributes
- 🚀 **High Performance**: Uses Arc's columnar msgpack format for maximum throughput
- 📦 **Compression**: Automatic gzip compression for efficient network usage
- 🔄 **Retry Logic**: Configurable retry with exponential backoff
- 🔐 **Authentication**: Bearer token support

## Installation

### Option 1: Use with OCB (OpenTelemetry Collector Builder)

Add to your `builder-config.yaml`:

```yaml
exporters:
  - gomod: github.com/basekick-labs/arc-opentelemetry-exporter v0.1.0
```

Then build:

```bash
ocb --config builder-config.yaml
```

### Option 2: Pre-built Binary

Download from [releases page](https://github.com/basekick-labs/arc-opentelemetry-exporter/releases).

## Configuration

```yaml
exporters:
  arc:
    # Arc API endpoint (required)
    endpoint: http://localhost:8000

    # Authentication token (optional)
    # auth_token: your-arc-token

    # Database configuration
    # Option 1: Single database for all signals (simple)
    database: default

    # Option 2: Separate databases per signal type (recommended)
    # Provides clean separation and independent retention/scaling policies
    traces_database: traces
    metrics_database: metrics
    logs_database: logs

    # Measurement/table names (optional)
    traces_measurement: distributed_traces
    logs_measurement: logs

    # Note: Metrics automatically use metric name as table name
    # e.g., "system.cpu.usage" -> "system_cpu_usage" table in metrics_database

    # HTTP client settings (optional)
    timeout: 30s
    compression: gzip

    # Retry configuration (optional)
    retry_on_failure:
      enabled: true
      initial_interval: 5s
      max_interval: 30s
      max_elapsed_time: 300s

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [arc]

    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [arc]

    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [arc]
```

## Quick Start

### 1. Start Arc

```bash
docker run -d -p 8000:8000 \
  -e STORAGE_BACKEND=local \
  -v arc-data:/app/data \
  ghcr.io/basekick-labs/arc:25.11.1
```

### 2. Create OTel Collector Config

```yaml
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
      http:
        endpoint: 0.0.0.0:4318

processors:
  batch:
    timeout: 1s
    send_batch_size: 1000

exporters:
  arc:
    endpoint: http://localhost:8000
    database: default

service:
  pipelines:
    traces:
      receivers: [otlp]
      processors: [batch]
      exporters: [arc]
    metrics:
      receivers: [otlp]
      processors: [batch]
      exporters: [arc]
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [arc]
```

### 3. Run OTel Collector

```bash
./otelcol-custom --config=config.yaml
```

### 4. Send Telemetry Data

Your applications instrumented with OpenTelemetry SDKs will now send data to Arc!

## Data Format

The exporter uses Arc's high-performance columnar msgpack format:

### Traces Format
```json
{
  "m": "distributed_traces",
  "columns": {
    "time": [1699900000000, ...],
    "trace_id": ["abc123...", ...],
    "span_id": ["def456...", ...],
    "parent_span_id": ["ghi789...", ...],
    "service_name": ["api-gateway", ...],
    "operation_name": ["HTTP GET", ...],
    "span_kind": ["server", ...],
    "duration_ns": [1234567, ...],
    "status_code": [200, ...],
    "attributes": [{"key": "value"}, ...]
  }
}
```

### Metrics Format

**Important:** Each metric name becomes its own table (measurement) in Arc. This prevents schema conflicts between different metric types.

**Example 1: Counter Metric**
```json
{
  "m": "http_requests_total",  // Metric name as table name
  "columns": {
    "time": [1699900000000, ...],
    "value": [42.0, 105.0, ...],
    "labels": [
      {"service": "api", "method": "GET", "status": "200"},
      {"service": "api", "method": "POST", "status": "201"},
      ...
    ]
  }
}
```

**Example 2: Gauge Metric**
```json
{
  "m": "system_cpu_usage",  // system.cpu.usage -> system_cpu_usage
  "columns": {
    "time": [1699900000000, ...],
    "value": [45.5, 52.3, ...],
    "labels": [
      {"host": "server1", "cpu": "cpu0"},
      {"host": "server1", "cpu": "cpu1"},
      ...
    ]
  }
}
```

**Metric Name Sanitization:**
- Dots (`.`) → Underscores (`_`)
- Dashes (`-`) → Underscores (`_`)
- Special characters removed

Examples:
- `system.cpu.usage` → `system_cpu_usage`
- `http.server.duration` → `http_server_duration`
- `process-memory-bytes` → `process_memory_bytes`

### Logs Format
```json
{
  "m": "logs",
  "columns": {
    "time": [1699900000000, ...],
    "severity": ["INFO", ...],
    "body": ["Request processed", ...],
    "trace_id": ["abc123...", ...],
    "span_id": ["def456...", ...],
    "attributes": [{"user_id": "123"}, ...]
  }
}
```

## Database Organization Strategies

### Strategy 1: Single Database (Default)

All signals in one database - simplest configuration.

```yaml
database: default
```

**Structure:**
```
default/
  ├── distributed_traces      (table)
  ├── logs                     (table)
  ├── system_cpu_usage         (table)
  ├── system_memory_usage      (table)
  └── http_requests_total      (table)
```

**Pros:** Simple, easy correlation across signals
**Cons:** All data in one namespace

### Strategy 2: Database Per Signal Type (Recommended)

Separate databases for traces, metrics, and logs - like Telegraf pattern.

```yaml
traces_database: traces
metrics_database: metrics
logs_database: logs
```

**Structure:**
```
traces/
  └── distributed_traces

metrics/
  ├── system_cpu_usage
  ├── system_memory_usage
  ├── http_requests_total
  └── ... (each metric = table)

logs/
  └── logs
```

**Pros:**
- Clean separation of concerns
- Independent retention policies per signal type
- Independent scaling (different storage backends)
- Easier to manage permissions
- Matches traditional observability architecture

**Cons:** Slightly more complex configuration

**Recommended for production deployments.**

## Querying Data in Arc

### Traces

```sql
-- All traces
SELECT * FROM distributed_traces
WHERE time > now() - INTERVAL '1 hour'
LIMIT 100;

-- Traces by service
SELECT service_name, operation_name, duration_ns / 1000000 AS duration_ms
FROM distributed_traces
WHERE service_name = 'api-gateway'
ORDER BY time DESC;
```

### Metrics

Each metric is in its own table. List all tables to see available metrics:

```sql
-- Show all metric tables (if using separate database)
-- First, connect to metrics database
USE metrics;
SHOW TABLES;
```

Query specific metrics:

```sql
-- CPU usage
SELECT time, value, labels
FROM metrics.system_cpu_usage
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC;

-- HTTP requests (if using labels as JSON)
SELECT
  time,
  value,
  labels->>'method' AS method,
  labels->>'status' AS status
FROM metrics.http_requests_total
WHERE time > now() - INTERVAL '1 hour';

-- Or without database prefix if already connected
SELECT time, value, labels
FROM system_cpu_usage
WHERE time > now() - INTERVAL '1 hour';
```

### Logs

```sql
-- Recent logs
SELECT time, severity, body, service_name
FROM logs
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC
LIMIT 100;

-- Error logs
SELECT *
FROM logs
WHERE severity IN ('ERROR', 'FATAL')
  AND time > now() - INTERVAL '1 hour';
```

## Performance

The Arc exporter is designed for high throughput:

- **Traces**: 500K-1M spans/sec
- **Metrics**: 3M-6M data points/sec
- **Logs**: 1M-2M logs/sec

Performance depends on:
- Batch size (use `batch` processor)
- Network latency
- Arc instance resources
- Compression settings

## Development

### Prerequisites

- Go 1.22+
- OpenTelemetry Collector Builder (ocb)

### Build

```bash
go build -o arc-exporter ./cmd/arc-exporter
```

### Test

```bash
go test ./...
```

### Local Development

```bash
# Install dependencies
go mod download

# Run tests
go test -v ./...

# Build
go build ./...
```

## Contributing

Contributions welcome! Please open an issue or PR.

## License

Apache 2.0 - See [LICENSE](LICENSE)

## Links

- [Arc Database](https://github.com/basekick-labs/arc)
- [Arc Documentation](https://basekick.net/docs)
- [OpenTelemetry Collector](https://opentelemetry.io/docs/collector/)
