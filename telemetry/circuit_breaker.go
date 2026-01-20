package telemetry

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"go.opentelemetry.io/otel/sdk/trace"
	"go.uber.org/zap"
)

// CircuitState circuit breaker status
type CircuitState int32

const (
	// StateClosed closed state (operational)
	StateClosed CircuitState = 0
	// StateOpen Open state (circuit breaker tripped)
	StateOpen CircuitState = 1
	// StateHalfOpen Half-open state (attempting recovery)
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

// Circuit Breaker Configuration
type CircuitBreakerConfig struct {
	Enabled              bool          `mapstructure:"enabled"`                 // Whether to enable circuit breaker
	FailureThreshold     int           `mapstructure:"failure_threshold"`       // Failure threshold (number of consecutive failures that trigger circuit breaking)
	SuccessThreshold     int           `mapstructure:"success_threshold"`       // success threshold (how many consecutive successful recoveries in the half-open state)
	Timeout              time.Duration `mapstructure:"timeout"`                 // circuit breaker timeout period (after how long to attempt recovery)
	HalfOpenMaxRequests  int           `mapstructure:"half_open_max_requests"`  // maximum number of requests allowed in half-open state
	FallbackExporterType string        `mapstructure:"fallback_exporter_type"`  // Degraded exporter type (stdout/noop)
}

// Circuit Breaker
type CircuitBreaker struct {
	config           CircuitBreakerConfig
	logger           *zap.Logger
	state            atomic.Int32 // Current status
	failureCount     atomic.Int32 // failure count
	successCount     atomic.Int32 // Successful count (half-open state)
	halfOpenRequests atomic.Int32 // half-open state request count
	lastStateChange  time.Time
	mu               sync.RWMutex

	// Original exporter and fallback exporter
	primaryExporter  trace.SpanExporter
	fallbackExporter trace.SpanExporter
}

// Create circuit breaker
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

// ExportSpans export Spans (wrap raw Exporter)
func (cb *CircuitBreaker) ExportSpans(ctx context.Context, spans []trace.ReadOnlySpan) error {
	if !cb.config.Enabled {
		// Circuit breaker is not enabled, directly use main provider
		return cb.primaryExporter.ExportSpans(ctx, spans)
	}

	currentState := CircuitState(cb.state.Load())

	switch currentState {
	case StateClosed:
		// Closed state: attempt to use master ejector
		err := cb.primaryExporter.ExportSpans(ctx, spans)
		if err != nil {
			cb.onFailure()
			return err
		}
		cb.onSuccess()
		return nil

	case StateOpen:
		// Open state: check if recovery should be attempted
		if cb.shouldAttemptReset() {
			cb.toHalfOpen()
			// Throttle attempt in half-open state
			if !cb.canAttemptRequest() {
				return cb.fallbackExporter.ExportSpans(ctx, spans)
			}
			
			// Try using the master extractor
			err := cb.primaryExporter.ExportSpans(ctx, spans)
			// Do not release count
			if err != nil {
				cb.onFailure()
				return cb.fallbackExporter.ExportSpans(ctx, spans)
			}
			cb.onSuccess()
			return nil
		}
		// Continue circuit breaking, use fallback exporter
		return cb.fallbackExporter.ExportSpans(ctx, spans)

	case StateHalfOpen:
		// Half-open state: rate limiting attempts recovery
		if !cb.canAttemptRequest() {
			// request number limit exceeded for semi-open state, use degraded exporter
			return cb.fallbackExporter.ExportSpans(ctx, spans)
		}

		// Try to use master extractor
		err := cb.primaryExporter.ExportSpans(ctx, spans)
		// Note: The count is not released here because we need to accumulate the success count to decide whether to recover
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

// Shut down exporter
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

// handle success
func (cb *CircuitBreaker) onSuccess() {
	cb.failureCount.Store(0) // Reset failure count

	currentState := CircuitState(cb.state.Load())
	if currentState == StateHalfOpen {
		successCount := cb.successCount.Add(1)
		if int(successCount) >= cb.config.SuccessThreshold {
			cb.toClosed()
		}
	}
}

// handle failure
func (cb *CircuitBreaker) onFailure() {
	failureCount := cb.failureCount.Add(1)

	currentState := CircuitState(cb.state.Load())
	if currentState == StateHalfOpen {
		// any failure immediately opens in half-open state
		cb.toOpen()
		return
	}

	if currentState == StateClosed && int(failureCount) >= cb.config.FailureThreshold {
		cb.toOpen()
	}
}

// switch to closed state
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

// toOpen switch to open state
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

// switch to half-open state
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

// whether a reset attempt should be made
func (cb *CircuitBreaker) shouldAttemptReset() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return time.Since(cb.lastStateChange) >= cb.config.Timeout
}

// canAttemptRequest whether requests are allowed in half-open state
func (cb *CircuitBreaker) canAttemptRequest() bool {
	current := cb.halfOpenRequests.Add(1)
	if int(current) > cb.config.HalfOpenMaxRequests {
		cb.halfOpenRequests.Add(-1) // rollback
		return false
	}
	return true
}

// GetState Get current state
func (cb *CircuitBreaker) GetState() CircuitState {
	return CircuitState(cb.state.Load())
}

// GetStats Retrieve statistics
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

