# Monitor Your Mac with OpenTelemetry + Arc

This example shows how to monitor your Mac's system metrics (CPU, memory, disk, network) using OpenTelemetry Collector and Arc.

## What You'll Get

System metrics every 10 seconds:
- **CPU**: Usage per core, utilization percentage
- **Memory**: Used, available, cached, utilization
- **Disk**: I/O operations, read/write bytes
- **Filesystem**: Usage, available space, inodes
- **Network**: Packets, bytes, errors per interface
- **Load**: System load average (1m, 5m, 15m)
- **Processes**: Process count

## Quick Start

### 1. Start Arc

```bash
cd /Users/nacho/dev/basekick-labs/arc
docker run -d -p 8000:8000 \
  -e STORAGE_BACKEND=local \
  -v arc-data:/app/data \
  --name arc \
  ghcr.io/basekick-labs/arc:latest
```

Or run from source:
```bash
cd /Users/nacho/dev/basekick-labs/arc
make run
```

### 2. Install OpenTelemetry Collector Builder

```bash
go install go.opentelemetry.io/collector/cmd/builder@v0.91.0
```

### 3. Build Custom Collector

```bash
cd /Users/nacho/dev/basekick-labs/arc-opentelemetry-exporter/examples/mac-monitoring

# Build the collector with Arc exporter and host metrics receiver
builder --config builder-config.yaml
```

This creates `./otelcol-arc/otelcol-arc` binary.

### 4. Run the Collector

```bash
# Set hostname (optional)
export HOSTNAME=$(hostname)

# Run the collector
./otelcol-arc/otelcol-arc --config otel-config.yaml
```

You should see output like:
```
2024-01-15T10:30:00.000Z	info	service/service.go:139	Starting otelcol-arc...
2024-01-15T10:30:00.000Z	info	service/service.go:160	Everything is ready. Begin running and processing data.
```

### 5. Verify Data is Flowing

In another terminal:

```bash
# Check available metrics tables
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SHOW TABLES FROM metrics","format":"json"}'

# Query CPU metrics
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time, value, labels->>'\''cpu'\'' as cpu, labels->>'\''state'\'' as state FROM metrics.system_cpu_time ORDER BY time DESC LIMIT 20","format":"json"}'

# Query memory utilization
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time, value, labels->>'\''state'\'' as state FROM metrics.system_memory_usage ORDER BY time DESC LIMIT 10","format":"json"}'

# Query disk I/O
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time, value, labels->>'\''device'\'' as device, labels->>'\''direction'\'' as direction FROM metrics.system_disk_io ORDER BY time DESC LIMIT 10","format":"json"}'

# Query network traffic
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time, value, labels->>'\''device'\'' as device, labels->>'\''direction'\'' as direction FROM metrics.system_network_io ORDER BY time DESC LIMIT 10","format":"json"}'
```

## Useful Queries

### Overall System Health Dashboard

```bash
# Current CPU usage by core
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT labels->>'\''cpu'\'' as cpu, labels->>'\''state'\'' as state, avg(value) as avg_value FROM metrics.system_cpu_time WHERE time > now() - INTERVAL '\''1 minute'\'' GROUP BY labels->>'\''cpu'\'', labels->>'\''state'\'' ORDER BY cpu, state","format":"json"}'

# Memory usage summary
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT labels->>'\''state'\'' as state, avg(value) / 1024 / 1024 / 1024 as avg_gb FROM metrics.system_memory_usage WHERE time > now() - INTERVAL '\''5 minutes'\'' GROUP BY labels->>'\''state'\''","format":"json"}'

# Disk usage by filesystem
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT labels->>'\''device'\'' as device, labels->>'\''mountpoint'\'' as mountpoint, avg(value) / 1024 / 1024 / 1024 as avg_gb FROM metrics.system_filesystem_usage WHERE time > now() - INTERVAL '\''5 minutes'\'' AND labels->>'\''state'\'' = '\''used'\'' GROUP BY labels->>'\''device'\'', labels->>'\''mountpoint'\''","format":"json"}'

# Network throughput
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time_bucket(INTERVAL '\''1 minute'\'', time) as minute, labels->>'\''device'\'' as device, labels->>'\''direction'\'' as direction, sum(value) / 1024 / 1024 as mb FROM metrics.system_network_io WHERE time > now() - INTERVAL '\''10 minutes'\'' GROUP BY minute, device, direction ORDER BY minute DESC, device, direction","format":"json"}'
```

