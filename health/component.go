package health

import (
	"context"
	"time"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"go.uber.org/zap"
)

const ComponentName = "health"

// Component 健康检查组件
type Component struct {
	aggregator     *Aggregator
	config         Config
	logger         *logger.CtxZapLogger
	healthCheckers []component.HealthChecker // 外部注册的健康检查器
}

// Config 健康检查配置
type Config struct {
	Enabled bool          `mapstructure:"enabled"` // 是否启用健康检查
	Timeout time.Duration `mapstructure:"timeout"` // 检查超时时间
}

// DefaultConfig 默认配置
func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Timeout: 5 * time.Second,
	}
}

// NewComponent 创建健康检查组件
func NewComponent() *Component {
	return &Component{
		logger: logger.GetLogger("yogan"),
	}
}

// Name 组件名称
func (c *Component) Name() string {
	return ComponentName
}

// DependsOn 依赖组件
func (c *Component) DependsOn() []string {
	return []string{
		component.ComponentConfig,
		component.ComponentLogger,
	}
}

// Init 初始化组件
func (c *Component) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 加载配置
	c.config = DefaultConfig()
	if loader.IsSet("health") {
		if err := loader.Unmarshal("health", &c.config); err != nil {
			c.logger.WarnCtx(ctx, "Failed to unmarshal health config, using default", zap.Error(err))
		}
	}

	if !c.config.Enabled {
		c.logger.InfoCtx(ctx, "Health check is disabled")
		return nil
	}

	// 创建聚合器
	c.aggregator = NewAggregator(c.config.Timeout)

	// 添加应用元数据
	c.aggregator.SetMetadata("service", "yogan")
	c.aggregator.SetMetadata("version", "1.0.0")

	c.logger.InfoCtx(ctx, "✅ Health check component initialized",
		zap.Duration("timeout", c.config.Timeout))

	return nil
}

// Start 启动组件
func (c *Component) Start(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	// 注册外部注入的健康检查器
	for _, checker := range c.healthCheckers {
		c.aggregator.Register(checker)
		c.logger.DebugCtx(ctx, "Registered health checker", zap.String("name", checker.Name()))
	}

	c.logger.InfoCtx(ctx, "✅ Health check component started")
	return nil
}

// Stop 停止组件
func (c *Component) Stop(ctx context.Context) error {
	if !c.config.Enabled {
		return nil
	}

	c.logger.InfoCtx(ctx, "✅ Health check component stopped")
	return nil
}

// RegisterHealthChecker 注册健康检查器
// 应用层可以调用此方法注册需要检查的组件
func (c *Component) RegisterHealthChecker(checker component.HealthChecker) {
	if checker != nil {
		c.healthCheckers = append(c.healthCheckers, checker)
	}
}

// GetAggregator 获取聚合器
func (c *Component) GetAggregator() *Aggregator {
	return c.aggregator
}

// IsEnabled 是否启用
func (c *Component) IsEnabled() bool {
	return c.config.Enabled
}

// Check 执行健康检查（便捷方法）
func (c *Component) Check(ctx context.Context) *Response {
	if !c.config.Enabled || c.aggregator == nil {
		return &Response{
			Status:    StatusHealthy,
			Timestamp: time.Now(),
			Checks:    make(map[string]CheckResult),
			Metadata:  map[string]interface{}{"enabled": false},
		}
	}
	return c.aggregator.Check(ctx)
}
