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

The exporter uses Arc's high-performance columnar msgpack format with **dynamic columns**. All attributes from OpenTelemetry (resource attributes and signal-specific attributes) are automatically converted into individual columns for optimal query performance.

### Traces Format

All span attributes and resource attributes become individual columns:

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
    "status_code": [0, ...],
    "status_message": ["", ...],
    "http.method": ["GET", ...],
    "http.status_code": [200, ...],
    "http.url": ["/api/users", ...],
    "host.name": ["server-1", ...]
  }
}
```

**Dynamic schema**: Columns are created automatically based on span attributes and resource attributes present in your traces.

### Metrics Format

**Important:** Each metric name becomes its own table (measurement) in Arc. This prevents schema conflicts between different metric types.

All metric attributes (data point attributes and resource attributes) become individual columns in Arc's columnar msgpack format:

**Example 1: Counter Metric**
```json
{
  "m": "http_requests_total",
  "columns": {
    "time": [1699900000000, 1699900001000, ...],
    "value": [42.0, 105.0, ...],
    "service": ["api", "api", ...],
    "method": ["GET", "POST", ...],
    "status": ["200", "201", ...],
    "host.name": ["server-1", "server-1", ...]
  }
}
```

**Example 2: Gauge Metric (System Monitoring)**
```json
{
  "m": "system_cpu_usage",
  "columns": {
    "time": [1699900000000, 1699900001000, ...],
    "value": [45.5, 52.3, ...],
    "host.name": ["macbook-pro", "macbook-pro", ...],
    "cpu": ["cpu0", "cpu1", ...],
    "state": ["idle", "user", ...]
  }
}
```

**Dynamic schema**: Columns are created automatically based on attributes present in your metrics.

**Metric Name Sanitization:**
- Dots (`.`) → Underscores (`_`)
- Dashes (`-`) → Underscores (`_`)
- Special characters removed

Examples:
- `system.cpu.usage` → `system_cpu_usage`
- `http.server.duration` → `http_server_duration`
- `process-memory-bytes` → `process_memory_bytes`

### Logs Format

All log attributes and resource attributes become individual columns:

```json
{
  "m": "logs",
  "columns": {
    "time": [1699900000000, ...],
    "severity": ["INFO", ...],
    "severity_number": [9, ...],
    "body": ["Request processed", ...],
    "trace_id": ["abc123...", ...],
    "span_id": ["def456...", ...],
    "trace_flags": [1, ...],
    "service_name": ["api-gateway", ...],
    "user_id": ["123", ...],
    "http.method": ["GET", ...],
    "host.name": ["server-1", ...]
  }
}
```

**Dynamic schema**: Columns are created automatically based on log attributes and resource attributes present in your logs.

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
SELECT * FROM traces.distributed_traces
WHERE time > now() - INTERVAL '1 hour'
LIMIT 100;

-- Traces by service
SELECT service_name, operation_name, duration_ns / 1000000 AS duration_ms
FROM traces.distributed_traces
WHERE service_name = 'api-gateway'
ORDER BY time DESC;

-- Filter by trace_id for debugging
SELECT * FROM traces.distributed_traces
WHERE trace_id = '5b8efff798038103d269b633813fc60c'
ORDER BY time;
```

### Metrics

Each metric is in its own table in the metrics database. All attributes are individual columns for fast querying.

```sql
-- CPU usage with filters
SELECT time, value, "host.name", cpu, state
FROM metrics.system_cpu_usage
WHERE time > now() - INTERVAL '1 hour'
  AND "host.name" = 'macbook-pro'
ORDER BY time DESC;

-- Memory usage
SELECT time, value, "host.name", state
FROM metrics.system_memory_usage
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC;

-- HTTP requests (attributes are columns!)
SELECT
  time,
  value,
  method,
  status,
  service,
  "host.name"
FROM metrics.http_requests_total
WHERE time > now() - INTERVAL '1 hour'
  AND method = 'GET'
  AND status = '200';

-- Aggregate metrics (e.g., requests per minute by service)
SELECT
  time_bucket(INTERVAL '1 minute', time) AS minute,
  service,
  method,
  SUM(value) AS total_requests
FROM metrics.http_requests_total
WHERE time > now() - INTERVAL '1 hour'
GROUP BY minute, service, method
ORDER BY minute, service;
```

### Logs

All attributes are individual columns for fast filtering:

```sql
-- Recent logs with attributes
SELECT time, severity, body, service_name, "host.name", user_id
FROM logs.logs
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC
LIMIT 100;

-- Error logs only (filter by attribute columns)
SELECT time, severity, body, service_name, trace_id, "http.method", "http.status_code"
FROM logs.logs
WHERE severity IN ('ERROR', 'FATAL')
  AND time > now() - INTERVAL '1 hour'
ORDER BY time DESC;

-- Logs for a specific trace (correlation with traces)
SELECT time, severity, body, service_name, user_id
FROM logs.logs
WHERE trace_id = '5b8efff798038103d269b633813fc60c'
ORDER BY time;

-- Count errors by service and host
SELECT
  service_name,
  "host.name",
  COUNT(*) AS error_count
FROM logs.logs
WHERE severity IN ('ERROR', 'FATAL')
  AND time > now() - INTERVAL '1 hour'
GROUP BY service_name, "host.name"
ORDER BY error_count DESC;
```

