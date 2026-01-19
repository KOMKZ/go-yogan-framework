// Package di 提供依赖注入和生命周期管理
package di

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/kafka"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/samber/do/v2"
	"gorm.io/gorm"
)

// StartCoreComponents 启动核心组件（按依赖顺序）
// 触发懒加载并注册便捷访问实例
// 返回错误表示组件启动失败
func StartCoreComponents(ctx context.Context, injector *do.RootScope, log *logger.CtxZapLogger) error {
	// ═══════════════════════════════════════════════════════════
	// Layer 2: 基础设施组件
	// ═══════════════════════════════════════════════════════════

	// Database - 触发连接并注册默认 *gorm.DB
	if dbMgr, err := do.Invoke[*database.Manager](injector); err == nil && dbMgr != nil {
		if db := dbMgr.DB("master"); db != nil {
			do.ProvideValue(injector, db) // *gorm.DB（默认 master）
		}
		log.DebugCtx(ctx, "✅ Database 组件已启动")
	}

	// Redis - 触发连接并注册默认 Client
	if redisMgr, err := do.Invoke[*redis.Manager](injector); err == nil && redisMgr != nil {
		if client := redisMgr.Client("main"); client != nil {
			do.ProvideValue(injector, client) // redis.UniversalClient
		}
		log.DebugCtx(ctx, "✅ Redis 组件已启动")
	}

	// Kafka - 触发连接
	if kafkaMgr, err := do.Invoke[*kafka.Manager](injector); err == nil && kafkaMgr != nil {
		log.DebugCtx(ctx, "✅ Kafka 组件已启动")
	}

	// ═══════════════════════════════════════════════════════════
	// Layer 3: 业务支撑组件（懒加载，使用时才初始化）
	// JWT/Event/Cache/Limiter/Health 按需初始化
	// ═══════════════════════════════════════════════════════════

	return nil
}

// RegisterDefaultInstances 注册便捷访问实例
// 应用层可通过 do.Invoke[*gorm.DB] 直接获取默认数据库
func RegisterDefaultInstances(injector *do.RootScope) {
	// 注册 *gorm.DB 便捷访问（从 Manager 获取 master）
	do.Provide(injector, func(i do.Injector) (*gorm.DB, error) {
		mgr, err := do.Invoke[*database.Manager](i)
		if err != nil || mgr == nil {
			return nil, err
		}
		return mgr.DB("master"), nil
	})
}
