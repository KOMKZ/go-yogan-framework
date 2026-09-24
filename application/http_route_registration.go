// 本文件负责 HTTP 业务路由的启动期注册边界。
// 上游由 Application.initializeHTTPServer 调用，下游执行应用 RouterRegistrar。
// 只将 samber/do 缺失依赖 panic 转为紧凑启动错误；其他 panic 仍原样抛出，
// 避免框架掩盖路由注册代码缺陷。
package application

import (
	"errors"
	"fmt"

	frameworkdi "github.com/KOMKZ/go-yogan-framework/di"
	"github.com/gin-gonic/gin"
)

func (a *Application) registerBusinessRoutes(engine *gin.Engine) (err error) {
	defer func() {
		recovered := recover()
		if recovered == nil {
			return
		}

		recoveredErr, ok := recovered.(error)
		if !ok {
			panic(recovered)
		}
		normalized := a.normalizeDIResolutionError(recoveredErr)
		var resolutionErr *frameworkdi.ResolutionError
		if !errors.As(normalized, &resolutionErr) {
			panic(recovered)
		}
		err = fmt.Errorf("register routes failed: %w", normalized)
	}()

	a.routerRegistrar.RegisterRoutes(engine, a)
	return nil
}
