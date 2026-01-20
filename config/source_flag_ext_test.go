package config

import (
	"reflect"
	"testing"
)

// TestConvertValue test type conversion function
func TestConvertValue(t *testing.T) {
	tests := []struct {
		name       string
		value      interface{}
		targetKind reflect.Kind
		expected   interface{}
		expectErr  bool
	}{
		// String conversion
		{"int to string", 123, reflect.String, "123", false},
		{"string to string", "hello", reflect.String, "hello", false},

		// Integer conversion
		{"int to int", 123, reflect.Int, 123, false},
		{"int64 to int", int64(456), reflect.Int, int64(456), false},
		{"string to int", "789", reflect.Int, int64(789), false},
		{"invalid string to int", "abc", reflect.Int, nil, true},
		{"float to int", 3.14, reflect.Int, nil, true},

		// Boolean conversion
		{"bool to bool", true, reflect.Bool, true, false},
		{"string true to bool", "true", reflect.Bool, true, false},
		{"string false to bool", "false", reflect.Bool, false, false},
		{"invalid string to bool", "maybe", reflect.Bool, nil, true},
		{"int to bool", 1, reflect.Bool, nil, true},

		// Other types
		{"any to any", 123, reflect.Float32, 123, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ConvertValue(tt.value, tt.targetKind)

			if tt.expectErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if result != tt.expected {
				t.Errorf("result = %v, want %v", result, tt.expected)
			}
		})
	}
}

// TestIsZeroValue test for zero value judgment
func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		// String
		{"empty string", "", true},
		{"non-empty string", "hello", false},

		// Int variants
		{"zero int", int(0), true},
		{"non-zero int", int(1), false},
		{"zero int8", int8(0), true},
		{"non-zero int8", int8(1), false},
		{"zero int16", int16(0), true},
		{"non-zero int16", int16(1), false},
		{"zero int32", int32(0), true},
		{"non-zero int32", int32(1), false},
		{"zero int64", int64(0), true},
		{"non-zero int64", int64(1), false},

		// Uint variants
		{"zero uint", uint(0), true},
		{"non-zero uint", uint(1), false},
		{"zero uint8", uint8(0), true},
		{"non-zero uint8", uint8(1), false},
		{"zero uint16", uint16(0), true},
		{"non-zero uint16", uint16(1), false},
		{"zero uint32", uint32(0), true},
		{"non-zero uint32", uint32(1), false},
		{"zero uint64", uint64(0), true},
		{"non-zero uint64", uint64(1), false},

		// Float variants
		{"zero float32", float32(0), true},
		{"non-zero float32", float32(1.5), false},
		{"zero float64", float64(0), true},
		{"non-zero float64", float64(1.5), false},

		// Bool
		{"false bool", false, true},
		{"true bool", true, false},

		// Pointer
		{"nil pointer", (*int)(nil), true},
		{"non-nil pointer", new(int), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.value)
			result := isZeroValue(v)

			if result != tt.expected {
				t.Errorf("isZeroValue(%v) = %v, want %v", tt.value, result, tt.expected)
			}
		})
	}
}

// TestInterfaceZeroValue_Check:test interface type zero value check
func TestIsZeroValue_Interface(t *testing.T) {
	var nilInterface interface{} = nil

	v := reflect.ValueOf(&nilInterface).Elem()
	if !isZeroValue(v) {
		t.Error("isZeroValue(nil interface) = false, want true")
	}
}

// TestFlagSource_NilFlags test nil flags
func TestFlagSource_NilFlags(t *testing.T) {
	source := NewFlagSource(nil, "grpc", 100)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty map for nil flags, got %v", data)
	}
}

// TestFlagSource_NilPointer test nil pointer
func TestFlagSource_NilPointer(t *testing.T) {
	var flags *struct{ Port int }

	source := NewFlagSource(flags, "grpc", 100)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if len(data) != 0 {
		t.Errorf("expected empty map for nil pointer, got %v", data)
	}
}

// TestFlagSource_NonStruct test non-struct types
func TestFlagSource_NonStruct(t *testing.T) {
	source := NewFlagSource("not a struct", "grpc", 100)

	_, err := source.Load()
	if err == nil {
		t.Error("expected error for non-struct flags")
	}
}

// TestFlagSource_SkipTagDash test skipping "-" tag
func TestFlagSource_SkipTagDash(t *testing.T) {
	type Flags struct {
		Port   int `config:"-"`
		Name   string
		Active bool `config:"app.active,-"` // Include - Should be skipped
	}

	source := NewFlagSource(&Flags{Port: 8080, Name: "test"}, "grpc", 100)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	// The port should be skipped (tag is "-")
	if _, ok := data["grpc.server.port"]; ok {
		t.Error("Port should be skipped with config:\"-\" tag")
	}
}

// TestFlagSource_DefaultMapping_GRPCPort test GRPCPort mapping
func TestFlagSource_DefaultMapping_GRPCPort(t *testing.T) {
	type Flags struct {
		GRPCPort int
	}

	source := NewFlagSource(&Flags{GRPCPort: 9001}, "grpc", 100)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if data["grpc.server.port"] != 9001 {
		t.Errorf("GRPCPort mapping failed, got %v", data)
	}
}

// TestFlagSource_DefaultMapping_HTTPPort test HTTPPort mapping
func TestFlagSource_DefaultMapping_HTTPPort(t *testing.T) {
	type Flags struct {
		HTTPPort int
	}

	source := NewFlagSource(&Flags{HTTPPort: 8081}, "http", 100)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if data["api_server.port"] != 8081 {
		t.Errorf("HTTPPort mapping failed, got %v", data)
	}
}

// TestFlagSource_DefaultMapping_Addresses_Test_Address_Mapping
func TestFlagSource_DefaultMapping_Addresses(t *testing.T) {
	type Flags struct {
		GRPCAddress string
		HTTPAddress string
	}

	source := NewFlagSource(&Flags{GRPCAddress: "10.0.0.1", HTTPAddress: "10.0.0.2"}, "mixed", 100)

	data, err := source.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}

	if data["grpc.server.address"] != "10.0.0.1" {
		t.Errorf("GRPCAddress mapping failed, got %v", data["grpc.server.address"])
	}
	if data["api_server.host"] != "10.0.0.2" {
		t.Errorf("HTTPAddress mapping failed, got %v", data["api_server.host"])
	}
}
