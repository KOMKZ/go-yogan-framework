package application

import (
	"github.com/gin-gonic/gin"
)

// Router registration interface (framework kernel)
// Supports modular routing definition, each module can be implemented independently
// 🎯 Optimization: Directly pass the Application (dependency container), no need for separate deps
type Router interface {
	Register(engine *gin.Engine, app *Application)
}

// RouterFunc functional routing registrar (convenience method)
// 🎯 Recommended to use functional registration, no need to define structs
type RouterFunc func(engine *gin.Engine, app *Application)

func (f RouterFunc) Register(engine *gin.Engine, app *Application) {
	f(engine, app)
}

// Manager Router manager (unified registration entry)
type Manager struct {
	routers []Router
}

// Create router manager
func NewManager() *Manager {
	return &Manager{
		routers: make([]Router, 0),
	}
}

// Add route registrar (struct method)
func (m *Manager) Add(routers ...Router) *Manager {
	m.routers = append(m.routers, routers...)
	return m
}

// AddFunc Add functional routing registrar (recommended)
// 🎯 Optimization: Directly pass route functions, no adapter needed
func (m *Manager) AddFunc(fn func(engine *gin.Engine, app *Application)) *Manager {
	m.routers = append(m.routers, RouterFunc(fn))
	return m
}

// Register unified registration for all routes
func (m *Manager) Register(engine *gin.Engine, app *Application) {
	for _, router := range m.routers {
		router.Register(engine, app)
	}
}

