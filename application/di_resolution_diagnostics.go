// 本文件负责应用启动期 DI 缺失依赖错误的规范化与结构化诊断输出。
// 上游入口是 BaseApplication.Setup；下游复用 di.ResolutionError 和框架 logger。
// 主生命周期只调用本方法，完整服务清单仅允许写入 DEBUG 字段，不得进入主错误文本。
package application

import (
	"errors"

	"github.com/KOMKZ/go-yogan-framework/di"
	"go.uber.org/zap"
)

func (b *BaseApplication) normalizeSetupError(err error) error {
	err = di.NormalizeResolutionError(b.injector, err)
	var resolutionErr *di.ResolutionError
	if !errors.As(err, &resolutionErr) {
		return err
	}

	registeredServices := resolutionErr.RegisteredServices()
	b.logger.DebugCtx(b.ctx, "DI dependency resolution diagnostics",
		zap.String("di_missing_service", resolutionErr.MissingService()),
		zap.Strings("di_dependency_path", resolutionErr.DependencyPath()),
		zap.Int("di_registered_count", len(registeredServices)),
		zap.Strings("di_registered_services", registeredServices),
	)
	return err
}
