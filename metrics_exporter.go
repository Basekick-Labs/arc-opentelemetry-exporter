package arcexporter

import (
	"bytes"
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"net/http"

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
	// Convert OTel metrics to Arc columnar format
	payload, err := e.metricsToColumnar(md)
	if err != nil {
		return fmt.Errorf("failed to convert metrics: %w", err)
	}

	// Send to Arc
	return e.sendToArc(ctx, payload)
}

func (e *metricsExporter) metricsToColumnar(md pmetric.Metrics) ([]byte, error) {
	// Columnar arrays
	times := []int64{}
	metricNames := []string{}
	metricTypes := []string{}
	metricUnits := []string{}
	values := []float64{}
	labels := []map[string]interface{}{}

	// Iterate through resource metrics
	for i := 0; i < md.ResourceMetrics().Len(); i++ {
		rm := md.ResourceMetrics().At(i)

		// Iterate through scope metrics
		for j := 0; j < rm.ScopeMetrics().Len(); j++ {
			sm := rm.ScopeMetrics().At(j)

			// Iterate through metrics
			for k := 0; k < sm.Metrics().Len(); k++ {
				metric := sm.Metrics().At(k)

				// Process based on metric type
				switch metric.Type() {
				case pmetric.MetricTypeGauge:
					e.processGauge(metric, &times, &metricNames, &metricTypes, &metricUnits, &values, &labels)
				case pmetric.MetricTypeSum:
					e.processSum(metric, &times, &metricNames, &metricTypes, &metricUnits, &values, &labels)
				case pmetric.MetricTypeHistogram:
					e.processHistogram(metric, &times, &metricNames, &metricTypes, &metricUnits, &values, &labels)
				case pmetric.MetricTypeSummary:
					e.processSummary(metric, &times, &metricNames, &metricTypes, &metricUnits, &values, &labels)
				}
			}
		}
	}

	// Create columnar payload
	columnarData := map[string]interface{}{
		"m": e.config.MetricsMeasurement,
		"columns": map[string]interface{}{
			"time":        times,
			"metric_name": metricNames,
			"metric_type": metricTypes,
			"metric_unit": metricUnits,
			"value":       values,
			"labels":      labels,
		},
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

func (e *metricsExporter) processGauge(
	metric pmetric.Metric,
	times *[]int64,
	metricNames *[]string,
	metricTypes *[]string,
	metricUnits *[]string,
	values *[]float64,
	labels *[]map[string]interface{},
) {
	gauge := metric.Gauge()
	for i := 0; i < gauge.DataPoints().Len(); i++ {
		dp := gauge.DataPoints().At(i)
		*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
		*metricNames = append(*metricNames, metric.Name())
		*metricTypes = append(*metricTypes, "gauge")
		*metricUnits = append(*metricUnits, metric.Unit())
		*values = append(*values, getNumberValue(dp))
		*labels = append(*labels, attributesToMap(dp.Attributes()))
	}
}

func (e *metricsExporter) processSum(
	metric pmetric.Metric,
	times *[]int64,
	metricNames *[]string,
	metricTypes *[]string,
	metricUnits *[]string,
	values *[]float64,
	labels *[]map[string]interface{},
) {
	sum := metric.Sum()
	metricType := "counter"
	if !sum.IsMonotonic() {
		metricType = "gauge"
	}

	for i := 0; i < sum.DataPoints().Len(); i++ {
		dp := sum.DataPoints().At(i)
		*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
		*metricNames = append(*metricNames, metric.Name())
		*metricTypes = append(*metricTypes, metricType)
		*metricUnits = append(*metricUnits, metric.Unit())
		*values = append(*values, getNumberValue(dp))
		*labels = append(*labels, attributesToMap(dp.Attributes()))
	}
}

func (e *metricsExporter) processHistogram(
	metric pmetric.Metric,
	times *[]int64,
	metricNames *[]string,
	metricTypes *[]string,
	metricUnits *[]string,
	values *[]float64,
	labels *[]map[string]interface{},
) {
	histogram := metric.Histogram()
	for i := 0; i < histogram.DataPoints().Len(); i++ {
		dp := histogram.DataPoints().At(i)

		// Store histogram summary stats as separate metrics
		attrs := attributesToMap(dp.Attributes())

		// Count
		*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
		*metricNames = append(*metricNames, metric.Name()+"_count")
		*metricTypes = append(*metricTypes, "histogram_count")
		*metricUnits = append(*metricUnits, "")
		*values = append(*values, float64(dp.Count()))
		*labels = append(*labels, attrs)

		// Sum
		*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
		*metricNames = append(*metricNames, metric.Name()+"_sum")
		*metricTypes = append(*metricTypes, "histogram_sum")
		*metricUnits = append(*metricUnits, metric.Unit())
		*values = append(*values, dp.Sum())
		*labels = append(*labels, attrs)

		// Buckets
		for j := 0; j < dp.BucketCounts().Len(); j++ {
			bucketAttrs := make(map[string]interface{})
			for k, v := range attrs {
				bucketAttrs[k] = v
			}
			if j < dp.ExplicitBounds().Len() {
				bucketAttrs["le"] = dp.ExplicitBounds().At(j)
			} else {
				bucketAttrs["le"] = "+Inf"
			}

			*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
			*metricNames = append(*metricNames, metric.Name()+"_bucket")
			*metricTypes = append(*metricTypes, "histogram_bucket")
			*metricUnits = append(*metricUnits, "")
			*values = append(*values, float64(dp.BucketCounts().At(j)))
			*labels = append(*labels, bucketAttrs)
		}
	}
}

func (e *metricsExporter) processSummary(
	metric pmetric.Metric,
	times *[]int64,
	metricNames *[]string,
	metricTypes *[]string,
	metricUnits *[]string,
	values *[]float64,
	labels *[]map[string]interface{},
) {
	summary := metric.Summary()
	for i := 0; i < summary.DataPoints().Len(); i++ {
		dp := summary.DataPoints().At(i)
		attrs := attributesToMap(dp.Attributes())

		// Count
		*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
		*metricNames = append(*metricNames, metric.Name()+"_count")
		*metricTypes = append(*metricTypes, "summary_count")
		*metricUnits = append(*metricUnits, "")
		*values = append(*values, float64(dp.Count()))
		*labels = append(*labels, attrs)

		// Sum
		*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
		*metricNames = append(*metricNames, metric.Name()+"_sum")
		*metricTypes = append(*metricTypes, "summary_sum")
		*metricUnits = append(*metricUnits, metric.Unit())
		*values = append(*values, dp.Sum())
		*labels = append(*labels, attrs)

		// Quantiles
		for j := 0; j < dp.QuantileValues().Len(); j++ {
			qv := dp.QuantileValues().At(j)
			quantileAttrs := make(map[string]interface{})
			for k, v := range attrs {
				quantileAttrs[k] = v
			}
			quantileAttrs["quantile"] = qv.Quantile()

			*times = append(*times, dp.Timestamp().AsTime().UnixMilli())
			*metricNames = append(*metricNames, metric.Name())
			*metricTypes = append(*metricTypes, "summary_quantile")
			*metricUnits = append(*metricUnits, metric.Unit())
			*values = append(*values, qv.Value())
			*labels = append(*labels, quantileAttrs)
		}
	}
}

func (e *metricsExporter) sendToArc(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/api/v1/write/msgpack?database=%s", e.config.Endpoint, e.config.Database)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/msgpack")
	req.Header.Set("Content-Encoding", "gzip")

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
