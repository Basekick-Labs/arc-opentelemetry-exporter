#!/bin/bash
# Quick start script for Mac monitoring with OpenTelemetry + Arc

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🚀 Starting Mac Monitoring with OpenTelemetry + Arc"
echo ""

# Setup Go environment
if command -v go &> /dev/null; then
    export PATH="$PATH:$(go env GOPATH)/bin"
    GOPATH="$(go env GOPATH)"
elif [ -f "/opt/homebrew/bin/go" ]; then
    export PATH="$PATH:/opt/homebrew/bin"
    export PATH="$PATH:$(/opt/homebrew/bin/go env GOPATH)/bin"
    GOPATH="$(/opt/homebrew/bin/go env GOPATH)"
else
    echo "❌ Go not found. Please install Go first."
    exit 1
fi

BUILDER="${GOPATH}/bin/builder"

# Check if builder is installed
if [ ! -f "$BUILDER" ]; then
    echo "📦 Installing OpenTelemetry Collector Builder..."
    go install go.opentelemetry.io/collector/cmd/builder@v0.91.0
    echo ""
fi

# Build collector if not exists
if [ ! -f "./otelcol-arc/otelcol-arc" ]; then
    echo "🔨 Building custom OpenTelemetry Collector..."
    "$BUILDER" --config builder-config.yaml
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
