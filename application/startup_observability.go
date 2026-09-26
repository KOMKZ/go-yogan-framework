// 本文件统一 HTTP 应用启动阶段的结构化计时字段。
// 消费方依赖日志字段而非内部函数；新增阶段时应保持字段名稳定并同步框架文档。
package application

import (
	"time"

	"go.uber.org/zap"
)

const startupPhaseLogMessage = "HTTP startup phase completed"

func (a *Application) logStartupPhase(phase string, started time.Time, fields ...zap.Field) {
	phaseFields := []zap.Field{
		zap.String("startup_phase", phase),
		zap.Int64("phase_duration_ms", time.Since(started).Milliseconds()),
		zap.Int64("startup_elapsed_ms", a.GetStartupTimeMs()),
	}
	a.MustGetLogger().InfoCtx(a.ctx, startupPhaseLogMessage, append(phaseFields, fields...)...)
}

func (a *Application) logStartupCheckpoint(phase string, fields ...zap.Field) {
	checkpointFields := []zap.Field{
		zap.String("startup_phase", phase),
		zap.Int64("startup_elapsed_ms", a.GetStartupTimeMs()),
	}
	a.MustGetLogger().InfoCtx(a.ctx, startupPhaseLogMessage, append(checkpointFields, fields...)...)
}
