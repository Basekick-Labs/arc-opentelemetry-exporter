package arcexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/exporter"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

type metricsExporter struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

func newMetricsExporter(config *Config, set exporter.CreateSettings) *metricsExporter {
	return &metricsExporter{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: set.Logger,
	}
}

func (e *metricsExporter) pushMetrics(ctx context.Context, md pmetric.Metrics) error {
	// Group metrics by name (each metric name becomes a separate measurement/table)
	metricGroups := make(map[string]*metricBatch)

	// Iterate through resource metrics
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		// Extract resource attributes (host.name, service.name, etc.)
		resourceAttrs := attributesToMap(rm.Resource().Attributes())

		// Iterate through scope metrics
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			// Iterate through metrics
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)
				metricName := sanitizeMetricName(metric.Name())

				// Get or create batch for this metric name
				batch, ok := metricGroups[metricName]
				if !ok {
					batch = &metricBatch{
						name:   metricName,
						times:  []int64{},
						values: []float64{},
						labels: []map[string]interface{}{},
					}
					metricGroups[metricName] = batch
				}

				// Process based on metric type (pass resource attributes)
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					e.processGauge(metric, batch, resourceAttrs)
				case pmetric.MetricTypeSum:
					e.processSum(metric, batch, resourceAttrs)
				case pmetric.MetricTypeHistogram:
					e.processHistogram(metric, batch, resourceAttrs)
				case pmetric.MetricTypeSummary:
					e.processSummary(metric, batch, resourceAttrs)
				}
			}
		}
	}

	// Send each metric group as a separate payload
	for metricName, batch := range metricGroups {
		payload, err := e.batchToColumnar(metricName, batch)
		if err != nil {
			return fmt.Errorf("failed to convert metric %s: %w", metricName, err)
		}

		if err := e.sendToArc(ctx, payload); err != nil {
			return fmt.Errorf("failed to send metric %s: %w", metricName, err)
		}
	}

	return nil
}

type metricBatch struct {
	name   string
	times  []int64
	values []float64
	labels []map[string]interface{}
}

func (e *metricsExporter) batchToColumnar(metricName string, batch *metricBatch) ([]byte, error) {
	// Dynamically extract all unique label keys from the batch
	labelKeys := make(map[string]bool)
	for _, labels := range batch.labels {
		for key := range labels {
			labelKeys[key] = true
		}
	}

	// Create columns map with time and value
	columns := map[string]interface{}{
		"time":  batch.times,
		"value": batch.values,
	}

	// For each unique label key, create a column with values
	for labelKey := range labelKeys {
		columnValues := make([]interface{}, len(batch.labels))
		for i, labels := range batch.labels {
			if val, ok := labels[labelKey]; ok {
				columnValues[i] = val
			} else {
				// Use nil for missing values (Arc will handle nulls)
				columnValues[i] = nil
			}
		}
		columns[labelKey] = columnValues
	}

	// Create columnar payload - Telegraf style with explicit columns
	columnarData := map[string]interface{}{
		"m":       metricName,
		"columns": columns,
	}

	// Serialize to msgpack
	msgpackData, err := msgpack.Marshal(columnarData)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal msgpack: %w", err)
	}

	// Compress with gzip
	var buf bytes.Buffer
	gzipWriter := gzip.NewWriter(&buf)
	if _, err := gzipWriter.Write(msgpackData); err != nil {
		return nil, fmt.Errorf("failed to compress: %w", err)
	}
	if err := gzipWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close gzip writer: %w", err)
	}

	return buf.Bytes(), nil
}

func (e *metricsExporter) processGauge(metric pmetric.Metric, batch *metricBatch, resourceAttrs map[string]interface{}) {
	gauge := metric.Gauge()
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
		batch.values = append(batch.values, getNumberValue(dp))

		// Merge resource attributes with data point attributes
		labels := mergeAttributes(resourceAttrs, attributesToMap(dp.Attributes()))
		batch.labels = append(batch.labels, labels)
	}
}

func (e *metricsExporter) processSum(metric pmetric.Metric, batch *metricBatch, resourceAttrs map[string]interface{}) {
	sum := metric.Sum()
	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)
		batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
		batch.values = append(batch.values, getNumberValue(dp))

		// Merge resource attributes with data point attributes
		labels := mergeAttributes(resourceAttrs, attributesToMap(dp.Attributes()))

		// Only include internal metadata if explicitly requested
		if e.config.IncludeMetricMetadata {
			labels["_monotonic"] = sum.IsMonotonic()
			labels["_aggregation_temporality"] = sum.AggregationTemporality().String()
		}

		batch.labels = append(batch.labels, labels)
	}
}

