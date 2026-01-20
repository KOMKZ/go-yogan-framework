package retry

import (
	"google.golang.org/grpc/codes"
)

// ============================================================
// Default configuration
// ============================================================

// Preset configurations
var (
	// GRPCDefaults gRPC default retry configuration
	GRPCDefaults = []Option{
		MaxAttempts(3),
		Condition(RetryOnGRPCCodes(
			codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
		)),
		Backoff(ExponentialBackoff(1 * 1000000000)), // 1s (time.Second)
	}
	
	// HTTP default retry configuration
	HTTPDefaults = []Option{
		MaxAttempts(3),
		Condition(RetryOnHTTPStatus(429, 502, 503, 504)),
		Backoff(ExponentialBackoff(1 * 1000000000)), // 1s
	}
	
	// DatabaseDefaults database default retry configuration
	DatabaseDefaults = []Option{
		MaxAttempts(3),
		Condition(RetryOnTemporaryError()),
		Backoff(ExponentialBackoff(100 * 1000000)), // 100ms
	}
)

