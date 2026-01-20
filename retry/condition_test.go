package retry

import (
	"context"
	"errors"
	"net"
	"syscall"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ============================================================
// Basic condition testing
// ============================================================

func TestAlwaysRetry(t *testing.T) {
	cond := AlwaysRetry()
	
	tests := []struct {
		name    string
		err     error
		attempt int
		want    bool
	}{
		{"with error", errors.New("test error"), 1, true},
		{"with error attempt 2", errors.New("test error"), 2, true},
		{"with nil error", nil, 1, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, tt.attempt)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNeverRetry(t *testing.T) {
	cond := NeverRetry()
	
	tests := []struct {
		name    string
		err     error
		attempt int
		want    bool
	}{
		{"with error", errors.New("test error"), 1, false},
		{"with error attempt 2", errors.New("test error"), 2, false},
		{"with nil error", nil, 1, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, tt.attempt)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// Error matching condition test
// ============================================================

var (
	ErrTest1 = errors.New("test error 1")
	ErrTest2 = errors.New("test error 2")
	ErrTest3 = errors.New("test error 3")
)

func TestRetryOnError(t *testing.T) {
	cond := RetryOnError(ErrTest1)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"match target", ErrTest1, true},
		{"wrapped target", errors.Join(ErrTest1, errors.New("other")), true},
		{"not match", ErrTest2, false},
		{"nil error", nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRetryOnErrors(t *testing.T) {
	cond := RetryOnErrors(ErrTest1, ErrTest2)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"match first", ErrTest1, true},
		{"match second", ErrTest2, true},
		{"not match", ErrTest3, false},
		{"nil error", nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// Custom condition test
// ============================================================

func TestRetryOnCondition(t *testing.T) {
	cond := RetryOnCondition(func(err error) bool {
		return err.Error() == "retry me"
	})
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"match condition", errors.New("retry me"), true},
		{"not match", errors.New("don't retry"), false},
		{"nil error", nil, false},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// gRPC conditional test
// ============================================================

func TestRetryOnGRPCCodes(t *testing.T) {
	cond := RetryOnGRPCCodes(codes.Unavailable, codes.DeadlineExceeded)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"match unavailable",
			status.Error(codes.Unavailable, "service unavailable"),
			true,
		},
		{
			"match deadline exceeded",
			status.Error(codes.DeadlineExceeded, "timeout"),
			true,
		},
		{
			"not match",
			status.Error(codes.InvalidArgument, "invalid"),
			false,
		},
		{
			"not grpc error",
			errors.New("normal error"),
			false,
		},
		{
			"nil error",
			nil,
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// HTTP condition test
// ============================================================

// Mock HTTP error
type mockHTTPError struct {
	statusCode int
	message    string
}

func (e *mockHTTPError) Error() string {
	return e.message
}

func (e *mockHTTPError) StatusCode() int {
	return e.statusCode
}

func TestRetryOnHTTPStatus(t *testing.T) {
	cond := RetryOnHTTPStatus(429, 503, 504)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"match 429",
			&mockHTTPError{statusCode: 429, message: "too many requests"},
			true,
		},
		{
			"match 503",
			&mockHTTPError{statusCode: 503, message: "service unavailable"},
			true,
		},
		{
			"not match",
			&mockHTTPError{statusCode: 400, message: "bad request"},
			false,
		},
		{
			"not http error",
			errors.New("normal error"),
			false,
		},
		{
			"nil error",
			nil,
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// Temporary error condition test
// ============================================================

// mockTemporaryError simulate temporary error
type mockTemporaryError struct {
	temporary bool
	timeout   bool
}

func (e *mockTemporaryError) Error() string {
	return "mock temporary error"
}

func (e *mockTemporaryError) Temporary() bool {
	return e.temporary
}

func (e *mockTemporaryError) Timeout() bool {
	return e.timeout
}

func TestRetryOnTemporaryError(t *testing.T) {
	cond := RetryOnTemporaryError()
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"temporary error",
			&mockTemporaryError{temporary: true},
			true,
		},
		{
			"not temporary",
			&mockTemporaryError{temporary: false},
			false,
		},
		{
			"context deadline exceeded",
			context.DeadlineExceeded,
			true,
		},
		{
			"context canceled",
			context.Canceled,
			true,
		},
		{
			"timeout",
			syscall.ETIMEDOUT,
			true,
		},
		{
			"normal error",
			errors.New("normal error"),
			false,
		},
		{
			"nil error",
			nil,
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// mockNetError simulate net.Error
type mockNetError struct {
	temporary bool
	timeout   bool
}

func (e *mockNetError) Error() string {
	return "mock net error"
}

func (e *mockNetError) Temporary() bool {
	return e.temporary
}

func (e *mockNetError) Timeout() bool {
	return e.timeout
}

func TestRetryOnTemporaryError_NetError(t *testing.T) {
	cond := RetryOnTemporaryError()
	
	tests := []struct {
		name string
		err  net.Error
		want bool
	}{
		{
			"net temporary error",
			&mockNetError{temporary: true, timeout: false},
			true,
		},
		{
			"net timeout error",
			&mockNetError{temporary: false, timeout: true},
			true,
		},
		{
			"net both",
			&mockNetError{temporary: true, timeout: true},
			true,
		},
		{
			"net neither",
			&mockNetError{temporary: false, timeout: false},
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// Composite condition test
// ============================================================

func TestAnd(t *testing.T) {
	cond := And(
		RetryOnError(ErrTest1),
		RetryOnCondition(func(err error) bool {
			return err.Error() != "skip"
		}),
	)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"both conditions match",
			ErrTest1,
			true,
		},
		{
			"first match, second not",
			errors.New("skip"),
			false,
		},
		{
			"first not match",
			ErrTest2,
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOr(t *testing.T) {
	cond := Or(
		RetryOnError(ErrTest1),
		RetryOnError(ErrTest2),
	)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"match first",
			ErrTest1,
			true,
		},
		{
			"match second",
			ErrTest2,
			true,
		},
		{
			"match neither",
			ErrTest3,
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestNot(t *testing.T) {
	cond := Not(RetryOnError(ErrTest1))
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"match target (inverted)",
			ErrTest1,
			false,
		},
		{
			"not match target (inverted)",
			ErrTest2,
			true,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// Complex combination testing
// ============================================================

func TestComplexCondition(t *testing.T) {
	// (gRPC Unavailable OR HTTP 503) AND NOT (Canceled)
	cond := And(
		Or(
			RetryOnGRPCCodes(codes.Unavailable),
			RetryOnHTTPStatus(503),
		),
		Not(RetryOnError(context.Canceled)),
	)
	
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{
			"grpc unavailable (match)",
			status.Error(codes.Unavailable, "unavailable"),
			true,
		},
		{
			"http 503 (match)",
			&mockHTTPError{statusCode: 503},
			true,
		},
		{
			"canceled (not match)",
			context.Canceled,
			false,
		},
		{
			"other error (not match)",
			errors.New("other"),
			false,
		},
	}
	
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cond.ShouldRetry(tt.err, 1)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// ============================================================
// Benchmark
// ============================================================

func BenchmarkRetryOnError(b *testing.B) {
	cond := RetryOnError(ErrTest1)
	err := ErrTest1
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cond.ShouldRetry(err, 1)
	}
}

func BenchmarkRetryOnGRPCCodes(b *testing.B) {
	cond := RetryOnGRPCCodes(codes.Unavailable)
	err := status.Error(codes.Unavailable, "unavailable")
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cond.ShouldRetry(err, 1)
	}
}

func BenchmarkRetryOnTemporaryError(b *testing.B) {
	cond := RetryOnTemporaryError()
	err := context.DeadlineExceeded
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cond.ShouldRetry(err, 1)
	}
}