### Performance Analysis

```bash
# CPU spikes in the last hour
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time, labels->>'\''cpu'\'' as cpu, value FROM metrics.system_cpu_time WHERE time > now() - INTERVAL '\''1 hour'\'' AND labels->>'\''state'\'' = '\''user'\'' AND value > 50 ORDER BY time DESC","format":"json"}'

# Memory pressure events
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time_bucket(INTERVAL '\''5 minutes'\'', time) as period, avg(value) / 1024 / 1024 / 1024 as avg_used_gb, max(value) / 1024 / 1024 / 1024 as max_used_gb FROM metrics.system_memory_usage WHERE time > now() - INTERVAL '\''1 hour'\'' AND labels->>'\''state'\'' = '\''used'\'' GROUP BY period HAVING avg(value) / 1024 / 1024 / 1024 > 8 ORDER BY period DESC","format":"json"}'

# High load average periods
curl -X POST http://localhost:8000/api/v1/query \
  -H "Content-Type: application/json" \
  -d '{"sql":"SELECT time, value as load_avg_1m FROM metrics.system_cpu_load_average_1m WHERE time > now() - INTERVAL '\''1 hour'\'' AND value > 4.0 ORDER BY time DESC","format":"json"}'
```

## Metrics Reference

All metrics collected by the `hostmetrics` receiver:

| Metric Name | Description | Labels |
|-------------|-------------|--------|
| `system_cpu_time` | CPU time by state | `cpu`, `state` (user/system/idle/etc) |
| `system_cpu_utilization` | CPU utilization (0-1) | `cpu`, `state` |
| `system_memory_usage` | Memory usage in bytes | `state` (used/free/cached/etc) |
| `system_memory_utilization` | Memory utilization (0-1) | `state` |
| `system_disk_io` | Disk I/O bytes | `device`, `direction` (read/write) |
| `system_disk_operations` | Disk operations count | `device`, `direction` |
| `system_filesystem_usage` | Filesystem usage in bytes | `device`, `mountpoint`, `state` |
| `system_filesystem_utilization` | Filesystem utilization (0-1) | `device`, `mountpoint` |
| `system_network_io` | Network I/O bytes | `device`, `direction` |
| `system_network_packets` | Network packets | `device`, `direction` |
| `system_network_errors` | Network errors | `device`, `direction` |
| `system_cpu_load_average_1m` | 1-minute load average | - |
| `system_cpu_load_average_5m` | 5-minute load average | - |
| `system_cpu_load_average_15m` | 15-minute load average | - |
| `system_processes_count` | Process count | - |

## Troubleshooting

### Collector not starting?
Check the config syntax:
```bash
./otelcol-arc/otelcol-arc validate --config otel-config.yaml
```

### No data in Arc?
1. Check collector logs for errors
2. Verify Arc is running: `curl http://localhost:8000/health`
3. Enable debug exporter to see what's being collected

### Permission issues on Mac?
Some metrics might require additional permissions. Grant Full Disk Access to Terminal in:
System Preferences → Security & Privacy → Privacy → Full Disk Access

## Run as a Service (Optional)

Create a LaunchAgent to run the collector automatically:

```bash
cat > ~/Library/LaunchAgents/com.arc.otelcol.plist <<'EOF'
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.arc.otelcol</string>
    <key>ProgramArguments</key>
    <array>
        <string>/Users/nacho/dev/basekick-labs/arc-opentelemetry-exporter/examples/mac-monitoring/otelcol-arc/otelcol-arc</string>
        <string>--config</string>
        <string>/Users/nacho/dev/basekick-labs/arc-opentelemetry-exporter/examples/mac-monitoring/otel-config.yaml</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
    <key>KeepAlive</key>
    <true/>
    <key>StandardOutPath</key>
    <string>/tmp/otelcol.log</string>
    <key>StandardErrorPath</key>
    <string>/tmp/otelcol.err</string>
</dict>
</plist>
EOF

# Load the service
launchctl load ~/Library/LaunchAgents/com.arc.otelcol.plist

# Check status
launchctl list | grep otelcol
```

To stop:
```bash
launchctl unload ~/Library/LaunchAgents/com.arc.otelcol.plist
```
