package httpclient

import (
	"testing"
	"time"
	
	"github.com/KOMKZ/go-yogan-framework/retry"
)

// ============================================================
// Option 测试
// ============================================================

func TestWithBaseURL(t *testing.T) {
	cfg := newConfig()
	WithBaseURL("https://api.example.com")(cfg)
	
	if cfg.baseURL != "https://api.example.com" {
		t.Error("base URL not set")
	}
}

func TestWithTimeout(t *testing.T) {
	cfg := newConfig()
	WithTimeout(10 * time.Second)(cfg)
	
	if cfg.timeout != 10*time.Second {
		t.Error("timeout not set")
	}
}

func TestWithHeader(t *testing.T) {
	cfg := newConfig()
	WithHeader("Authorization", "Bearer token")(cfg)
	
	if cfg.headers["Authorization"] != "Bearer token" {
		t.Error("header not set")
	}
}

func TestWithHeaders(t *testing.T) {
	cfg := newConfig()
	headers := map[string]string{
		"Authorization": "Bearer token",
		"X-Custom":      "value",
	}
	WithHeaders(headers)(cfg)
	
	if cfg.headers["Authorization"] != "Bearer token" {
		t.Error("Authorization not set")
	}
	if cfg.headers["X-Custom"] != "value" {
		t.Error("X-Custom not set")
	}
}

func TestWithQuery(t *testing.T) {
	cfg := newConfig()
	WithQuery("page", "1")(cfg)
	
	if cfg.queries.Get("page") != "1" {
		t.Error("query not set")
	}
}

func TestWithRetry(t *testing.T) {
	cfg := newConfig()
	WithRetry(retry.MaxAttempts(3))(cfg)
	
	if !cfg.retryEnabled {
		t.Error("retry not enabled")
	}
	if len(cfg.retryOpts) != 1 {
		t.Error("retry opts not set")
	}
}

func TestWithRetryDefaults(t *testing.T) {
	cfg := newConfig()
	WithRetryDefaults()(cfg)
	
	if !cfg.retryEnabled {
		t.Error("retry not enabled")
	}
	if len(cfg.retryOpts) == 0 {
		t.Error("retry defaults not set")
	}
}

func TestDisableRetry(t *testing.T) {
	cfg := newConfig()
	cfg.retryEnabled = true // 先启用
	DisableRetry()(cfg)
	
	if cfg.retryEnabled {
		t.Error("retry should be disabled")
	}
}

func TestWithInsecureSkipVerify(t *testing.T) {
	cfg := newConfig()
	WithInsecureSkipVerify()(cfg)
	
	if cfg.transport == nil {
		t.Error("transport should be created")
	}
	if cfg.transport.TLSClientConfig == nil {
		t.Error("TLS config should be created")
	}
	if !cfg.transport.TLSClientConfig.InsecureSkipVerify {
		t.Error("InsecureSkipVerify should be true")
	}
}

// ============================================================
// config.merge 测试
// ============================================================

func TestConfig_merge_BaseURL(t *testing.T) {
	base := newConfig()
	base.baseURL = "https://base.example.com"
	
	other := newConfig()
	other.baseURL = ""
	
	merged := base.merge(other)
	if merged.baseURL != "https://base.example.com" {
		t.Error("base URL should be inherited")
	}
}

func TestConfig_merge_Headers(t *testing.T) {
	base := newConfig()
	base.headers["X-Base"] = "base"
	
	other := newConfig()
	other.headers["X-Other"] = "other"
	other.headers["X-Base"] = "override" // 覆盖
	
	merged := base.merge(other)
	if merged.headers["X-Base"] != "override" {
		t.Error("header should be overridden")
	}
	if merged.headers["X-Other"] != "other" {
		t.Error("other header should be merged")
	}
}

func TestConfig_merge_Timeout(t *testing.T) {
	base := newConfig()
	base.timeout = 10 * time.Second
	
	other := newConfig()
	other.timeout = 5 * time.Second
	
	merged := base.merge(other)
	if merged.timeout != 5*time.Second {
		t.Error("timeout should be overridden")
	}
}

func TestConfig_merge_Retry(t *testing.T) {
	base := newConfig()
	base.retryEnabled = true
	base.retryOpts = []retry.Option{retry.MaxAttempts(3)}
	
	other := newConfig()
	other.retryEnabled = false // 禁用
	
	merged := base.merge(other)
	if merged.retryEnabled {
		t.Error("retry should be disabled")
	}
}

func TestConfig_merge_RetryOverride(t *testing.T) {
	base := newConfig()
	base.retryEnabled = true
	base.retryOpts = []retry.Option{retry.MaxAttempts(3)}
	
	other := newConfig()
	other.retryEnabled = true
	other.retryOpts = []retry.Option{retry.MaxAttempts(5)}
	
	merged := base.merge(other)
	if !merged.retryEnabled {
		t.Error("retry should be enabled")
	}
	if len(merged.retryOpts) != 1 {
		t.Error("retry opts should be overridden")
	}
}

// ============================================================
// applyOptions 测试
// ============================================================

func TestApplyOptions(t *testing.T) {
	cfg := newConfig()
	opts := []Option{
		WithBaseURL("https://api.example.com"),
		WithTimeout(5 * time.Second),
		WithHeader("X-Test", "value"),
	}
	
	applyOptions(cfg, opts)
	
	if cfg.baseURL != "https://api.example.com" {
		t.Error("baseURL not applied")
	}
	if cfg.timeout != 5*time.Second {
		t.Error("timeout not applied")
	}
	if cfg.headers["X-Test"] != "value" {
		t.Error("header not applied")
	}
}

func TestApplyOptions_NilOption(t *testing.T) {
	cfg := newConfig()
	opts := []Option{
		WithBaseURL("https://api.example.com"),
		nil, // nil option 应该被跳过
		WithTimeout(5 * time.Second),
	}
	
	// 不应该 panic
	applyOptions(cfg, opts)
	
	if cfg.baseURL != "https://api.example.com" {
		t.Error("baseURL not applied")
	}
	if cfg.timeout != 5*time.Second {
		t.Error("timeout not applied")
	}
}

// ============================================================
// newConfig 测试
// ============================================================

func TestNewConfig(t *testing.T) {
	cfg := newConfig()
	
	if cfg == nil {
		t.Fatal("newConfig() should not return nil")
	}
	
	if cfg.timeout != 30*time.Second {
		t.Errorf("default timeout should be 30s, got %v", cfg.timeout)
	}
	
	if cfg.headers == nil {
		t.Error("headers should be initialized")
	}
	
	if cfg.queries == nil {
		t.Error("queries should be initialized")
	}
	
	if cfg.retryEnabled {
		t.Error("retry should be disabled by default")
	}
}