func (e *metricsExporter) processHistogram(metric pmetric.Metric, batch *metricBatch, resourceAttrs map[string]interface{}) {
	histogram := metric.Histogram()
	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dp := histogram.DataPoints().At(i)
		attrs := mergeAttributes(resourceAttrs, attributesToMap(dp.Attributes()))

		// Store histogram as multiple data points with different labels
		// Count
		countLabels := copyMap(attrs)
		if e.config.IncludeMetricMetadata {
			countLabels["_histogram_field"] = "count"
		} else {
			countLabels["histogram_field"] = "count"
		}
		batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
		batch.values = append(batch.values, float64(dp.Count()))
		batch.labels = append(batch.labels, countLabels)

		// Sum
		sumLabels := copyMap(attrs)
		if e.config.IncludeMetricMetadata {
			sumLabels["_histogram_field"] = "sum"
		} else {
			sumLabels["histogram_field"] = "sum"
		}
		batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
		batch.values = append(batch.values, dp.Sum())
		batch.labels = append(batch.labels, sumLabels)

		// Min (if available)
		if dp.HasMin() {
			minLabels := copyMap(attrs)
			if e.config.IncludeMetricMetadata {
				minLabels["_histogram_field"] = "min"
			} else {
				minLabels["histogram_field"] = "min"
			}
			batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
			batch.values = append(batch.values, dp.Min())
			batch.labels = append(batch.labels, minLabels)
		}

		// Max (if available)
		if dp.HasMax() {
			maxLabels := copyMap(attrs)
			if e.config.IncludeMetricMetadata {
				maxLabels["_histogram_field"] = "max"
			} else {
				maxLabels["histogram_field"] = "max"
			}
			batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
			batch.values = append(batch.values, dp.Max())
			batch.labels = append(batch.labels, maxLabels)
		}

		// Buckets
		for j := 0; j < dp.BucketCounts().Len(); j++ {
			bucketLabels := copyMap(attrs)
			if e.config.IncludeMetricMetadata {
				bucketLabels["_histogram_field"] = "bucket"
			} else {
				bucketLabels["histogram_field"] = "bucket"
			}
			if j < dp.ExplicitBounds().Len() {
				bucketLabels["le"] = dp.ExplicitBounds().At(j)
			} else {
				bucketLabels["le"] = "+Inf"
			}

			batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
			batch.values = append(batch.values, float64(dp.BucketCounts().At(j)))
			batch.labels = append(batch.labels, bucketLabels)
		}
	}
}

func (e *metricsExporter) processSummary(metric pmetric.Metric, batch *metricBatch, resourceAttrs map[string]interface{}) {
	summary := metric.Summary()
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)
		attrs := mergeAttributes(resourceAttrs, attributesToMap(dp.Attributes()))

		// Count
		countLabels := copyMap(attrs)
		if e.config.IncludeMetricMetadata {
			countLabels["_summary_field"] = "count"
		} else {
			countLabels["summary_field"] = "count"
		}
		batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
		batch.values = append(batch.values, float64(dp.Count()))
		batch.labels = append(batch.labels, countLabels)

		// Sum
		sumLabels := copyMap(attrs)
		if e.config.IncludeMetricMetadata {
			sumLabels["_summary_field"] = "sum"
		} else {
			sumLabels["summary_field"] = "sum"
		}
		batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
		batch.values = append(batch.values, dp.Sum())
		batch.labels = append(batch.labels, sumLabels)

		// Quantiles
		for j := 0; j < dp.QuantileValues().Len(); j++ {
			qv := dp.QuantileValues().At(j)
			quantileLabels := copyMap(attrs)
			if e.config.IncludeMetricMetadata {
				quantileLabels["_summary_field"] = "quantile"
			} else {
				quantileLabels["summary_field"] = "quantile"
			}
			quantileLabels["quantile"] = qv.Quantile()

			batch.times = append(batch.times, dp.Timestamp().AsTime().UnixMilli())
			batch.values = append(batch.values, qv.Value())
			batch.labels = append(batch.labels, quantileLabels)
		}
	}
}

func (e *metricsExporter) sendToArc(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/api/v1/write/msgpack", e.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/msgpack")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-Arc-Database", e.config.MetricsDatabase)

	if e.config.AuthToken != "" {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", e.config.AuthToken))
	}

	resp, err := e.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("arc returned status %d: %s", resp.StatusCode, string(body))
	}

	e.logger.Debug("Successfully sent metrics to Arc",
		zap.Int("status", resp.StatusCode),
		zap.Int("payload_size", len(payload)))

	return nil
}

func getNumberValue(dp pmetric.NumberDataPoint) float64 {
	switch dp.ValueType() {
	case pmetric.NumberDataPointValueTypeDouble:
		return dp.DoubleValue()
	case pmetric.NumberDataPointValueTypeInt:
		return float64(dp.IntValue())
	default:
		return 0
	}
}

// sanitizeMetricName converts OTel metric names to Arc-friendly table names
// e.g., "system.cpu.usage" -> "system_cpu_usage"
func sanitizeMetricName(name string) string {
	// Replace dots with underscores
	name = strings.ReplaceAll(name, ".", "_")
	// Replace dashes with underscores
	name = strings.ReplaceAll(name, "-", "_")
	// Remove any other special characters
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' {
			return r
		}
		return '_'
	}, name)
	return name
}

// copyMap creates a shallow copy of a map
func copyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// mergeAttributes merges resource attributes with data point attributes
// Data point attributes take precedence over resource attributes
func mergeAttributes(resourceAttrs, dataPointAttrs map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(resourceAttrs)+len(dataPointAttrs))

	// First copy resource attributes
	for k, v := range resourceAttrs {
		result[k] = v
	}

	// Then override with data point attributes
	for k, v := range dataPointAttrs {
		result[k] = v
	}

	return result
}
