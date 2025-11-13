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
	"go.opentelemetry.io/collector/pdata/plog"
	"go.uber.org/zap"
)

type logsExporter struct {
	config *Config
	client *http.Client
	logger *zap.Logger
}

func newLogsExporter(config *Config, set exporter.CreateSettings) *logsExporter {
	return &logsExporter{
		config: config,
		client: &http.Client{
			Timeout: config.Timeout,
		},
		logger: set.Logger,
	}
}

func (e *logsExporter) pushLogs(ctx context.Context, ld plog.Logs) error {
	// Convert OTel logs to Arc columnar format
	payload, err := e.logsToColumnar(ld)
	if err != nil {
		return fmt.Errorf("failed to convert logs: %w", err)
	}

	// Send to Arc
	return e.sendToArc(ctx, payload)
}

func (e *logsExporter) logsToColumnar(ld plog.Logs) ([]byte, error) {
	// Columnar arrays for fixed fields
	times := []int64{}
	severities := []string{}
	severityNumbers := []int32{}
	bodies := []string{}
	traceIDs := []string{}
	spanIDs := []string{}
	traceFlags := []uint32{}
	serviceNames := []string{}

	// Collect all attributes for dynamic columns
	allAttributes := []map[string]interface{}{}

	// Iterate through resource logs
	for i := 0; i < ld.ResourceLogs().Len(); i++ {
		rl := ld.ResourceLogs().At(i)

		// Resource attributes
		resAttrs := attributesToMap(rl.Resource().Attributes())

		// Iterate through scope logs
		for j := 0; j < rl.ScopeLogs().Len(); j++ {
			sl := rl.ScopeLogs().At(j)

			// Iterate through log records
			for k := 0; k < sl.LogRecords().Len(); k++ {
				lr := sl.LogRecords().At(k)

				// Time (Arc expects milliseconds)
				times = append(times, lr.Timestamp().AsTime().UnixMilli())

				// Severity
				severities = append(severities, lr.SeverityText())
				severityNumbers = append(severityNumbers, int32(lr.SeverityNumber()))

				// Body
				body := ""
				switch lr.Body().Type() {
				case 1: // String
					body = lr.Body().Str()
				default:
					body = lr.Body().AsString()
				}
				bodies = append(bodies, body)

				// Trace context
				traceID := ""
				if !lr.TraceID().IsEmpty() {
					traceID = lr.TraceID().String()
				}
				traceIDs = append(traceIDs, traceID)

				spanID := ""
				if !lr.SpanID().IsEmpty() {
					spanID = lr.SpanID().String()
				}
				spanIDs = append(spanIDs, spanID)

				traceFlags = append(traceFlags, uint32(lr.Flags()))

				// Service name from resource attributes
				serviceName := ""
				if sn, ok := resAttrs["service.name"]; ok {
					if snStr, ok := sn.(string); ok {
						serviceName = snStr
					}
				}
				serviceNames = append(serviceNames, serviceName)

				// Merge resource attributes with log attributes
				logAttrs := attributesToMap(lr.Attributes())
				mergedAttrs := mergeAttributes(resAttrs, logAttrs)
				allAttributes = append(allAttributes, mergedAttrs)
			}
		}
	}

	// Dynamically extract all unique attribute keys
	attributeKeys := make(map[string]bool)
	for _, attrs := range allAttributes {
		for key := range attrs {
			// Skip service.name since we already have service_name column
			if key != "service.name" {
				attributeKeys[key] = true
			}
		}
	}

	// Create columns map with fixed fields
	columns := map[string]interface{}{
		"time":            times,
		"severity":        severities,
		"severity_number": severityNumbers,
		"body":            bodies,
		"trace_id":        traceIDs,
		"span_id":         spanIDs,
		"trace_flags":     traceFlags,
		"service_name":    serviceNames,
	}

	// Add dynamic columns for each attribute
	for attrKey := range attributeKeys {
		columnValues := make([]interface{}, len(allAttributes))
		for i, attrs := range allAttributes {
			if val, ok := attrs[attrKey]; ok {
				columnValues[i] = val
			} else {
				columnValues[i] = nil
			}
		}
		columns[attrKey] = columnValues
	}

	// Create columnar payload
	columnarData := map[string]interface{}{
		"m":       e.config.LogsMeasurement,
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

func (e *logsExporter) sendToArc(ctx context.Context, payload []byte) error {
	url := fmt.Sprintf("%s/api/v1/write/msgpack", e.config.Endpoint)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/msgpack")
	req.Header.Set("Content-Encoding", "gzip")
	req.Header.Set("X-Arc-Database", e.config.LogsDatabase)

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

	e.logger.Debug("Successfully sent logs to Arc",
		zap.Int("status", resp.StatusCode),
		zap.Int("payload_size", len(payload)))

	return nil
}

// mergeAttributes merges resource attributes with log attributes
// Log attributes take precedence over resource attributes
func mergeAttributes(resourceAttrs, logAttrs map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(resourceAttrs)+len(logAttrs))

	// First copy resource attributes
	for k, v := range resourceAttrs {
		result[k] = v
	}

	// Then override with log attributes
	for k, v := range logAttrs {
		result[k] = v
	}

	return result
}
