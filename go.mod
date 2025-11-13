module github.com/basekick-labs/arc-opentelemetry-exporter

go 1.22

require (
	github.com/vmihailenco/msgpack/v5 v5.4.1
	go.opentelemetry.io/collector/component v0.92.0
	go.opentelemetry.io/collector/config/confighttp v0.92.0
	go.opentelemetry.io/collector/config/configretry v0.92.0
	go.opentelemetry.io/collector/exporter v0.92.0
	go.opentelemetry.io/collector/pdata v1.0.1
	go.uber.org/zap v1.26.0
)