### Unified Observability: Join Across Signals (Arc's Flagship Feature!)

One of Arc's most powerful capabilities is the ability to **correlate traces, metrics, and logs in a single SQL query**. This is what makes Arc truly unified observability.

#### Example 1: Full Context for Failed Requests

Get traces, error logs, and CPU metrics for failed requests in one query:

```sql
-- Correlate traces with errors and system metrics
SELECT
  t.time,
  t.trace_id,
  t.service_name,
  t.operation_name,
  t.duration_ns / 1000000 AS duration_ms,
  t.status_code,
  t."http.method",
  t."http.status_code",
  l.severity,
  l.body AS error_message,
  cpu.value AS cpu_usage
FROM traces.distributed_traces t
LEFT JOIN logs.logs l
  ON t.trace_id = l.trace_id
LEFT JOIN metrics.system_cpu_usage cpu
  ON t.service_name = cpu.service_name
  AND time_bucket(INTERVAL '1 minute', t.time) = time_bucket(INTERVAL '1 minute', cpu.time)
WHERE t.status_code >= 2  -- OTel status code: 2 = Error
  AND t.time > now() - INTERVAL '1 hour'
ORDER BY t.time DESC
LIMIT 100;
```

This query shows:
- **Traces**: Which requests failed and how long they took
- **Logs**: Error messages associated with those traces
- **Metrics**: CPU usage at the time of failure

#### Example 2: Service Health Dashboard

Complete service health in one query:

```sql
-- Service health: error rate, latency, and resource usage
WITH trace_stats AS (
  SELECT
    time_bucket(INTERVAL '5 minutes', time) AS bucket,
    service_name,
    COUNT(*) AS request_count,
    AVG(duration_ns / 1000000) AS avg_latency_ms,
    SUM(CASE WHEN status_code >= 2 THEN 1 ELSE 0 END) AS error_count
  FROM traces.distributed_traces
  WHERE time > now() - INTERVAL '1 hour'
  GROUP BY bucket, service_name
),
error_logs AS (
  SELECT
    time_bucket(INTERVAL '5 minutes', time) AS bucket,
    service_name,
    COUNT(*) AS log_error_count
  FROM logs.logs
  WHERE severity IN ('ERROR', 'FATAL')
    AND time > now() - INTERVAL '1 hour'
  GROUP BY bucket, service_name
),
cpu_stats AS (
  SELECT
    time_bucket(INTERVAL '5 minutes', time) AS bucket,
    service_name,
    AVG(value) AS avg_cpu
  FROM metrics.system_cpu_usage
  WHERE time > now() - INTERVAL '1 hour'
  GROUP BY bucket, service_name
)
SELECT
  ts.bucket AS time,
  ts.service_name,
  ts.request_count,
  ROUND(ts.avg_latency_ms, 2) AS avg_latency_ms,
  ts.error_count,
  ROUND((ts.error_count::float / NULLIF(ts.request_count, 0) * 100), 2) AS error_rate_pct,
  el.log_error_count,
  ROUND(cs.avg_cpu, 2) AS avg_cpu_usage
FROM trace_stats ts
LEFT JOIN error_logs el ON ts.bucket = el.bucket AND ts.service_name = el.service_name
LEFT JOIN cpu_stats cs ON ts.bucket = cs.bucket AND ts.service_name = cs.service_name
ORDER BY ts.bucket DESC, ts.service_name;
```

This query provides:
- Request volume and latency (from traces)
- Error rate (from traces)
- Error log count (from logs)
- CPU usage (from metrics)

All in one SQL query, all from one database!

#### Example 3: Debug a Specific Incident

Investigate a production incident with complete context:

```sql
-- Complete timeline for a slow request
SELECT
  t.time,
  'trace' AS signal_type,
  t.operation_name AS event,
  t.duration_ns / 1000000 AS duration_ms,
  t.status_code,
  NULL AS severity,
  NULL AS body
FROM traces.distributed_traces t
WHERE t.trace_id = '5b8efff798038103d269b633813fc60c'

UNION ALL

SELECT
  l.time,
  'log' AS signal_type,
  l.service_name AS event,
  NULL AS duration_ms,
  NULL AS status_code,
  l.severity,
  l.body
FROM logs.logs l
WHERE l.trace_id = '5b8efff798038103d269b633813fc60c'

ORDER BY time;
```

This gives you a **unified timeline** of all traces and logs for a single request!

#### Why This Matters

Traditional observability tools require:
- Jaeger for traces
- Prometheus for metrics
- Loki/Elasticsearch for logs
- **Manual correlation** between 3 separate systems

With Arc + OpenTelemetry:
- ✅ All signals in one database
- ✅ Join across traces, metrics, and logs
- ✅ Use SQL for powerful analysis
- ✅ No manual correlation needed
- ✅ Single query for complete context

**This is the future of observability.**

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
