package arcexporter

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
