package kafka

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Component Kafka 组件
//
// 实现 component.Component 接口，提供 Kafka 消息队列能力
// 依赖：config, logger
type Component struct {
	manager *Manager
	logger  *logger.CtxZapLogger
}

// NewComponent 创建 Kafka 组件
func NewComponent() *Component {
	return &Component{}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentKafka
}

// DependsOn Kafka 组件依赖配置和日志组件
func (c *Component) DependsOn() []string {
	return []string{component.ComponentConfig, component.ComponentLogger}
}

// Init 初始化 Kafka 管理器
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	c.logger = logger.GetLogger("yogan")
	c.logger.DebugCtx(ctx, "🔧 Kafka 组件开始初始化...")

	// 读取 Kafka 配置
	var cfg Config
	if err := loader.Unmarshal("kafka", &cfg); err != nil {
		// 如果没有配置，跳过初始化
		c.logger.DebugCtx(ctx, "未配置 Kafka，跳过初始化")
		return nil
	}

	// 如果没有配置 brokers，跳过
	if len(cfg.Brokers) == 0 {
		c.logger.InfoCtx(ctx, "Kafka brokers 未配置，跳过初始化")
		return nil
	}

	// 创建管理器
	manager, err := NewManager(cfg, c.logger.GetZapLogger())
	if err != nil {
		return fmt.Errorf("创建 Kafka 管理器失败: %w", err)
	}

	c.manager = manager
	c.logger.DebugCtx(ctx, "✅ Kafka 管理器创建成功",
		zap.Strings("brokers", cfg.Brokers),
		zap.Bool("producer_enabled", cfg.Producer.Enabled),
		zap.Bool("consumer_enabled", cfg.Consumer.Enabled))

	return nil
}

// Start 启动 Kafka 组件（连接 Kafka）
func (c *Component) Start(ctx context.Context) error {
	if c.manager == nil {
		return nil // 未配置，跳过
	}

	if err := c.manager.Connect(ctx); err != nil {
		return fmt.Errorf("连接 Kafka 失败: %w", err)
	}

	c.logger.InfoCtx(ctx, "✅ Kafka 组件启动完成")
	return nil
}

// Stop 停止 Kafka 组件（关闭连接）
func (c *Component) Stop(ctx context.Context) error {
	if c.manager != nil {
		if err := c.manager.Close(); err != nil {
			return fmt.Errorf("关闭 Kafka 连接失败: %w", err)
		}
	}
	c.logger.InfoCtx(ctx, "✅ Kafka 组件已停止")
	return nil
}

// GetManager 获取 Kafka 管理器
func (c *Component) GetManager() *Manager {
	return c.manager
}

// GetHealthChecker 获取健康检查器
// 实现 component.HealthCheckProvider 接口
func (c *Component) GetHealthChecker() component.HealthChecker {
	if c.manager == nil {
		return nil
	}
	return NewHealthChecker(c.manager)
}

