package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// FlagSource 命令行参数数据源
// 通过 struct tag 定义参数到配置 key 的映射
type FlagSource struct {
	flags    interface{} // AppFlags 结构体
	priority int
	appType  string // 应用类型：grpc, http, mixed
}

// NewFlagSource 创建命令行参数数据源
func NewFlagSource(flags interface{}, appType string, priority int) *FlagSource {
	return &FlagSource{
		flags:    flags,
		appType:  appType,
		priority: priority,
	}
}

// Name 数据源名称
func (s *FlagSource) Name() string {
	return "flags"
}

// Priority 优先级
func (s *FlagSource) Priority() int {
	return s.priority
}

// Load 加载命令行参数配置
func (s *FlagSource) Load() (map[string]interface{}, error) {
	result := make(map[string]interface{})

	if s.flags == nil {
		return result, nil
	}

	v := reflect.ValueOf(s.flags)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return result, nil
		}
		v = v.Elem()
	}

	if v.Kind() != reflect.Struct {
		return nil, fmt.Errorf("flags must be a struct or pointer to struct")
	}

	t := v.Type()

	// 遍历所有字段
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// 跳过未导出的字段
		if !field.CanInterface() {
			continue
		}

		// 获取 config tag（定义映射到哪个配置 key）
		configTag := fieldType.Tag.Get("config")
		if configTag == "" {
			// 没有 tag，尝试使用默认映射规则
			s.applyDefaultMapping(fieldType.Name, field, result)
			continue
		}

		// 解析 tag：支持多个映射，用逗号分隔
		// 例如：`config:"grpc.server.port,api_server.port"`
		keys := strings.Split(configTag, ",")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" || key == "-" {
				continue
			}

			// 设置值（如果非零值）
			if !isZeroValue(field) {
				result[key] = field.Interface()
			}
		}
	}

	return result, nil
}

// applyDefaultMapping 应用默认映射规则（基于字段名和应用类型）
func (s *FlagSource) applyDefaultMapping(fieldName string, field reflect.Value, result map[string]interface{}) {
	if isZeroValue(field) {
		return
	}

	value := field.Interface()

	switch fieldName {
	case "Port":
		// 根据应用类型选择配置路径
		switch s.appType {
		case "grpc":
			result["grpc.server.port"] = value
		case "http":
			result["api_server.port"] = value
		case "mixed":
			// 混合模式，同时设置两者
			result["grpc.server.port"] = value
			result["api_server.port"] = value
		}

	case "Address":
		switch s.appType {
		case "grpc":
			result["grpc.server.address"] = value
		case "http":
			result["api_server.host"] = value
		case "mixed":
			result["grpc.server.address"] = value
			result["api_server.host"] = value
		}

	case "GRPCPort":
		result["grpc.server.port"] = value

	case "HTTPPort":
		result["api_server.port"] = value

	case "GRPCAddress":
		result["grpc.server.address"] = value

	case "HTTPAddress":
		result["api_server.host"] = value
	}
}

// isZeroValue 判断是否为零值
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Ptr, reflect.Interface:
		return v.IsNil()
	default:
		return false
	}
}

// ConvertValue 类型转换辅助函数
func ConvertValue(value interface{}, targetKind reflect.Kind) (interface{}, error) {
	switch targetKind {
	case reflect.String:
		return fmt.Sprintf("%v", value), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		switch v := value.(type) {
		case int:
			return v, nil
		case int64:
			return v, nil
		case string:
			return strconv.ParseInt(v, 10, 64)
		default:
			return 0, fmt.Errorf("cannot convert %T to int", value)
		}
	case reflect.Bool:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strconv.ParseBool(v)
		default:
			return false, fmt.Errorf("cannot convert %T to bool", value)
		}
	default:
		return value, nil
	}
}

