package arcexporter

import (
	"errors"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/config/configretry"
)

// Config defines configuration for Arc exporter.
type Config struct {
	confighttp.HTTPClientSettings `mapstructure:",squash"`
	configretry.BackOffConfig     `mapstructure:"retry_on_failure"`

	// Endpoint is the Arc API endpoint
	Endpoint string `mapstructure:"endpoint"`

	// AuthToken is the optional authentication token for Arc
	AuthToken string `mapstructure:"auth_token"`

	// Database is the Arc database name (default: "default")
	Database string `mapstructure:"database"`

	// TracesMeasurement is the measurement name for traces (default: "distributed_traces")
	TracesMeasurement string `mapstructure:"traces_measurement"`

	// MetricsMeasurement is the measurement name for metrics (default: "metrics")
	MetricsMeasurement string `mapstructure:"metrics_measurement"`

	// LogsMeasurement is the measurement name for logs (default: "logs")
	LogsMeasurement string `mapstructure:"logs_measurement"`
}

var _ component.Config = (*Config)(nil)

// Validate checks if the exporter configuration is valid
func (cfg *Config) Validate() error {
	if cfg.Endpoint == "" {
		return errors.New("endpoint is required")
	}

	// Set defaults
	if cfg.Database == "" {
		cfg.Database = "default"
	}
	if cfg.TracesMeasurement == "" {
		cfg.TracesMeasurement = "distributed_traces"
	}
	if cfg.MetricsMeasurement == "" {
		cfg.MetricsMeasurement = "metrics"
	}
	if cfg.LogsMeasurement == "" {
		cfg.LogsMeasurement = "logs"
	}

	return nil
}
