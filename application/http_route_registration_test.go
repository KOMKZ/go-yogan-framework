// 本文件验证 HTTP 路由注册期的 DI panic 边界。
// 缺失依赖必须返回可诊断的紧凑错误，非 DI panic 必须继续暴露编程缺陷。
package application

import (
	"errors"
	"testing"

	frameworkdi "github.com/KOMKZ/go-yogan-framework/di"
	"github.com/gin-gonic/gin"
	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
)

type missingRouteDependency struct{}

type missingDependencyRouteRegistrar struct{}

func (missingDependencyRouteRegistrar) RegisterRoutes(_ *gin.Engine, app *Application) {
	do.MustInvoke[*missingRouteDependency](app.GetInjector())
}

type ordinaryPanicRouteRegistrar struct{}

func (ordinaryPanicRouteRegistrar) RegisterRoutes(_ *gin.Engine, _ *Application) {
	panic("route bug")
}

func TestRegisterBusinessRoutesCompactsDIResolutionPanic(t *testing.T) {
	app := New("./testdata", "TEST", nil)
	app.RegisterRoutes(missingDependencyRouteRegistrar{})

	err := app.registerBusinessRoutes(gin.New())
	require.Error(t, err)
	require.ErrorIs(t, err, do.ErrServiceNotFound)
	require.NotContains(t, err.Error(), "available services:")

	var resolutionErr *frameworkdi.ResolutionError
	require.True(t, errors.As(err, &resolutionErr))
	require.Contains(t, resolutionErr.MissingService(), "missingRouteDependency")
}

func TestRegisterBusinessRoutesRethrowsNonDIPanic(t *testing.T) {
	app := New("./testdata", "TEST", nil)
	app.RegisterRoutes(ordinaryPanicRouteRegistrar{})

	require.PanicsWithValue(t, "route bug", func() {
		_ = app.registerBusinessRoutes(gin.New())
	})
}
