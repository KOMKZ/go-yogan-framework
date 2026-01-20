package governance

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdRegistry implementation for service registration
type EtcdRegistry struct {
	// etcd client
	client *clientv3.Client

	// service information
	serviceInfo *ServiceInfo

	// lease management
	leaseID     clientv3.LeaseID
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse

	// Lifecycle Management
	ctx    context.Context
	cancel context.CancelFunc

	// state management
	mu         sync.RWMutex
	registered bool

	// Retry control
	retryEnabled      bool
	maxRetries        int
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
	retryBackoff      float64
	onRegisterFailed  func(error)

	// Log
	logger *logger.CtxZapLogger
}

// EtcdConfig etcd registration configuration (deprecated, use EtcdRegistryConfig)
type EtcdConfig = EtcdRegistryConfig

// NewEtcdRegistry creates an etcd registry
func NewEtcdRegistry(cfg EtcdRegistryConfig, log *logger.CtxZapLogger) (*EtcdRegistry, error) {
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Create etcd client
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
		Logger:      log.GetZapLogger(), // 🎯 Inject our logger
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Set default values
	retryEnabled := cfg.EnableRetry
	initialRetryDelay := cfg.InitialRetryDelay
	if initialRetryDelay == 0 {
		initialRetryDelay = 1 * time.Second
	}
	maxRetryDelay := cfg.MaxRetryDelay
	if maxRetryDelay == 0 {
		maxRetryDelay = 30 * time.Second
	}
	retryBackoff := cfg.RetryBackoff
	if retryBackoff == 0 {
		retryBackoff = 2.0
	}

	return &EtcdRegistry{
		client:            client,
		ctx:               ctx,
		cancel:            cancel,
		logger:            log,
		retryEnabled:      retryEnabled,
		maxRetries:        cfg.MaxRetries,
		initialRetryDelay: initialRetryDelay,
		maxRetryDelay:     maxRetryDelay,
		retryBackoff:      retryBackoff,
		onRegisterFailed:  cfg.OnRegisterFailed,
	}, nil
}

// Register service registration
func (r *EtcdRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Supports idempotent re-registration: if already registered, clean up old status first
	if r.registered {
		r.logger.WarnCtx(ctx, "Service already registered, will re-register")
		r.cancel() // Stop old heartbeat monitoring
		r.ctx, r.cancel = context.WithCancel(context.Background())
	}

	// Save service information
	r.serviceInfo = info

	// Create lease
	lease := clientv3.NewLease(r.client)
	leaseResp, err := lease.Grant(ctx, info.TTL)
	if err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}

	r.leaseID = leaseResp.ID

	// Construct service key and value
	serviceKey := r.buildServiceKey(info)
	serviceValue, err := r.marshalServiceInfo(info)
	if err != nil {
		return fmt.Errorf("marshal service info: %w", err)
	}

	// Register service (bind lease)
	_, err = r.client.Put(ctx, serviceKey, serviceValue, clientv3.WithLease(r.leaseID))
	if err != nil {
		// Revoke lease
		lease.Revoke(context.Background(), r.leaseID)
		return fmt.Errorf("put service: %w", err)
	}

	// Start heartbeat keepalive
	keepAliveCh, err := lease.KeepAlive(r.ctx, r.leaseID)
	if err != nil {
		// Delete service and revoke lease
		r.client.Delete(context.Background(), serviceKey)
		lease.Revoke(context.Background(), r.leaseID)
		return fmt.Errorf("start keepalive: %w", err)
	}

	r.keepAliveCh = keepAliveCh
	r.registered = true

	// Start heartbeat monitoring
	go r.monitorKeepAlive()

	r.logger.DebugCtx(ctx, "✅ Service registered to etcd",
		zap.String("key", serviceKey),
		zap.String("service", info.ServiceName),
		zap.String("instance", info.InstanceID),
		zap.Int64("ttl", info.TTL),
		zap.String("lease_id", fmt.Sprintf("%x", r.leaseID)),
	)

	return nil
}

// Unregister service
func (r *EtcdRegistry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.registered {
		return ErrNotRegistered
	}

	// Stop heartbeat
	r.cancel()

	// Delete service
	serviceKey := r.buildServiceKey(r.serviceInfo)
	_, err := r.client.Delete(ctx, serviceKey)
	if err != nil {
		r.logger.ErrorCtx(ctx, "Failed to delete service", zap.Error(err))
	}

	// Revoke lease
	if r.leaseID > 0 {
		_, err = r.client.Revoke(ctx, r.leaseID)
		if err != nil {
			r.logger.ErrorCtx(ctx, "Failed to revoke lease", zap.Error(err))
		}
	}

	r.registered = false

	r.logger.DebugCtx(ctx, "✅ Service deregistered from etcd",
		zap.String("key", serviceKey),
		zap.String("service", r.serviceInfo.ServiceName),
	)

	return nil
}

// UpdateMetadata Update service metadata
func (r *EtcdRegistry) UpdateMetadata(ctx context.Context, metadata map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.registered {
		return ErrNotRegistered
	}

	// Update local metadata
	if r.serviceInfo.Metadata == nil {
		r.serviceInfo.Metadata = make(map[string]string)
	}
	for k, v := range metadata {
		r.serviceInfo.Metadata[k] = v
	}

	// Re serialize and update to etcd
	serviceKey := r.buildServiceKey(r.serviceInfo)
	serviceValue, err := r.marshalServiceInfo(r.serviceInfo)
	if err != nil {
		return fmt.Errorf("marshal service info: %w", err)
	}

	_, err = r.client.Put(ctx, serviceKey, serviceValue, clientv3.WithLease(r.leaseID))
	if err != nil {
		return fmt.Errorf("update service: %w", err)
	}

	r.logger.DebugCtx(ctx, "✅ Service metadata updated", zap.Any("metadata", metadata))

	return nil
}

