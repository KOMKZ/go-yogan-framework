package cache

import (
	"testing"
)

func TestJSONSerializer_Serialize(t *testing.T) {
	s := NewJSONSerializer()

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{
			name:    "serialize string",
			input:   "hello",
			wantErr: false,
		},
		{
			name:    "serialize int",
			input:   123,
			wantErr: false,
		},
		{
			name:    "serialize struct",
			input:   struct{ Name string }{"test"},
			wantErr: false,
		},
		{
			name:    "serialize map",
			input:   map[string]int{"a": 1, "b": 2},
			wantErr: false,
		},
		{
			name:    "serialize nil",
			input:   nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := s.Serialize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Serialize() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(data) == 0 && tt.input != nil {
				t.Error("Serialize() returned empty data for non-nil input")
			}
		})
	}
}

func TestJSONSerializer_Deserialize(t *testing.T) {
	s := NewJSONSerializer()

	t.Run("deserialize string", func(t *testing.T) {
		data, _ := s.Serialize("hello")
		var result string
		err := s.Deserialize(data, &result)
		if err != nil {
			t.Errorf("Deserialize() error = %v", err)
		}
		if result != "hello" {
			t.Errorf("Deserialize() = %v, want %v", result, "hello")
		}
	})

	t.Run("deserialize struct", func(t *testing.T) {
		type TestStruct struct {
			Name string `json:"name"`
			Age  int    `json:"age"`
		}
		input := TestStruct{Name: "test", Age: 18}
		data, _ := s.Serialize(input)
		var result TestStruct
		err := s.Deserialize(data, &result)
		if err != nil {
			t.Errorf("Deserialize() error = %v", err)
		}
		if result.Name != input.Name || result.Age != input.Age {
			t.Errorf("Deserialize() = %+v, want %+v", result, input)
		}
	})

	t.Run("deserialize invalid json", func(t *testing.T) {
		var result string
		err := s.Deserialize([]byte("invalid json"), &result)
		if err == nil {
			t.Error("Deserialize() expected error for invalid json")
		}
	})
}

func TestJSONSerializer_Name(t *testing.T) {
	s := NewJSONSerializer()
	if s.Name() != "json" {
		t.Errorf("Name() = %v, want %v", s.Name(), "json")
	}
}
