package arcexporter

import (
	"context"
	"time"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/exporter/exporterhelper"
)

const (
	// typeStr is the type of the exporter
	typeStr = "arc"

	// defaultTimeout is the default HTTP timeout
	defaultTimeout = 30 * time.Second
)

// NewFactory creates a factory for Arc exporter.
func NewFactory() exporter.Factory {
	return exporter.NewFactory(
		typeStr,
		createDefaultConfig,
		exporter.WithTraces(createTracesExporter, component.StabilityLevelBeta),
		exporter.WithMetrics(createMetricsExporter, component.StabilityLevelBeta),
		exporter.WithLogs(createLogsExporter, component.StabilityLevelBeta),
	)
}

func createDefaultConfig() component.Config {
	return &Config{
		HTTPClientSettings: confighttp.HTTPClientSettings{
			Timeout:     defaultTimeout,
			Compression: "gzip",
		},
		BackOffConfig:     configretry.NewDefaultBackOffConfig(),
		Database:          "default",
		TracesMeasurement: "distributed_traces",
		LogsMeasurement:   "logs",
	}
}

func createTracesExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Traces, error) {
	c := cfg.(*Config)
	exp := newTracesExporter(c, set)

	return exporterhelper.NewTracesExporter(
		ctx,
		set,
		cfg,
		exp.pushTraces,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: c.Timeout}),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(exporterhelper.NewDefaultQueueSettings()),
	)
}

func createMetricsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Metrics, error) {
	c := cfg.(*Config)
	exp := newMetricsExporter(c, set)

	return exporterhelper.NewMetricsExporter(
		ctx,
		set,
		cfg,
		exp.pushMetrics,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: c.Timeout}),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(exporterhelper.NewDefaultQueueSettings()),
	)
}

func createLogsExporter(
	ctx context.Context,
	set exporter.CreateSettings,
	cfg component.Config,
) (exporter.Logs, error) {
	c := cfg.(*Config)
	exp := newLogsExporter(c, set)

	return exporterhelper.NewLogsExporter(
		ctx,
		set,
		cfg,
		exp.pushLogs,
		exporterhelper.WithTimeout(exporterhelper.TimeoutSettings{Timeout: c.Timeout}),
		exporterhelper.WithRetry(c.BackOffConfig),
		exporterhelper.WithQueue(exporterhelper.NewDefaultQueueSettings()),
	)
}
