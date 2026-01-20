package config

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

// FlagSource command line argument data source
// Define parameter to configuration key mapping through struct tags
type FlagSource struct {
	flags    interface{} // AppFlags structure
	priority int
	appType  string // Application type: grpc, http, mixed
}

// NewFlagSource creates command line argument data source
func NewFlagSource(flags interface{}, appType string, priority int) *FlagSource {
	return &FlagSource{
		flags:    flags,
		appType:  appType,
		priority: priority,
	}
}

// Data source name
func (s *FlagSource) Name() string {
	return "flags"
}

// Priority
func (s *FlagSource) Priority() int {
	return s.priority
}

// Load command line argument configuration
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

	// Iterate through all fields
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// skip unexported fields
		if !field.CanInterface() {
			continue
		}

		// Get config tag (define which configuration key to map to)
		configTag := fieldType.Tag.Get("config")
		if configTag == "" {
			// No tag, try using default mapping rules
			s.applyDefaultMapping(fieldType.Name, field, result)
			continue
		}

		// Parse tag: Support multiple mappings, separated by commas
		// For example: `config:"grpc.server.port,api_server.port"`
		keys := strings.Split(configTag, ",")
		for _, key := range keys {
			key = strings.TrimSpace(key)
			if key == "" || key == "-" {
				continue
			}

			// Set value (if non-zero)
			if !isZeroValue(field) {
				result[key] = field.Interface()
			}
		}
	}

	return result, nil
}

// Apply default mapping rules (based on field names and application type)
func (s *FlagSource) applyDefaultMapping(fieldName string, field reflect.Value, result map[string]interface{}) {
	if isZeroValue(field) {
		return
	}

	value := field.Interface()

	switch fieldName {
	case "Port":
		// Choose configuration path based on application type
		switch s.appType {
		case "grpc":
			result["grpc.server.port"] = value
		case "http":
			result["api_server.port"] = value
		case "mixed":
			// Hybrid mode, set both simultaneously
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

// checks if it is a zero value
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

// ConvertValue type conversion helper function
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

