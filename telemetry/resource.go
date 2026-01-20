package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// createResource creates Resource (service information)
func (m *Manager) createResource(ctx context.Context) (*resource.Resource, error) {
	// Basic attributes
	attrs := []attribute.KeyValue{
		semconv.ServiceName(m.config.ServiceName),
		semconv.ServiceVersion(m.config.ServiceVersion),
	}

	// Add custom resource properties (support nested structure)
	flattenedAttrs := flattenMap(m.config.ResourceAttrs, "")
	for key, value := range flattenedAttrs {
		// Supports environment variable substitution
		expandedValue := os.ExpandEnv(value)
		attrs = append(attrs, attribute.String(key, expandedValue))
	}

	// Create Resource
	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithHost(),         // Automatically add host information
		resource.WithProcess(),      // Automatically add process information
		resource.WithTelemetrySDK(), // Automatically add SDK information
	)
}

// flattenMap flattens nested maps into dot-separated key-value pairs
// For example: {"deployment": {"environment": "test"}} => {"deployment.environment": "test"}
func flattenMap(m map[string]interface{}, prefix string) map[string]string {
	result := make(map[string]string)
	for key, value := range m {
		fullKey := key
		if prefix != "" {
			fullKey = prefix + "." + key
		}

		switch v := value.(type) {
		case string:
			result[fullKey] = v
		case map[string]interface{}:
			// Recursively process nested maps
			nested := flattenMap(v, fullKey)
			for nestedKey, nestedValue := range nested {
				result[nestedKey] = nestedValue
			}
		default:
			// Convert other types to strings
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
