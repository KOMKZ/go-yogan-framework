// 本文件管理 database.Manager 可用的 GORM 方言工厂。
// MySQL 与 PostgreSQL 是框架内建生产驱动；SQLite 等可选驱动必须由消费方显式注册，
// 避免仅使用 MySQL 的生产二进制被迫编译 CGO 驱动。新增驱动时应提供独立注册包，
// 业务仓库只配置驱动名称，不应直接依赖 registry 内部状态。
package database

import (
	"fmt"
	"strings"
	"sync"

	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// DialectorFactory 根据 DSN 创建 GORM 方言。
type DialectorFactory func(dsn string) gorm.Dialector

var dialectorRegistry = struct {
	sync.RWMutex
	factories map[string]DialectorFactory
}{
	factories: map[string]DialectorFactory{
		"mysql":    mysql.Open,
		"postgres": postgres.Open,
	},
}

// RegisterDialector 注册可选数据库方言。
// 同名驱动不允许覆盖，避免 import 顺序改变生产数据库实现。
func RegisterDialector(name string, factory DialectorFactory) error {
	name = strings.ToLower(strings.TrimSpace(name))
	if name == "" {
		return fmt.Errorf("database dialector name is required")
	}
	if factory == nil {
		return fmt.Errorf("database dialector %q factory is nil", name)
	}

	dialectorRegistry.Lock()
	defer dialectorRegistry.Unlock()
	if _, exists := dialectorRegistry.factories[name]; exists {
		return fmt.Errorf("database dialector %q is already registered", name)
	}
	dialectorRegistry.factories[name] = factory
	return nil
}

func resolveDialector(name string, dsn string) (gorm.Dialector, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	dialectorRegistry.RLock()
	factory, exists := dialectorRegistry.factories[name]
	dialectorRegistry.RUnlock()
	if !exists {
		return nil, fmt.Errorf("unsupported driver: %s", name)
	}
	return factory(dsn), nil
}
