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

// internal configuration structure
type config struct {
	// Client configuration
	baseURL    string
	timeout    time.Duration
	transport  *http.Transport
	cookieJar  http.CookieJar
	headers    map[string]string
	
	// Request configuration
	ctx        context.Context
	queries    url.Values
	body       io.Reader
	retryOpts  []retry.Option
	retryEnabled bool
	
	// Breaker configuration
	breakerManager  BreakerManager
	breakerResource string
	breakerFallback func(ctx context.Context, err error) (*Response, error)
	breakerDisabled bool
	
	// Advanced configuration
	beforeRequest func(*http.Request) error
	afterResponse func(*Response) error
}

// Option configuration option type
type Option func(*config)

// ============================================================
// Client level options
// ============================================================

// SetBaseURL sets the base URL
func WithBaseURL(baseURL string) Option {
	return func(c *config) {
		c.baseURL = baseURL
	}
}

// Set timeout duration
func WithTimeout(duration time.Duration) Option {
	return func(c *config) {
		c.timeout = duration
	}
}

// WithHeader sets a single header
func WithHeader(key, value string) Option {
	return func(c *config) {
		if c.headers == nil {
			c.headers = make(map[string]string)
		}
		c.headers[key] = value
	}
}

// WithHeaders set multiple headers
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

// WithTransport set custom Transport
func WithTransport(transport *http.Transport) Option {
	return func(c *config) {
		c.transport = transport
	}
}

// WithInsecureSkipVerify skip TLS verification (insecure, for development environment only)
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

// Set Cookie Jar
func WithCookieJar(jar http.CookieJar) Option {
	return func(c *config) {
		c.cookieJar = jar
	}
}

// ============================================================
// Request level options
// ============================================================

// Set Context with WithContext
func WithContext(ctx context.Context) Option {
	return func(c *config) {
		c.ctx = ctx
	}
}

// WithQuery sets a single query parameter
func WithQuery(key, value string) Option {
	return func(c *config) {
		if c.queries == nil {
			c.queries = make(url.Values)
		}
		c.queries.Set(key, value)
	}
}

// WithQueries set multiple Query parameters
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

// WithJSON sets the JSON Body (automatically sets Content-Type)
func WithJSON(data interface{}) Option {
	return func(c *config) {
		// JSON serialization is handled in Client.Do()
		// This is just a marker; actual serialization is performed during execution.
	}
}

// WithForm sets the Form Body (automatically sets Content-Type)
func WithForm(data map[string]string) Option {
	return func(c *config) {
		// Form encoding will be handled in Client.Do()
	}
}

// WithBody sets the original Body
func WithBody(reader io.Reader) Option {
	return func(c *config) {
		c.body = reader
	}
}

// WithBodyString sets the string Body
func WithBodyString(s string) Option {
	return func(c *config) {
		// Converting the string to a Reader is handled in Client.Do()
	}
}

// ============================================================
// Retry option
// ============================================================

// Set retry options_withRetry
func WithRetry(opts ...retry.Option) Option {
	return func(c *config) {
		c.retryEnabled = true
		c.retryOpts = opts
	}
}

// Use default retry strategy
func WithRetryDefaults() Option {
	return func(c *config) {
		c.retryEnabled = true
		c.retryOpts = retry.HTTPDefaults
	}
}

// DisableRetry Disable retry
func DisableRetry() Option {
	return func(c *config) {
		c.retryEnabled = false
		c.retryOpts = nil
	}
}

// ============================================================
// Advanced options
// ============================================================

// WithBeforeRequest set request pre-hook
func WithBeforeRequest(fn func(*http.Request) error) Option {
	return func(c *config) {
		c.beforeRequest = fn
	}
}

// Sets the post-response hook
func WithAfterResponse(fn func(*Response) error) Option {
	return func(c *config) {
		c.afterResponse = fn
	}
}

// ============================================================
// Internal auxiliary function
// ============================================================

// Create default configuration
func newConfig() *config {
	return &config{
		timeout:      30 * time.Second, // Default 30-second timeout
		headers:      make(map[string]string),
		queries:      make(url.Values),
		retryEnabled: false,
	}
}

// Apply options
func applyOptions(cfg *config, opts []Option) {
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
}

// merge Merge configuration (Request level configuration overrides Client level configuration)
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
	
	// Merge Headers
	for k, v := range c.headers {
		merged.headers[k] = v
	}
	for k, v := range other.headers {
		merged.headers[k] = v
	}
	
	// Merge Queries
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
	
	// Request level configuration override
	if other.ctx != nil {
		merged.ctx = other.ctx
	}
	if other.body != nil {
		merged.body = other.body
	}
	if other.timeout > 0 {
		merged.timeout = other.timeout
	}
	
	// Retry configuration override
	if other.retryEnabled != c.retryEnabled || len(other.retryOpts) > 0 {
		merged.retryEnabled = other.retryEnabled
		merged.retryOpts = other.retryOpts
	}
	
	// Breaker configuration override
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
	
	// hook function override
	if other.beforeRequest != nil {
		merged.beforeRequest = other.beforeRequest
	}
	if other.afterResponse != nil {
		merged.afterResponse = other.afterResponse
	}
	
	return merged
}

