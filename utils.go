package arcexporter

import "io"

// mergeAttributes merges resource attributes with signal-specific attributes
// Signal-specific attributes take precedence over resource attributes
func mergeAttributes(resourceAttrs, signalAttrs map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(resourceAttrs)+len(signalAttrs))

	// First copy resource attributes
	for k, v := range resourceAttrs {
		result[k] = v
	}

	// Then override with signal-specific attributes
	for k, v := range signalAttrs {
		result[k] = v
	}

	return result
}

const maxArcErrorBodyBytes = 4096

// readArcErrorBody bounds untrusted error responses before including them in
// exporter errors. The extra byte lets us mark truncated responses.
func readArcErrorBody(body io.Reader) string {
	limited, err := io.ReadAll(io.LimitReader(body, maxArcErrorBodyBytes+1))
	if err != nil {
		return "<unable to read response body>"
	}
	if len(limited) > maxArcErrorBodyBytes {
		return string(limited[:maxArcErrorBodyBytes]) + "... (truncated)"
	}
	return string(limited)
}