// Checks if the service is registered
func (r *EtcdRegistry) IsRegistered() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.registered
}

// Close the registry
func (r *EtcdRegistry) Close() error {
	r.cancel()
	return r.client.Close()
}

// monitorKeepAlive Monitor heartbeat renewal
func (r *EtcdRegistry) monitorKeepAlive() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	lastHeartbeat := time.Now()
	ctx := context.Background() // Create background context

	for {
		select {
		case <-r.ctx.Done():
			r.logger.DebugCtx(ctx, "Heartbeat monitoring stopped")
			return

		case resp, ok := <-r.keepAliveCh:
			if !ok {
				r.logger.ErrorCtx(ctx, "Heartbeat channel closed",
					zap.String("service", r.serviceInfo.ServiceName))
				r.handleKeepAliveFailure()
				return
			}

			if resp != nil {
				lastHeartbeat = time.Now()
				r.logger.DebugCtx(ctx, "Heartbeat renewed",
					zap.String("service", r.serviceInfo.ServiceName),
					zap.Int64("ttl", resp.TTL),
				)
			}

		case <-ticker.C:
			// 🎯 Timeout detection: No heartbeat response received within 10 seconds
			if time.Since(lastHeartbeat) > 10*time.Second {
				r.logger.WarnCtx(ctx, "⚠️  Heartbeat timeout, possible network issue",
					zap.String("service", r.serviceInfo.ServiceName),
					zap.Duration("since_last", time.Since(lastHeartbeat)),
				)
			}
		}
	}
}

// handleHeartbeatFailure
func (r *EtcdRegistry) handleKeepAliveFailure() {
	ctx := context.Background()
	r.mu.Lock()
	r.registered = false
	r.mu.Unlock()

	r.logger.ErrorCtx(ctx, "❌ Heartbeat channel closed, starting retry registration",
		zap.String("service", r.serviceInfo.ServiceName))

	if r.retryEnabled {
		go r.retryRegister() // Initiate retry process
	} else {
		// Do not enable retries, trigger failure callback
		if r.onRegisterFailed != nil {
			r.onRegisterFailed(ErrKeepAliveFailed)
		}
	}
}

// retryRegister with exponential backoff
func (r *EtcdRegistry) retryRegister() {
	ctx := context.Background()

	retryDelay := r.initialRetryDelay
	retryCount := 0

	for {
		// Check if cancelled
		select {
		case <-r.ctx.Done():
			r.logger.DebugCtx(ctx, "Retry cancelled")
			return
		default:
		}

		// Check retry count limit
		if r.maxRetries > 0 && retryCount >= r.maxRetries {
			r.logger.ErrorCtx(ctx, "❌ Max retry attempts reached, giving up",
				zap.Int("retries", retryCount))
			if r.onRegisterFailed != nil {
				r.onRegisterFailed(ErrMaxRetriesExceeded)
			}
			return
		}

		retryCount++
		r.logger.DebugCtx(ctx, "🔄 Attempting re-registration",
			zap.Int("attempt", retryCount),
			zap.Duration("delay", retryDelay))

		time.Sleep(retryDelay)

		// 🎯 Key Step 1: Pre-health check setup
		if !r.checkEtcdHealth(ctx) {
			r.logger.WarnCtx(ctx, "⚠️  etcd health check failed, waiting for next retry")
			retryDelay = r.calculateBackoff(retryDelay)
			continue
		}

		// 🎯 Step 2: Attempt re-registration
		err := r.reRegister(ctx)
		if err == nil {
			r.logger.DebugCtx(ctx, "✅ Re-registration succeeded",
				zap.Int("attempts", retryCount))
			return
		}

		r.logger.WarnCtx(ctx, "⚠️  Re-registration failed",
			zap.Error(err),
			zap.Int("attempt", retryCount))

		// 🎯 Key Step 3: Exponential Backoff
		retryDelay = r.calculateBackoff(retryDelay)
	}
}

// build service key
// Format: /services/{serviceName}/{instanceID}
func (r *EtcdRegistry) buildServiceKey(info *ServiceInfo) string {
	return fmt.Sprintf("/services/%s/%s", info.ServiceName, info.InstanceID)
}

// marshalServiceInfo serialize service information
func (r *EtcdRegistry) marshalServiceInfo(info *ServiceInfo) (string, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// checkEtcdHealth Check etcd health status
func (r *EtcdRegistry) checkEtcdHealth(ctx context.Context) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// Try to read a key (considered healthy even if it does not exist)
	_, err := r.client.Get(checkCtx, "/health-check")
	if err == nil {
		return true
	}

	// key does not exist indicates etcd is accessible
	if err.Error() == "etcdserver: key not found" {
		return true
	}

	r.logger.WarnCtx(ctx, "etcd health check failed", zap.Error(err))
	return false
}

// reRegister Re-execute registration process
func (r *EtcdRegistry) reRegister(ctx context.Context) error {
	// Clear old leases (if any)
	if r.leaseID > 0 {
		r.client.Revoke(context.Background(), r.leaseID)
		r.leaseID = 0
	}

	// Call Register to go through the entire process again
	return r.Register(ctx, r.serviceInfo)
}

// calculateBackoff Calculate exponential backoff delay (with upper limit)
func (r *EtcdRegistry) calculateBackoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * r.retryBackoff)
	if next > r.maxRetryDelay {
		return r.maxRetryDelay
	}
	return next
}
