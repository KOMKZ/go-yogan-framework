package telemetry

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

// createResource 创建 Resource（服务信息）
func (m *Manager) createResource(ctx context.Context) (*resource.Resource, error) {
	// 基础属性
	attrs := []attribute.KeyValue{
		semconv.ServiceName(m.config.ServiceName),
		semconv.ServiceVersion(m.config.ServiceVersion),
	}

	// 添加自定义资源属性（支持嵌套结构）
	flattenedAttrs := flattenMap(m.config.ResourceAttrs, "")
	for key, value := range flattenedAttrs {
		// 支持环境变量替换
		expandedValue := os.ExpandEnv(value)
		attrs = append(attrs, attribute.String(key, expandedValue))
	}

	// 创建 Resource
	return resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithHost(),         // 自动添加主机信息
		resource.WithProcess(),      // 自动添加进程信息
		resource.WithTelemetrySDK(), // 自动添加 SDK 信息
	)
}

// flattenMap 将嵌套的 map 展平为点号分隔的键值对
// 例如：{"deployment": {"environment": "test"}} => {"deployment.environment": "test"}
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
			// 递归处理嵌套 map
			nested := flattenMap(v, fullKey)
			for nestedKey, nestedValue := range nested {
				result[nestedKey] = nestedValue
			}
		default:
			// 其他类型转为字符串
			result[fullKey] = fmt.Sprintf("%v", v)
		}
	}
	return result
}
