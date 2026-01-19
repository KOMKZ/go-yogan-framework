package application

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/component"
	"github.com/KOMKZ/go-yogan-framework/logger"
)

// LoggerComponent 日志组件（核心组件）
type LoggerComponent struct {
	coreLogger *logger.CtxZapLogger
}

// NewLoggerComponent 创建日志组件
func NewLoggerComponent() *LoggerComponent {
	return &LoggerComponent{}
}

// Name 组件名称
func (l *LoggerComponent) Name() string {
	return component.ComponentLogger
}

// DependsOn 日志组件依赖配置组件
func (l *LoggerComponent) DependsOn() []string {
	return []string{component.ComponentConfig}
}

// Init 初始化日志管理器
func (l *LoggerComponent) Init(ctx context.Context, loader component.ConfigLoader) error {
	// 读取 Logger 配置
	var loggerCfg *logger.ManagerConfig
	if err := loader.Unmarshal("logger", &loggerCfg); err == nil && loggerCfg != nil {
		logger.MustResetManager(*loggerCfg)
	} else {
		logger.InitManager(logger.DefaultManagerConfig())
	}

	// 获取核心日志实例
	l.coreLogger = logger.GetLogger("yogan")

	return nil
}

// Start 启动日志组件（日志无需启动）
func (l *LoggerComponent) Start(ctx context.Context) error {
	return nil
}

// Stop 停止日志组件（关闭所有日志实例）
func (l *LoggerComponent) Stop(ctx context.Context) error {
	if l.coreLogger != nil {
		l.coreLogger.DebugCtx(ctx, "✅ 应用已关闭")
		logger.CloseAll()
	}
	return nil
}

// GetLogger 获取核心日志实例
func (l *LoggerComponent) GetLogger() *logger.CtxZapLogger {
	return l.coreLogger
}

// SetLogger 设置日志实例（用于 DI 模式复用已创建的 Logger）
func (l *LoggerComponent) SetLogger(log *logger.CtxZapLogger) {
	l.coreLogger = log
}
