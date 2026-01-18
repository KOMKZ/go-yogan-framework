package event

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/registry"
)

// Config 事件组件配置
type Config struct {
	Enabled  bool `mapstructure:"enabled"`
	PoolSize int  `mapstructure:"pool_size"`
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Enabled:  true,
		PoolSize: 100,
	}
}

// Component 事件组件
type Component struct {
	dispatcher *dispatcher
	registry   *registry.Registry
	logger     *logger.CtxZapLogger
	config     Config
}

// NewComponent 创建事件组件
func NewComponent() *Component {
	return &Component{}
}

// Name 返回组件名称
func (c *Component) Name() string {
	return component.ComponentEvent
}

// DependsOn 返回依赖的组件
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
	}
}

// SetRegistry 设置 Registry（框架自动调用）
func (c *Component) SetRegistry(r *registry.Registry) {
	c.registry = r
}

// Init 初始化组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	c.logger = logger.GetLogger("yogan")
	c.logger.DebugCtx(ctx, "🔧 事件组件开始初始化...")

	// 加载配置
	c.config = DefaultConfig()
	if err := loader.Unmarshal("event", &c.config); err != nil {
		c.logger.DebugCtx(ctx, "使用默认事件配置")
	}

	if !c.config.Enabled {
		c.logger.InfoCtx(ctx, "⏭️ 事件组件已禁用")
		return nil
	}

	// 创建分发器
	c.dispatcher = NewDispatcher(WithPoolSize(c.config.PoolSize))

	c.logger.InfoCtx(ctx, fmt.Sprintf("✅ 事件组件初始化完成 (pool_size=%d)", c.config.PoolSize))
	return nil
}

// Start 启动组件
func (c *Component) Start(ctx context.Context) error {
	return nil
}

// Stop 停止组件
func (c *Component) Stop(ctx context.Context) error {
	if c.dispatcher != nil {
		c.dispatcher.Close()
		c.logger.InfoCtx(ctx, "✅ 事件组件已停止")
	}
	return nil
}

// GetDispatcher 获取事件分发器
func (c *Component) GetDispatcher() Dispatcher {
	return c.dispatcher
}

// IsEnabled 是否启用
func (c *Component) IsEnabled() bool {
	return c.config.Enabled && c.dispatcher != nil
}


// SetKafkaPublisher 设置 Kafka 发布者
// 调用后，Dispatch 方法可使用 WithKafka() 选项发送事件到 Kafka
func (c *Component) SetKafkaPublisher(publisher KafkaPublisher) {
	if c.dispatcher != nil {
		c.dispatcher.kafkaPublisher = publisher
	}
}
