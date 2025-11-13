# Quick Start Guide

Get started with the Arc OpenTelemetry Exporter in 5 minutes!

## Prerequisites

- Docker and Docker Compose installed
- Or OpenTelemetry Collector Builder (ocb) for custom builds

## Option 1: Docker Compose (Easiest)

### 1. Start Arc and OTel Collector

```bash
cd examples
docker-compose up -d
```

This starts:
- **Arc**: Database on port 8000
- **OTel Collector**: Receiving telemetry on ports 4317 (gRPC) and 4318 (HTTP)

### 2. Send Test Data

Send a test trace using curl:

```bash
# Install grpcurl if needed: brew install grpcurl

# Send test trace
grpcurl -plaintext \
  -d '{
    "resource_spans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"string_value": "test-service"}
        }]
      },
      "scope_spans": [{
        "spans": [{
          "trace_id": "5B8EFFF798038103D269B633813FC60C",
          "span_id": "EEE19B7EC3C1B174",
          "name": "test-operation",
          "kind": 1,
          "start_time_unix_nano": 1000000000,
          "end_time_unix_nano": 2000000000
        }]
      }]
    }]
  }' \
  localhost:4317 \
  opentelemetry.proto.collector.trace.v1.TraceService/Export
```

Or send via HTTP:

```bash
curl -X POST http://localhost:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "test-service"}
        }]
      },
      "scopeSpans": [{
        "spans": [{
          "traceId": "5B8EFFF798038103D269B633813FC60C",
          "spanId": "EEE19B7EC3C1B174",
          "name": "test-operation",
          "kind": 1,
          "startTimeUnixNano": "1000000000",
          "endTimeUnixNano": "2000000000"
        }]
      }]
    }]
  }'
```

### 3. Query Data in Arc

```bash
# Check Arc health
curl http://localhost:8000/health

# Query traces (requires Arc token or public access)
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "sql": "SELECT * FROM distributed_traces LIMIT 10"
  }'
```

### 4. Stop Services

```bash
docker-compose down
```

## Option 2: Custom Collector Build

### 1. Install OCB (OpenTelemetry Collector Builder)

```bash
# macOS
brew install opentelemetry-collector-builder

# Or download from releases
# https://github.com/open-telemetry/opentelemetry-collector/releases
```

### 2. Build Custom Collector

```bash
cd examples
ocb --config builder-config.yaml
```

This creates a custom collector binary in `./dist/otelcol-arc`.

### 3. Start Arc

```bash
docker run -d \
  -p 8000:8000 \
  -e STORAGE_BACKEND=local \
  -v arc-data:/app/data \
  ghcr.io/basekick-labs/arc:latest
```

### 4. Run Custom Collector

```bash
./dist/otelcol-arc --config otel-collector-config.yaml
```

## Option 3: Integrate with Existing Application

### Python Application Example

1. **Install OpenTelemetry SDK**

```bash
pip install opentelemetry-api opentelemetry-sdk opentelemetry-exporter-otlp
```

2. **Instrument Your App**

```python
from opentelemetry import trace
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter

# Set up tracing
trace.set_tracer_provider(TracerProvider())
tracer = trace.get_tracer(__name__)

# Configure OTLP exporter pointing to collector
otlp_exporter = OTLPSpanExporter(
    endpoint="localhost:4317",
    insecure=True
)

# Add span processor
trace.get_tracer_provider().add_span_processor(
    BatchSpanProcessor(otlp_exporter)
)

# Create spans
with tracer.start_as_current_span("my-operation"):
    print("Doing work...")
    # Your code here
```

3. **Run Your App**

The app will send traces to the OTel Collector (port 4317), which exports to Arc!

### Node.js Application Example

1. **Install Dependencies**

```bash
npm install @opentelemetry/api \
  @opentelemetry/sdk-node \
  @opentelemetry/auto-instrumentations-node \
  @opentelemetry/exporter-trace-otlp-grpc
```

2. **Create tracing.js**

```javascript
const { NodeSDK } = require('@opentelemetry/sdk-node');
const { getNodeAutoInstrumentations } = require('@opentelemetry/auto-instrumentations-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');

const sdk = new NodeSDK({
  traceExporter: new OTLPTraceExporter({
    url: 'http://localhost:4317',
  }),
  instrumentations: [getNodeAutoInstrumentations()],
});

sdk.start();
```

3. **Run Your App**

```bash
node -r ./tracing.js app.js
```

## Verify Data in Arc

### Using SQL Queries

```sql
-- Query traces
SELECT
  time,
  trace_id,
  span_id,
  service_name,
  operation_name,
  duration_ns / 1000000 AS duration_ms
FROM distributed_traces
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC
LIMIT 100;

-- Query metrics
SELECT
  time,
  metric_name,
  metric_type,
  value
FROM metrics
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC
LIMIT 100;

-- Query logs
SELECT
  time,
  severity,
  body,
  service_name
FROM logs
WHERE time > now() - INTERVAL '1 hour'
ORDER BY time DESC
LIMIT 100;
```

### Using Arc API

```bash
# Query via HTTP
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{
    "sql": "SELECT COUNT(*) as trace_count FROM distributed_traces"
  }'
```

## Troubleshooting

### Collector Not Starting

Check logs:
```bash
docker-compose logs otel-collector
```

Common issues:
- Port 4317/4318 already in use
- Configuration file syntax errors
- Arc endpoint not reachable

### No Data in Arc

1. **Check Collector logs**
   ```bash
   docker-compose logs otel-collector
   ```

2. **Check Arc logs**
   ```bash
   docker-compose logs arc
   ```

3. **Verify connectivity**
   ```bash
   curl http://localhost:8000/health
   ```

4. **Check Arc database**
   ```bash
   curl -X POST http://localhost:8000/api/v1/query \
     -H "Content-Type: application/json" \
     -d '{"sql": "SHOW TABLES"}'
   ```

### Performance Issues

- Increase batch size in collector config
- Add more collector replicas
- Scale Arc instance resources
- Enable compression (gzip)

## Next Steps

- Read the full [README](README.md) for detailed configuration
- Check out [Arc documentation](https://basekick.net/docs)
- Join our [Discord community](https://discord.gg/nxnWfUxsdm)
- Star the project on [GitHub](https://github.com/basekick-labs/arc-opentelemetry-exporter)

## Need Help?

- Open an [issue](https://github.com/basekick-labs/arc-opentelemetry-exporter/issues)
- Ask in [Discord](https://discord.gg/nxnWfUxsdm)
- Check [Arc docs](https://basekick.net/docs)
