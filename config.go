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

	// Database is the default Arc database name (default: "default")
	// Used as fallback if signal-specific databases are not set
	Database string `mapstructure:"database"`

	// TracesDatabase is the database for traces (optional, defaults to Database)
	TracesDatabase string `mapstructure:"traces_database"`

	// MetricsDatabase is the database for metrics (optional, defaults to Database)
	MetricsDatabase string `mapstructure:"metrics_database"`

	// LogsDatabase is the database for logs (optional, defaults to Database)
	LogsDatabase string `mapstructure:"logs_database"`

	// TracesMeasurement is the measurement name for traces (default: "distributed_traces")
	TracesMeasurement string `mapstructure:"traces_measurement"`

	// LogsMeasurement is the measurement name for logs (default: "logs")
	LogsMeasurement string `mapstructure:"logs_measurement"`

	// IncludeMetricMetadata controls whether to include internal OTel metadata in labels
	// (e.g., _monotonic, _aggregation_temporality). Default: false
	IncludeMetricMetadata bool `mapstructure:"include_metric_metadata"`

	// Note: Metrics do not have a single measurement name. Each metric name becomes
	// its own measurement/table (e.g., "system.cpu.usage" -> "system_cpu_usage" table)
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

	// Signal-specific databases default to Database if not set
	if cfg.TracesDatabase == "" {
		cfg.TracesDatabase = cfg.Database
	}
	if cfg.MetricsDatabase == "" {
		cfg.MetricsDatabase = cfg.Database
	}
	if cfg.LogsDatabase == "" {
		cfg.LogsDatabase = cfg.Database
	}

	if cfg.TracesMeasurement == "" {
		cfg.TracesMeasurement = "distributed_traces"
	}
	if cfg.LogsMeasurement == "" {
		cfg.LogsMeasurement = "logs"
	}

	return nil
}
