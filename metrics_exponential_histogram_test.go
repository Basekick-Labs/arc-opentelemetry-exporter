package arcexporter

import (
	"compress/gzip"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/vmihailenco/msgpack/v5"
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/pmetric"
	"go.uber.org/zap"
)

func TestPushMetrics_ExponentialHistogram(t *testing.T) {
	var received map[string]interface{}
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if r.Method != http.MethodPost {
			t.Errorf("request method = %q, want POST", r.Method)
		}
		if got := r.Header.Get("X-Arc-Database"); got != "metrics" {
			t.Errorf("database header = %q, want metrics", got)
		}
		reader, err := gzip.NewReader(r.Body)
		if err != nil {
			t.Errorf("open gzip body: %v", err)
			return
		}
		defer func() { _ = reader.Close() }()
		payload, err := io.ReadAll(reader)
		if err != nil {
			t.Errorf("read gzip body: %v", err)
			return
		}
		if err := msgpack.Unmarshal(payload, &received); err != nil {
			t.Errorf("decode MessagePack payload: %v", err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	rm.Resource().Attributes().PutStr("service.name", "checkout")
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("request.duration")
	exponential := metric.SetEmptyExponentialHistogram()
	exponential.SetAggregationTemporality(pmetric.AggregationTemporalityDelta)
	dp := exponential.DataPoints().AppendEmpty()
	dp.SetStartTimestamp(pcommon.Timestamp(1_700_000_000_000_000_000))
	dp.SetTimestamp(pcommon.Timestamp(1_700_000_001_000_000_000))
	dp.SetCount(10)
	dp.SetSum(2.5)
	dp.SetMin(0.01)
	dp.SetMax(1.5)
	dp.SetScale(3)
	dp.SetZeroCount(1)
	dp.SetZeroThreshold(0.001)
	dp.Attributes().PutStr("route", "/orders")
	dp.Positive().SetOffset(2)
	dp.Positive().BucketCounts().FromRaw([]uint64{2, 3})
	dp.Negative().SetOffset(-1)
	dp.Negative().BucketCounts().FromRaw([]uint64{4})

	e := &metricsExporter{
		config: &Config{Endpoint: server.URL, MetricsDatabase: "metrics"},
		client: server.Client(),
		logger: zap.NewNop(),
	}
	if err := e.pushMetrics(context.Background(), md); err != nil {
		t.Fatalf("pushMetrics returned error: %v", err)
	}
	if requests != 1 {
		t.Fatalf("Arc requests = %d, want 1 (exponential histogram must not be dropped)", requests)
	}
	if got := received["m"]; got != "request_duration" {
		t.Fatalf("measurement = %v, want request_duration", got)
	}

	columns, ok := received["columns"].(map[string]interface{})
	if !ok {
		t.Fatalf("columns has type %T, want map[string]interface{}", received["columns"])
	}
	fields, ok := columns["exponential_histogram_field"].([]interface{})
	if !ok {
		t.Fatalf("exponential_histogram_field has type %T", columns["exponential_histogram_field"])
	}
	if len(fields) != 8 { // count, sum, min, max, zero_count, and three buckets
		t.Fatalf("exponential histogram rows = %d, want 8", len(fields))
	}
	for _, key := range []string{"scale", "zero_threshold", "aggregation_temporality", "start_time", "bucket_sign", "bucket_index", "service.name", "route"} {
		if _, ok := columns[key]; !ok {
			t.Errorf("missing retained exponential histogram column %q", key)
		}
	}
}

func TestProcessExponentialHistogram_OmitsAbsentOptionalStatistics(t *testing.T) {
	metric := pmetric.NewMetric()
	metric.SetName("request.duration")
	exponential := metric.SetEmptyExponentialHistogram()
	dp := exponential.DataPoints().AppendEmpty()
	dp.SetTimestamp(pcommon.Timestamp(1_700_000_001_000_000_000))
	dp.SetCount(2)

	e := &metricsExporter{config: &Config{}}
	batch := &metricBatch{}
	e.processExponentialHistogram(metric, batch, nil)

	if len(batch.values) != 2 { // count and zero_count; no synthetic sum/min/max
		t.Fatalf("rows = %d, want 2", len(batch.values))
	}
	fieldKey := "exponential_histogram_field"
	if got := batch.labels[0][fieldKey]; got != "count" {
		t.Errorf("first field = %v, want count", got)
	}
	if got := batch.labels[1][fieldKey]; got != "zero_count" {
		t.Errorf("second field = %v, want zero_count", got)
	}
}

func TestPushMetrics_UnsupportedMetricTypeReturnsError(t *testing.T) {
	md := pmetric.NewMetrics()
	rm := md.ResourceMetrics().AppendEmpty()
	sm := rm.ScopeMetrics().AppendEmpty()
	metric := sm.Metrics().AppendEmpty()
	metric.SetName("future.metric")
	// A newly introduced/empty metric type must fail explicitly rather than
	// produce an empty write or silently disappear.

	e := &metricsExporter{config: &Config{}}
	err := e.pushMetrics(context.Background(), md)
	if err == nil {
		t.Fatal("expected unsupported metric type error")
	}
	if !strings.Contains(err.Error(), "unsupported OpenTelemetry metric type") {
		t.Fatalf("unexpected error: %v", err)
	}
}
