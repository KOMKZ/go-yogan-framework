package application

import "github.com/gin-gonic/gin"

// Router Registrar routing registration interface
// Business applications implement this interface to register routes
// 🎯 Optimization: Directly access Application (dependency container) when registering routes
type RouterRegistrar interface {
	RegisterRoutes(engine *gin.Engine, app *Application)
}

