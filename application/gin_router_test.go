package application

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestNewManager 测试创建路由管理器
func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.Empty(t, manager.routers)
}

// mockRouterImpl 模拟路由（实现正确的接口）
type mockRouterImpl struct {
	registered bool
}

func (m *mockRouterImpl) Register(engine *gin.Engine, app *Application) {
	m.registered = true
}

// TestManager_Add 测试添加路由
func TestManager_Add(t *testing.T) {
	manager := NewManager()

	router := &mockRouterImpl{}
	result := manager.Add(router)

	assert.Equal(t, manager, result)
	assert.Len(t, manager.routers, 1)
}

// TestManager_AddFunc 测试添加路由函数
func TestManager_AddFunc(t *testing.T) {
	manager := NewManager()

	called := false
	result := manager.AddFunc(func(engine *gin.Engine, app *Application) {
		called = true
	})

	assert.Equal(t, manager, result)
	assert.Len(t, manager.routers, 1)

	// 注册路由
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	manager.Register(engine, nil)

	assert.True(t, called)
}

// TestManager_Register 测试注册路由
func TestManager_Register(t *testing.T) {
	manager := NewManager()

	router1 := &mockRouterImpl{}
	router2 := &mockRouterImpl{}

	manager.Add(router1).Add(router2)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	manager.Register(engine, nil)

	assert.True(t, router1.registered)
	assert.True(t, router2.registered)
}

// TestRouterInterface 测试 Router 接口
func TestRouterInterface(t *testing.T) {
	router := &mockRouterImpl{}

	var r Router = router
	assert.NotNil(t, r)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	r.Register(engine, nil)

	assert.True(t, router.registered)
}

// TestRouterFunc 测试 RouterFunc 类型
func TestRouterFunc(t *testing.T) {
	called := false
	fn := RouterFunc(func(engine *gin.Engine, app *Application) {
		called = true
	})

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	fn.Register(engine, nil)

	assert.True(t, called)
}
