package cache

import (
	"encoding/json"
)

// JSONSerializer JSON serializer
type JSONSerializer struct{}

// Create JSON serializer
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize object to JSON
func (s *JSONSerializer) Serialize(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Deserialize JSON to object
func (s *JSONSerializer) Deserialize(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// Name Returns the serializer name
func (s *JSONSerializer) Name() string {
	return "json"
}
