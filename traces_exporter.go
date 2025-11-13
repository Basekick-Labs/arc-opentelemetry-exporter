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
	"go.opentelemetry.io/collector/pdata/pcommon"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"go.uber.org/zap"
)

type tracesExporter struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

func newTracesExporter(config *Config, set exporter.CreateSettings) *tracesExporter {
	return &tracesExporter{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: set.Logger,
	}
}

func (e *tracesExporter) pushTraces(ctx context.Context, td ptrace.Traces) error {
	// Convert OTel traces to Arc columnar format
	payload, err := e.tracesToColumnar(td)
	if err != nil {
		return fmt.Errorf("failed to convert traces: %w", err)
	}

	// Send to Arc
	return e.sendToArc(ctx, payload)
}

func (e *tracesExporter) tracesToColumnar(td ptrace.Traces) ([]byte, error) {
	// Columnar arrays
	times := []int64{}
	traceIDs := []string{}
	spanIDs := []string{}
	parentSpanIDs := []string{}
	serviceNames := []string{}
	operationNames := []string{}
	spanKinds := []string{}
	durationsNs := []int64{}
	statusCodes := []int32{}
	statusMessages := []string{}
	attributes := []map[string]interface{}{}

	// Iterate through resource spans
	for i := 0; i < td.ResourceSpans().Len(); i++ {
		rs := td.ResourceSpans().At(i)

		// Get service name from resource attributes
		serviceName := ""
		if serviceAttr, ok := rs.Resource().Attributes().Get("service.name"); ok {
			serviceName = serviceAttr.Str()
		}

		// Iterate through scope spans
		for j := 0; j < rs.ScopeSpans().Len(); j++ {
			ss := rs.ScopeSpans().At(j)

			// Iterate through spans
			for k := 0; k < ss.Spans().Len(); k++ {
				span := ss.Spans().At(k)

				// Time (Arc expects milliseconds)
				times = append(times, span.StartTimestamp().AsTime().UnixMilli())

				// IDs
				traceIDs = append(traceIDs, span.TraceID().String())
				spanIDs = append(spanIDs, span.SpanID().String())

				parentSpanID := ""
				if !span.ParentSpanID().IsEmpty() {
					parentSpanID = span.ParentSpanID().String()
				}
				parentSpanIDs = append(parentSpanIDs, parentSpanID)

				// Service and operation
				serviceNames = append(serviceNames, serviceName)
				operationNames = append(operationNames, span.Name())

				// Span kind
				spanKind := spanKindToString(span.Kind())
				spanKinds = append(spanKinds, spanKind)

				// Duration in nanoseconds
				duration := span.EndTimestamp().AsTime().Sub(span.StartTimestamp().AsTime()).Nanoseconds()
				durationsNs = append(durationsNs, duration)

				// Status
				statusCodes = append(statusCodes, int32(span.Status().Code()))
				statusMessages = append(statusMessages, span.Status().Message())

				// Attributes
				attrs := attributesToMap(span.Attributes())
				attributes = append(attributes, attrs)
			}
		}
	}

	// Create columnar payload
	columnarData := map[string]interface{}{
		"m": e.config.TracesMeasurement,
		"columns": map[string]interface{}{
			"time":              times,
			"trace_id":          traceIDs,
			"span_id":           spanIDs,
			"parent_span_id":    parentSpanIDs,
			"service_name":      serviceNames,
			"operation_name":    operationNames,
			"span_kind":         spanKinds,
			"duration_ns":       durationsNs,
			"status_code":       statusCodes,
			"status_message":    statusMessages,
			"attributes":        attributes,
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

func (e *tracesExporter) sendToArc(ctx context.Context, payload []byte) error {
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

	e.logger.Debug("Successfully sent traces to Arc",
		zap.Int("status", resp.StatusCode),
		zap.Int("payload_size", len(payload)))

	return nil
}

func spanKindToString(kind ptrace.SpanKind) string {
	switch kind {
	case ptrace.SpanKindServer:
		return "server"
	case ptrace.SpanKindClient:
		return "client"
	case ptrace.SpanKindProducer:
		return "producer"
	case ptrace.SpanKindConsumer:
		return "consumer"
	case ptrace.SpanKindInternal:
		return "internal"
	default:
		return "unspecified"
	}
}

func attributesToMap(attrs pcommon.Map) map[string]interface{} {
	result := make(map[string]interface{})
	attrs.Range(func(k string, v pcommon.Value) bool {
		result[k] = valueToInterface(v)
		return true
	})
	return result
}

func valueToInterface(v pcommon.Value) interface{} {
	switch v.Type() {
	case pcommon.ValueTypeStr:
		return v.Str()
	case pcommon.ValueTypeInt:
		return v.Int()
	case pcommon.ValueTypeDouble:
		return v.Double()
	case pcommon.ValueTypeBool:
		return v.Bool()
	case pcommon.ValueTypeBytes:
		return v.Bytes().AsRaw()
	case pcommon.ValueTypeSlice:
		slice := v.Slice()
		result := make([]interface{}, 0, slice.Len())
		for i := 0; i < slice.Len(); i++ {
			result = append(result, valueToInterface(slice.At(i)))
		}
		return result
	case pcommon.ValueTypeMap:
		return attributesToMap(v.Map())
	default:
		return nil
	}
}
