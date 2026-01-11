package application

import (
	"github.com/gin-gonic/gin"
)

// Router 路由注册器接口（框架内核）
// 支持模块化路由定义，每个模块可以独立实现
// 🎯 优化：直接传递 Application（依赖容器），不需要单独的 deps
type Router interface {
	Register(engine *gin.Engine, app *Application)
}

// RouterFunc 函数式路由注册器（便捷方式）
// 🎯 推荐使用函数式注册，无需定义结构体
type RouterFunc func(engine *gin.Engine, app *Application)

func (f RouterFunc) Register(engine *gin.Engine, app *Application) {
	f(engine, app)
}

// Manager 路由管理器（统一注册入口）
type Manager struct {
	routers []Router
}

// NewManager 创建路由管理器
func NewManager() *Manager {
	return &Manager{
		routers: make([]Router, 0),
	}
}

// Add 添加路由注册器（结构体方式）
func (m *Manager) Add(routers ...Router) *Manager {
	m.routers = append(m.routers, routers...)
	return m
}

// AddFunc 添加函数式路由注册器（推荐）
// 🎯 优化：直接传递路由函数，无需适配器
func (m *Manager) AddFunc(fn func(engine *gin.Engine, app *Application)) *Manager {
	m.routers = append(m.routers, RouterFunc(fn))
	return m
}

// Register 统一注册所有路由
func (m *Manager) Register(engine *gin.Engine, app *Application) {
	for _, router := range m.routers {
		router.Register(engine, app)
	}
}

