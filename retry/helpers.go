package retry

import (
	"google.golang.org/grpc/codes"
)

// ============================================================
// 预设配置
// ============================================================

// Presets 预设配置
var (
	// GRPCDefaults gRPC 默认重试配置
	GRPCDefaults = []Option{
		MaxAttempts(3),
		Condition(RetryOnGRPCCodes(
			codes.Unavailable,
			codes.DeadlineExceeded,
			codes.ResourceExhausted,
		)),
		Backoff(ExponentialBackoff(1 * 1000000000)), // 1s (time.Second)
	}
	
	// HTTPDefaults HTTP 默认重试配置
	HTTPDefaults = []Option{
		MaxAttempts(3),
		Condition(RetryOnHTTPStatus(429, 502, 503, 504)),
		Backoff(ExponentialBackoff(1 * 1000000000)), // 1s
	}
	
	// DatabaseDefaults 数据库默认重试配置
	DatabaseDefaults = []Option{
		MaxAttempts(3),
		Condition(RetryOnTemporaryError()),
		Backoff(ExponentialBackoff(100 * 1000000)), // 100ms
	}
)

