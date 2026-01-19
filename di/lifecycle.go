// Package di 提供依赖注入和生命周期管理
package di

import (
	"context"

	"github.com/KOMKZ/go-yogan-framework/database"
	"github.com/KOMKZ/go-yogan-framework/kafka"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/KOMKZ/go-yogan-framework/redis"
	"github.com/samber/do/v2"
)

// StartCoreComponents 触发核心组件初始化并注册便捷访问实例
// 组件的 Init/Start 逻辑在各自的 Provider 中实现
// 这里只负责：1. 触发懒加载 2. 注册便捷实例
func StartCoreComponents(ctx context.Context, injector *do.RootScope, log *logger.CtxZapLogger) error {
	// Database - 触发初始化，注册 *gorm.DB 便捷访问
	if dbMgr, err := do.Invoke[*database.Manager](injector); err == nil && dbMgr != nil {
		if db := dbMgr.DB("master"); db != nil {
			do.ProvideValue(injector, db)
		}
		log.DebugCtx(ctx, "✅ Database 组件已就绪")
	}

	// Redis - 触发初始化，注册 Client 便捷访问
	if redisMgr, err := do.Invoke[*redis.Manager](injector); err == nil && redisMgr != nil {
		if client := redisMgr.Client("main"); client != nil {
			do.ProvideValue(injector, client)
		}
		log.DebugCtx(ctx, "✅ Redis 组件已就绪")
	}

	// Kafka - 触发初始化（Manager 已在 Provider 中完成连接）
	if _, err := do.Invoke[*kafka.Manager](injector); err == nil {
		log.DebugCtx(ctx, "✅ Kafka 组件已就绪")
	}

	return nil
}
