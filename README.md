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

    # Database name (optional, default: "default")
    database: production

    # Measurement/table names (optional)
    traces_measurement: distributed_traces
    metrics_measurement: metrics
    logs_measurement: logs

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
docker run -d \
  -p 8000:8000 \
  -e STORAGE_BACKEND=local \
  -v arc-data:/app/data \
  ghcr.io/basekick-labs/arc:latest
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
```json
{
  "m": "metrics",
  "columns": {
    "time": [1699900000000, ...],
    "metric_name": ["http_requests_total", ...],
    "metric_type": ["counter", ...],
    "value": [42.0, ...],
    "labels": [{"service": "api"}, ...]
  }
}
```

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
