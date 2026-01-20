package application

import (
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

// TestNewManager test creating route manager
func TestNewManager(t *testing.T) {
	manager := NewManager()
	assert.NotNil(t, manager)
	assert.Empty(t, manager.routers)
}

// mockRouterImpl Mock router (implements correct interface)
type mockRouterImpl struct {
	registered bool
}

func (m *mockRouterImpl) Register(engine *gin.Engine, app *Application) {
	m.registered = true
}

// TestManager_Add test route addition
func TestManager_Add(t *testing.T) {
	manager := NewManager()

	router := &mockRouterImpl{}
	result := manager.Add(router)

	assert.Equal(t, manager, result)
	assert.Len(t, manager.routers, 1)
}

// TestManager_AddFunc test function for adding routes
func TestManager_AddFunc(t *testing.T) {
	manager := NewManager()

	called := false
	result := manager.AddFunc(func(engine *gin.Engine, app *Application) {
		called = true
	})

	assert.Equal(t, manager, result)
	assert.Len(t, manager.routers, 1)

	// Register routes
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	manager.Register(engine, nil)

	assert.True(t, called)
}

// TestManager_Register test registration route
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

// TestRouterInterface test router interface
func TestRouterInterface(t *testing.T) {
	router := &mockRouterImpl{}

	var r Router = router
	assert.NotNil(t, r)

	gin.SetMode(gin.TestMode)
	engine := gin.New()
	r.Register(engine, nil)

	assert.True(t, router.registered)
}

// TestRouterFunc test RouterFunc type
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
