package application

import "github.com/gin-gonic/gin"

// RouterRegistrar 路由注册接口
// 业务应用实现此接口来注册路由
// 🎯 优化：路由注册时可以直接访问 Application（依赖容器）
type RouterRegistrar interface {
	RegisterRoutes(engine *gin.Engine, app *Application)
}

