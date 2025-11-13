#!/bin/bash
# Restart the OpenTelemetry Collector with clean rebuild

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

echo "🔄 Rebuilding OpenTelemetry Collector with latest version..."

# Setup Go environment
export PATH="/opt/homebrew/bin:$PATH"
export GOPATH="$(/opt/homebrew/bin/go env GOPATH)"

# Stop any running collector
pkill -f "otelcol-arc/otelcol-arc" || true

# Clean and rebuild
rm -rf otelcol-arc
"${GOPATH}/bin/builder" --config builder-config.yaml

echo ""
echo "✅ Rebuild complete!"
echo ""

# Set hostname
export HOSTNAME=$(hostname)

echo "🎯 Starting OpenTelemetry Collector..."
echo "   Hostname: $HOSTNAME"
echo ""

# Run the collector
./otelcol-arc/otelcol-arc --config otel-config.yaml
