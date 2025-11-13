#!/bin/bash
# Quick start script for Mac monitoring with OpenTelemetry + Arc

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 Starting Mac Monitoring with OpenTelemetry + Arc"
echo ""

# Check if builder is installed
if ! command -v builder &> /dev/null; then
    echo "📦 Installing OpenTelemetry Collector Builder..."
    go install go.opentelemetry.io/collector/cmd/builder@v0.91.0
fi

# Build collector if not exists
if [ ! -f "./otelcol-arc/otelcol-arc" ]; then
    echo "🔨 Building custom OpenTelemetry Collector..."
    builder --config builder-config.yaml
    echo ""
fi

# Check if Arc is running
if ! curl -s http://localhost:8000/health > /dev/null 2>&1; then
    echo "⚠️  Arc is not running on http://localhost:8000"
    echo ""
    echo "Please start Arc first:"
    echo "  Option 1 - Docker:"
    echo "    docker run -d -p 8000:8000 -e STORAGE_BACKEND=local -v arc-data:/app/data --name arc ghcr.io/basekick-labs/arc:latest"
    echo ""
    echo "  Option 2 - From source:"
    echo "    cd /Users/nacho/dev/basekick-labs/arc && make run"
    echo ""
    exit 1
fi

echo "✅ Arc is running"
echo ""

# Set hostname
export HOSTNAME=$(hostname)

echo "🎯 Starting OpenTelemetry Collector..."
echo "   Hostname: $HOSTNAME"
echo "   Collecting metrics every 10 seconds"
echo ""
echo "Press Ctrl+C to stop"
echo ""

# Run the collector
./otelcol-arc/otelcol-arc --config otel-config.yaml
