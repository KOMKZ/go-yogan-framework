package swagger

import (
	"testing"

	"github.com/KOMKZ/go-yogan-framework/config"
	"github.com/KOMKZ/go-yogan-framework/logger"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/assert"
)

func TestSetup_ManagerNotFound(t *testing.T) {
	gin.SetMode(gin.TestMode)

	injector := do.New()
	engine := gin.New()

	err := Setup(injector, engine)
	// Manager 未注册时应返回错误
	assert.Error(t, err)
}

func TestSetup_ManagerNil(t *testing.T) {
	gin.SetMode(gin.TestMode)

	injector := do.New()

	// 注册一个返回 nil 的 Provider
	do.Provide(injector, func(i do.Injector) (*Manager, error) {
		return nil, nil
	})

	engine := gin.New()
	err := Setup(injector, engine)
	assert.NoError(t, err)

	// 未启用时不应注册路由
	assert.Empty(t, engine.Routes())
}

func TestSetup_ManagerEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	injector := do.New()

	// 注册依赖
	do.Provide(injector, func(i do.Injector) (*config.Loader, error) {
		return nil, nil
	})
	do.Provide(injector, func(i do.Injector) (*logger.CtxZapLogger, error) {
		return logger.GetLogger("test"), nil
	})

	// 注册启用的 Manager
	do.Provide(injector, func(i do.Injector) (*Manager, error) {
		cfg := Config{
			Enabled:  true,
			UIPath:   "/swagger/*any",
			SpecPath: "/openapi.json",
		}
		return NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test")), nil
	})

	engine := gin.New()
	err := Setup(injector, engine)
	assert.NoError(t, err)

	// 应注册路由
	assert.Len(t, engine.Routes(), 2)
}

func TestMustSetup_Panic(t *testing.T) {
	gin.SetMode(gin.TestMode)

	injector := do.New()
	engine := gin.New()

	assert.Panics(t, func() {
		MustSetup(injector, engine)
	})
}

func TestSetupWithInfo_ManagerEnabled(t *testing.T) {
	gin.SetMode(gin.TestMode)

	injector := do.New()

	// 注册启用的 Manager
	do.Provide(injector, func(i do.Injector) (*Manager, error) {
		cfg := Config{
			Enabled:  true,
			UIPath:   "/swagger/*any",
			SpecPath: "/openapi.json",
		}
		return NewManager(cfg, DefaultSwaggerInfo(), logger.GetLogger("test")), nil
	})

	engine := gin.New()
	err := SetupWithInfo(injector, engine)
	assert.NoError(t, err)

	// 应注册路由
	assert.Len(t, engine.Routes(), 2)
}
