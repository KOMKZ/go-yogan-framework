package swagger

import (
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
)

// Setup 快捷方法：从 DI 容器获取 Manager 并注册路由
// 应用层可以在 OnReady 回调中调用此方法
//
// 使用示例：
//
//	import _ "your-app/docs" // 导入 swag init 生成的 docs 包
//
//	func (a *App) onReady(core *application.Application) error {
//	    swagger.Setup(core.GetInjector(), core.GetHTTPServer().GetEngine())
//	    return nil
//	}
func Setup(injector do.Injector, engine *gin.Engine) error {
	mgr, err := do.Invoke[*Manager](injector)
	if err != nil {
		return err
	}
	if mgr == nil {
		return nil // Swagger 未启用
	}

	mgr.RegisterRoutes(engine)
	return nil
}

// SetupWithInfo 快捷方法：设置 SwaggerInfo 并注册路由
// 用于需要动态设置 API 信息的场景
//
// 使用示例：
//
//	import (
//	    _ "your-app/docs"
//	    "your-app/docs"
//	)
//
//	func (a *App) onReady(core *application.Application) error {
//	    // 动态设置 API 信息
//	    docs.SwaggerInfo.Title = a.GetConfig().App.Name + " API"
//	    docs.SwaggerInfo.Version = a.GetVersion()
//	    docs.SwaggerInfo.Host = a.GetConfig().ApiServer.Host
//	    docs.SwaggerInfo.BasePath = "/api/v1"
//
//	    swagger.SetupWithInfo(core.GetInjector(), core.GetHTTPServer().GetEngine())
//	    return nil
//	}
func SetupWithInfo(injector do.Injector, engine *gin.Engine) error {
	mgr, err := do.Invoke[*Manager](injector)
	if err != nil {
		return err
	}
	if mgr == nil {
		return nil
	}

	mgr.SetupInfo()
	mgr.RegisterRoutes(engine)
	return nil
}

// MustSetup 快捷方法：设置 Swagger（失败时 panic）
func MustSetup(injector do.Injector, engine *gin.Engine) {
	if err := Setup(injector, engine); err != nil {
		panic(err)
	}
}
