package telemetry

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// CircuitState 熔断器状态
type CircuitState int32

const (
	// StateClosed 闭合状态（正常工作）
	StateClosed CircuitState = 0
	// StateOpen 打开状态（熔断中）
	StateOpen CircuitState = 1
	// StateHalfOpen 半开状态（尝试恢复）
	StateHalfOpen CircuitState = 2
)

func (s CircuitState) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// CircuitBreakerConfig 熔断器配置
type CircuitBreakerConfig struct {
	Enabled              bool          `mapstructure:"enabled"`                 // 是否启用熔断器
	FailureThreshold     int           `mapstructure:"failure_threshold"`       // 失败阈值（连续失败多少次触发熔断）
	SuccessThreshold     int           `mapstructure:"success_threshold"`       // 成功阈值（半开状态下连续成功多少次恢复）
	Timeout              time.Duration `mapstructure:"timeout"`                 // 熔断超时时间（多久后尝试恢复）
	HalfOpenMaxRequests  int           `mapstructure:"half_open_max_requests"`  // 半开状态允许的最大请求数
	FallbackExporterType string        `mapstructure:"fallback_exporter_type"`  // 降级导出器类型（stdout/noop）
}

// CircuitBreaker 熔断器
type CircuitBreaker struct {
	config           CircuitBreakerConfig
	logger           *zap.Logger
	state            atomic.Int32 // 当前状态
	failureCount     atomic.Int32 // 失败计数
	successCount     atomic.Int32 // 成功计数（半开状态）
	halfOpenRequests atomic.Int32 // 半开状态的请求计数
	lastStateChange  time.Time
	mu               sync.RWMutex

	// 原始导出器和降级导出器
	primaryExporter  trace.SpanExporter
	fallbackExporter trace.SpanExporter
}

// NewCircuitBreaker 创建熔断器
func NewCircuitBreaker(
	config CircuitBreakerConfig,
	logger *zap.Logger,
	primaryExporter trace.SpanExporter,
	fallbackExporter trace.SpanExporter,
) *CircuitBreaker {
	cb := &CircuitBreaker{
		config:           config,
		logger:           logger,
		primaryExporter:  primaryExporter,
		fallbackExporter: fallbackExporter,
		lastStateChange:  time.Now(),
	}
	cb.state.Store(int32(StateClosed))
	return cb
}

// ExportSpans 导出 Spans（包装原始 Exporter）
func (cb *CircuitBreaker) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	if !cb.config.Enabled {
		// 熔断器未启用，直接使用主导出器
		return cb.primaryExporter.ExportSpans(ctx, spans)
	}

	currentState := CircuitState(cb.state.Load())

	switch currentState {
	case StateClosed:
		// 闭合状态：尝试使用主导出器
		err := cb.primaryExporter.ExportSpans(ctx, spans)
		if err != nil {
			cb.onFailure()
			return err
		}
		cb.onSuccess()
		return nil

	case StateOpen:
		// 打开状态：检查是否该尝试恢复
		if cb.shouldAttemptReset() {
			cb.toHalfOpen()
			// 在半开状态限流尝试
			if !cb.canAttemptRequest() {
				return cb.fallbackExporter.ExportSpans(ctx, spans)
			}
			
			// 尝试使用主导出器
			err := cb.primaryExporter.ExportSpans(ctx, spans)
			// 不释放计数
			if err != nil {
				cb.onFailure()
				return cb.fallbackExporter.ExportSpans(ctx, spans)
			}
			cb.onSuccess()
			return nil
		}
		// 继续熔断，使用降级导出器
		return cb.fallbackExporter.ExportSpans(ctx, spans)

	case StateHalfOpen:
		// 半开状态：限流尝试恢复
		if !cb.canAttemptRequest() {
			// 超过半开状态的请求数限制，使用降级导出器
			return cb.fallbackExporter.ExportSpans(ctx, spans)
		}

		// 尝试使用主导出器
		err := cb.primaryExporter.ExportSpans(ctx, spans)
		// 注意：这里不释放计数，因为我们需要累计成功次数来决定是否恢复
		if err != nil {
			cb.onFailure()
			return cb.fallbackExporter.ExportSpans(ctx, spans)
		}
		cb.onSuccess()
		return nil

	default:
		return cb.primaryExporter.ExportSpans(ctx, spans)
	}
}

