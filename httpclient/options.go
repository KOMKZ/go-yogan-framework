package httpclient

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"
	"net/url"
	"time"
	
	"github.com/KOMKZ/go-yogan-framework/retry"
)

// config 内部配置结构
type config struct {
	// Client 配置
	baseURL    string
	timeout    time.Duration
	transport  *http.Transport
	cookieJar  http.CookieJar
	headers    map[string]string
	
	// Request 配置
	ctx        context.Context
	queries    url.Values
	body       io.Reader
	retryOpts  []retry.Option
	retryEnabled bool
	
	// Breaker 配置
	breakerManager  BreakerManager
	breakerResource string
	breakerFallback func(ctx context.Context, err error) (*Response, error)
	breakerDisabled bool
	
	// 高级配置
	beforeRequest func(*http.Request) error
	afterResponse func(*Response) error
}

// Option 配置选项类型
type Option func(*config)

// ============================================================
// Client 级别选项
// ============================================================

// WithBaseURL 设置基础 URL
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		c.baseURL = baseURL
	}
}

// WithTimeout 设置超时时间
func WithTimeout(duration time.Duration) Option {
	return func(c *config) {
		c.timeout = duration
	}
}

// WithHeader 设置单个 Header
func WithHeader(key, value string) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers[key] = value
	}
}

// WithHeaders 设置多个 Headers
func WithHeaders(headers map[string]string) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		for k, v := range headers {
			c.headers[k] = v
		}
	}
}

// WithTransport 设置自定义 Transport
func WithTransport(transport *http.Transport) Option {
	return func(c *config) {
		c.transport = transport
	}
}

// WithInsecureSkipVerify 跳过 TLS 验证（不安全，仅用于开发环境）
func WithInsecureSkipVerify() Option {
	return func(c *config) {
		if c.transport == nil {
			c.transport = &http.Transport{}
		}
		if c.transport.TLSClientConfig == nil {
			c.transport.TLSClientConfig = &tls.Config{}
		}
		c.transport.TLSClientConfig.InsecureSkipVerify = true
	}
}

// WithCookieJar 设置 Cookie Jar
func WithCookieJar(jar http.CookieJar) Option {
	return func(c *config) {
		c.cookieJar = jar
	}
}

// ============================================================
// Request 级别选项
// ============================================================

// WithContext 设置 Context
func WithContext(ctx context.Context) Option {
	return func(c *config) {
		c.ctx = ctx
	}
}

// WithQuery 设置单个 Query 参数
func WithQuery(key, value string) Option {
	return func(c *config) {
		if c.queries == nil {
			c.queries = make(url.Values)
		}
		c.queries.Set(key, value)
	}
}

// WithQueries 设置多个 Query 参数
func WithQueries(queries url.Values) Option {
	return func(c *config) {
		if c.queries == nil {
			c.queries = make(url.Values)
		}
		for k, vs := range queries {
			for _, v := range vs {
				c.queries.Add(k, v)
			}
		}
	}
}

// WithJSON 设置 JSON Body（会自动设置 Content-Type）
func WithJSON(data interface{}) Option {
	return func(c *config) {
		// JSON 序列化会在 Client.Do() 中处理
		// 这里只是标记，实际序列化在执行时进行
	}
}

// WithForm 设置 Form Body（会自动设置 Content-Type）
func WithForm(data map[string]string) Option {
	return func(c *config) {
		// Form 编码会在 Client.Do() 中处理
	}
}

// WithBody 设置原始 Body
func WithBody(reader io.Reader) Option {
	return func(c *config) {
		c.body = reader
	}
}

// WithBodyString 设置字符串 Body
func WithBodyString(s string) Option {
	return func(c *config) {
		// 字符串转 Reader 会在 Client.Do() 中处理
	}
}

// ============================================================
// Retry 选项
// ============================================================

// WithRetry 设置重试选项
func WithRetry(opts ...retry.Option) Option {
	return func(c *config) {
		c.retryEnabled = true
		c.retryOpts = opts
	}
}

// WithRetryDefaults 使用默认重试策略
func WithRetryDefaults() Option {
	return func(c *config) {
		c.retryEnabled = true
		c.retryOpts = retry.HTTPDefaults
	}
}

// DisableRetry 禁用重试
func DisableRetry() Option {
	return func(c *config) {
		c.retryEnabled = false
		c.retryOpts = nil
	}
}

// ============================================================
// 高级选项
// ============================================================

// WithBeforeRequest 设置请求前钩子
func WithBeforeRequest(fn func(*http.Request) error) Option {
	return func(c *config) {
		c.beforeRequest = fn
	}
}

// WithAfterResponse 设置响应后钩子
func WithAfterResponse(fn func(*Response) error) Option {
	return func(c *config) {
		c.afterResponse = fn
	}
}

// ============================================================
// 内部辅助函数
// ============================================================

// newConfig 创建默认配置
func newConfig() *config {
	return &config{
		timeout:      30 * time.Second, // 默认 30 秒超时
		headers:      make(map[string]string),
		queries:      make(url.Values),
		retryEnabled: false,
	}
}

// applyOptions 应用选项
func applyOptions(cfg *config, opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
}

// merge 合并配置（Request 级配置覆盖 Client 级配置）
func (c *config) merge(other *config) *config {
	merged := &config{
		baseURL:         c.baseURL,
		timeout:         c.timeout,
		transport:       c.transport,
		cookieJar:       c.cookieJar,
		headers:         make(map[string]string),
		queries:         make(url.Values),
		retryEnabled:    c.retryEnabled,
		retryOpts:       c.retryOpts,
		breakerManager:  c.breakerManager,
		breakerResource: c.breakerResource,
		breakerFallback: c.breakerFallback,
		breakerDisabled: c.breakerDisabled,
		beforeRequest:   c.beforeRequest,
		afterResponse:   c.afterResponse,
	}
	
	// 合并 Headers
	for k, v := range c.headers {
		merged.headers[k] = v
	}
	for k, v := range other.headers {
		merged.headers[k] = v
	}
	
	// 合并 Queries
	for k, vs := range c.queries {
		for _, v := range vs {
			merged.queries.Add(k, v)
		}
	}
	for k, vs := range other.queries {
		for _, v := range vs {
			merged.queries.Add(k, v)
		}
	}
	
	// Request 级配置覆盖
	if other.ctx != nil {
		merged.ctx = other.ctx
	}
	if other.body != nil {
		merged.body = other.body
	}
	if other.timeout > 0 {
		merged.timeout = other.timeout
	}
	
	// Retry 配置覆盖
	if other.retryEnabled != c.retryEnabled || len(other.retryOpts) > 0 {
		merged.retryEnabled = other.retryEnabled
		merged.retryOpts = other.retryOpts
	}
	
	// Breaker 配置覆盖
	if other.breakerManager != nil {
		merged.breakerManager = other.breakerManager
	}
	if other.breakerResource != "" {
		merged.breakerResource = other.breakerResource
	}
	if other.breakerFallback != nil {
		merged.breakerFallback = other.breakerFallback
	}
	if other.breakerDisabled {
		merged.breakerDisabled = other.breakerDisabled
	}
	
	// 钩子函数覆盖
	if other.beforeRequest != nil {
		merged.beforeRequest = other.beforeRequest
	}
	if other.afterResponse != nil {
		merged.afterResponse = other.afterResponse
	}
	
	return merged
}

