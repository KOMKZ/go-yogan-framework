package cache

import (
	"encoding/json"
)

// JSONSerializer JSON 序列化器
type JSONSerializer struct{}

// NewJSONSerializer 创建 JSON 序列化器
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize 序列化对象为 JSON
func (s *JSONSerializer) Serialize(v any) ([]byte, error) {
	return json.Marshal(v)
}

// Deserialize 反序列化 JSON 为对象
func (s *JSONSerializer) Deserialize(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

// Name 返回序列化器名称
func (s *JSONSerializer) Name() string {
	return "json"
}