// Shutdown 关闭导出器
func (cb *CircuitBreaker) Shutdown(ctx context.Context) error {
	var err1, err2 error
	if cb.primaryExporter != nil {
		err1 = cb.primaryExporter.Shutdown(ctx)
	}
	if cb.fallbackExporter != nil {
		err2 = cb.fallbackExporter.Shutdown(ctx)
	}
	if err1 != nil {
		return err1
	}
	return err2
}

// onSuccess 处理成功
func (cb *CircuitBreaker) onSuccess() {
	cb.failureCount.Store(0) // 重置失败计数

	currentState := CircuitState(cb.state.Load())
	if currentState == StateHalfOpen {
		successCount := cb.successCount.Add(1)
		if int(successCount) >= cb.config.SuccessThreshold {
			cb.toClosed()
		}
	}
}

// onFailure 处理失败
func (cb *CircuitBreaker) onFailure() {
	failureCount := cb.failureCount.Add(1)

	currentState := CircuitState(cb.state.Load())
	if currentState == StateHalfOpen {
		// 半开状态任何失败都立即打开
		cb.toOpen()
		return
	}

	if currentState == StateClosed && int(failureCount) >= cb.config.FailureThreshold {
		cb.toOpen()
	}
}

// toClosed 切换到闭合状态
func (cb *CircuitBreaker) toClosed() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := CircuitState(cb.state.Load())
	if oldState == StateClosed {
		return
	}

	cb.state.Store(int32(StateClosed))
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
	cb.halfOpenRequests.Store(0)
	cb.lastStateChange = time.Now()

	cb.logger.Info("🟢 Circuit breaker state changed",
		zap.String("from", oldState.String()),
		zap.String("to", "closed"),
		zap.String("reason", "recovery_successful"),
	)
}

// toOpen 切换到打开状态
func (cb *CircuitBreaker) toOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := CircuitState(cb.state.Load())
	if oldState == StateOpen {
		return
	}

	cb.state.Store(int32(StateOpen))
	cb.successCount.Store(0)
	cb.halfOpenRequests.Store(0)
	cb.lastStateChange = time.Now()

	cb.logger.Warn("🔴 Circuit breaker state changed",
		zap.String("from", oldState.String()),
		zap.String("to", "open"),
		zap.Int32("failure_count", cb.failureCount.Load()),
		zap.Int("failure_threshold", cb.config.FailureThreshold),
		zap.String("fallback_exporter", cb.config.FallbackExporterType),
	)
}

// toHalfOpen 切换到半开状态
func (cb *CircuitBreaker) toHalfOpen() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	oldState := CircuitState(cb.state.Load())
	if oldState == StateHalfOpen {
		return
	}

	cb.state.Store(int32(StateHalfOpen))
	cb.failureCount.Store(0)
	cb.successCount.Store(0)
	cb.halfOpenRequests.Store(0)
	cb.lastStateChange = time.Now()

	cb.logger.Info("🟡 Circuit breaker state changed",
		zap.String("from", oldState.String()),
		zap.String("to", "half-open"),
		zap.String("reason", "attempting_recovery"),
	)
}

// shouldAttemptReset 是否应该尝试恢复
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return time.Since(cb.lastStateChange) >= cb.config.Timeout
}

// canAttemptRequest 半开状态是否允许请求
func (cb *CircuitBreaker) canAttemptRequest() bool {
	current := cb.halfOpenRequests.Add(1)
	if int(current) > cb.config.HalfOpenMaxRequests {
		cb.halfOpenRequests.Add(-1) // 回退
		return false
	}
	return true
}

// GetState 获取当前状态
func (cb *CircuitBreaker) GetState() CircuitState {
	return CircuitState(cb.state.Load())
}

// GetStats 获取统计信息
func (cb *CircuitBreaker) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"state":               cb.GetState().String(),
		"failure_count":       cb.failureCount.Load(),
		"success_count":       cb.successCount.Load(),
		"half_open_requests":  cb.halfOpenRequests.Load(),
		"last_state_change":   cb.lastStateChange.Format(time.RFC3339),
		"time_since_change":   time.Since(cb.lastStateChange).String(),
		"failure_threshold":   cb.config.FailureThreshold,
		"success_threshold":   cb.config.SuccessThreshold,
		"timeout":             cb.config.Timeout.String(),
		"fallback_exporter":   cb.config.FallbackExporterType,
	}
}

