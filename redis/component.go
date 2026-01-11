package redis

import (
	"context"
	"fmt"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

// Component Redis 组件
//
// 实现 component.Component 接口，提供 Redis 管理能力
// 依赖：config, logger
type Component struct {
	manager *Manager
	logger  *logger.CtxZapLogger // 🎯 组件统一使用字段保存 logger
}

// NewComponent 创建 Redis 组件
func NewComponent() *Component {
	return &Component{}
}

// Name 组件名称
func (c *Component) Name() string {
	return component.ComponentRedis
}

// DependsOn Redis 组件依赖配置和日志组件
func (c *Component) DependsOn() []string {
	return []string{component.ComponentConfig, component.ComponentLogger}
}

// Init 初始化 Redis 管理器
//
// 🎯 简化后的实现：直接从 ConfigLoader 读取配置
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 🎯 统一在 Init 开始时保存 logger 到字段
	c.logger = logger.GetLogger("yogan")
	c.logger.DebugCtx(ctx, "🔧 Redis 组件开始初始化...")

	// 直接从 ConfigLoader 读取 Redis 配置！
	var redisConfigs map[string]Config
	if err := loader.Unmarshal("redis.instances", &redisConfigs); err != nil {
		return fmt.Errorf("读取 Redis 配置失败: %w", err)
	}

	c.logger.DebugCtx(ctx, "✅ 读取配置成功", zap.Int("configs_count", len(redisConfigs)))

	// 如果未配置，跳过初始化
	if len(redisConfigs) == 0 {
		c.logger.DebugCtx(ctx, "未配置 Redis，跳过初始化")
		return nil
	}

	// 创建 Redis 管理器（使用底层 zap.Logger）
	manager, err := NewManager(redisConfigs, c.logger.GetZapLogger())
	if err != nil {
		return fmt.Errorf("创建 Redis 管理器失败: %w", err)
	}

	c.manager = manager
	c.logger.DebugCtx(ctx, "✅ Redis 初始化成功")
	return nil
}

// Start 启动 Redis 组件（Redis 无需启动）
func (c *Component) Start(ctx context.Context) error {
	return nil
}

// Stop 停止 Redis 组件（关闭连接）
func (c *Component) Stop(ctx context.Context) error {
	if c.manager != nil {
		if err := c.manager.Close(); err != nil {
			return fmt.Errorf("关闭 Redis 连接失败: %w", err)
		}
	}
	return nil
}

// GetManager 获取 Redis 管理器
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
