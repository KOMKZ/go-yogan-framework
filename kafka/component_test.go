package kafka

import (
	"context"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/stretchr/testify/assert"
)

func TestComponent_Name(t *testing.T) {
	c := NewComponent()
	assert.Equal(t, component.ComponentKafka, c.Name())
	assert.Equal(t, "kafka", c.Name())
}

func TestComponent_DependsOn(t *testing.T) {
	c := NewComponent()
	deps := c.DependsOn()

	assert.Contains(t, deps, component.ComponentConfig)
	assert.Contains(t, deps, component.ComponentLogger)
	assert.Len(t, deps, 2)
}

func TestNewComponent(t *testing.T) {
	c := NewComponent()
	assert.NotNil(t, c)
	assert.Nil(t, c.manager)
	assert.Nil(t, c.logger)
}

func TestComponent_GetManager_Nil(t *testing.T) {
	c := NewComponent()
	assert.Nil(t, c.GetManager())
}

func TestComponent_GetHealthChecker_NilManager(t *testing.T) {
	c := NewComponent()
	checker := c.GetHealthChecker()
	assert.Nil(t, checker)
}

func TestComponent_Stop_NilManager(t *testing.T) {
	c := NewComponent()

	// 初始化 logger
	c.Init(context.Background(), &mockConfigLoader{})

	err := c.Stop(context.Background())
	assert.NoError(t, err)
}

func TestComponent_Start_NilManager(t *testing.T) {
	c := NewComponent()

	err := c.Start(context.Background())
	assert.NoError(t, err)
}

// mockConfigLoader 模拟配置加载器
type mockConfigLoader struct {
	data   map[string]interface{}
	errKey string
}

func (m *mockConfigLoader) Unmarshal(key string, v interface{}) error {
	if m.errKey == key {
		return assert.AnError
	}
	// 对于 kafka key，不返回错误但也不填充数据
	// 这样可以测试 "未配置" 的场景
	return nil
}

func (m *mockConfigLoader) Get(key string) interface{} {
	if m.data != nil {
		return m.data[key]
	}
	return nil
}

func (m *mockConfigLoader) GetString(key string) string {
	if v, ok := m.data[key].(string); ok {
		return v
	}
	return ""
}

func (m *mockConfigLoader) GetInt(key string) int {
	if v, ok := m.data[key].(int); ok {
		return v
	}
	return 0
}

func (m *mockConfigLoader) GetBool(key string) bool {
	if v, ok := m.data[key].(bool); ok {
		return v
	}
	return false
}

func (m *mockConfigLoader) IsSet(key string) bool {
	if m.data != nil {
		_, ok := m.data[key]
		return ok
	}
	return false
}

func TestComponent_Init_NoBrokers(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoader{}

	err := c.Init(context.Background(), loader)
	assert.NoError(t, err)
	assert.Nil(t, c.GetManager())
}

func TestComponent_Init_UnmarshalError(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoaderWithError{
		err: assert.AnError,
	}

	// 由于配置解析失败会被当作 "未配置" 处理，不会返回错误
	err := c.Init(context.Background(), loader)
	assert.NoError(t, err)
}

// mockConfigLoaderWithError 返回错误的配置加载器
type mockConfigLoaderWithError struct {
	err error
}

func (m *mockConfigLoaderWithError) Unmarshal(key string, v interface{}) error {
	return m.err
}

func (m *mockConfigLoaderWithError) Get(key string) interface{} {
	return nil
}

func (m *mockConfigLoaderWithError) GetString(key string) string {
	return ""
}

func (m *mockConfigLoaderWithError) GetInt(key string) int {
	return 0
}

func (m *mockConfigLoaderWithError) GetBool(key string) bool {
	return false
}

func (m *mockConfigLoaderWithError) IsSet(key string) bool {
	return false
}

// mockConfigLoaderWithKafka 返回 Kafka 配置的加载器
type mockConfigLoaderWithKafka struct {
	brokers []string
}

func (m *mockConfigLoaderWithKafka) Unmarshal(key string, v interface{}) error {
	if key == "kafka" {
		if cfg, ok := v.(*Config); ok {
			cfg.Brokers = m.brokers
			cfg.Producer.Enabled = true
			cfg.Producer.RequiredAcks = 1
		}
	}
	return nil
}

func (m *mockConfigLoaderWithKafka) Get(key string) interface{} {
	return nil
}

func (m *mockConfigLoaderWithKafka) GetString(key string) string {
	return ""
}

func (m *mockConfigLoaderWithKafka) GetInt(key string) int {
	return 0
}

func (m *mockConfigLoaderWithKafka) GetBool(key string) bool {
	return false
}

func (m *mockConfigLoaderWithKafka) IsSet(key string) bool {
	return false
}

func TestComponent_Init_WithBrokers(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoaderWithKafka{
		brokers: []string{"localhost:9092"},
	}

	err := c.Init(context.Background(), loader)
	assert.NoError(t, err)
	assert.NotNil(t, c.GetManager())
}

func TestComponent_GetHealthChecker_WithManager(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoaderWithKafka{
		brokers: []string{"localhost:9092"},
	}

	err := c.Init(context.Background(), loader)
	assert.NoError(t, err)

	checker := c.GetHealthChecker()
	assert.NotNil(t, checker)
	assert.Equal(t, "kafka", checker.Name())
}

func TestComponent_Start_WithManager(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoaderWithKafka{
		brokers: []string{"localhost:9092"},
	}

	err := c.Init(context.Background(), loader)
	assert.NoError(t, err)

	// Start 会尝试连接
	// 如果有真实 Kafka 运行则成功，否则失败
	// 这里只测试方法可调用
	_ = c.Start(context.Background())
}

func TestComponent_Stop_WithManager(t *testing.T) {
	c := NewComponent()
	loader := &mockConfigLoaderWithKafka{
		brokers: []string{"localhost:9092"},
	}

	err := c.Init(context.Background(), loader)
	assert.NoError(t, err)

	// Stop 应该成功
	err = c.Stop(context.Background())
	assert.NoError(t, err)
}

// 测试组件接口实现
func TestComponent_ImplementsInterface(t *testing.T) {
	c := NewComponent()

	// 验证实现了 component.Component 接口
	var _ component.Component = c

	// 验证实现了 component.HealthCheckProvider 接口
	var _ component.HealthCheckProvider = c
}

