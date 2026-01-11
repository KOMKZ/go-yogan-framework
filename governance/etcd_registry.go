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

// EtcdRegistry etcd 服务注册实现
type EtcdRegistry struct {
	// etcd 客户端
	client *clientv3.Client

	// 服务信息
	serviceInfo *ServiceInfo

	// 租约管理
	leaseID     clientv3.LeaseID
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse

	// 生命周期管理
	ctx    context.Context
	cancel context.CancelFunc

	// 状态管理
	mu         sync.RWMutex
	registered bool

	// 重试控制
	retryEnabled      bool
	maxRetries        int
	initialRetryDelay time.Duration
	maxRetryDelay     time.Duration
	retryBackoff      float64
	onRegisterFailed  func(error)

	// 日志
	logger *logger.CtxZapLogger
}

// EtcdConfig etcd 注册配置（已废弃，使用 EtcdRegistryConfig）
type EtcdConfig = EtcdRegistryConfig

// NewEtcdRegistry 创建 etcd 注册器
func NewEtcdRegistry(cfg EtcdRegistryConfig, log *logger.CtxZapLogger) (*EtcdRegistry, error) {
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 创建 etcd 客户端
	client, err := clientv3.New(clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
		Username:    cfg.Username,
		Password:    cfg.Password,
		Logger:      log.GetZapLogger(), // 🎯 注入我们的 logger
	})
	if err != nil {
		return nil, fmt.Errorf("create etcd client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// 设置默认值
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

// Register 注册服务
func (r *EtcdRegistry) Register(ctx context.Context, info *ServiceInfo) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 支持幂等重新注册：如果已注册，先清理旧状态
	if r.registered {
		r.logger.WarnCtx(ctx, "Service already registered, will re-register")
		r.cancel() // 停止旧的心跳监控
		r.ctx, r.cancel = context.WithCancel(context.Background())
	}

	// 保存服务信息
	r.serviceInfo = info

	// 创建租约
	lease := clientv3.NewLease(r.client)
	leaseResp, err := lease.Grant(ctx, info.TTL)
	if err != nil {
		return fmt.Errorf("grant lease: %w", err)
	}

	r.leaseID = leaseResp.ID

	// 构造服务key和value
	serviceKey := r.buildServiceKey(info)
	serviceValue, err := r.marshalServiceInfo(info)
	if err != nil {
		return fmt.Errorf("marshal service info: %w", err)
	}

	// 注册服务（绑定租约）
	_, err = r.client.Put(ctx, serviceKey, serviceValue, clientv3.WithLease(r.leaseID))
	if err != nil {
		// 撤销租约
		lease.Revoke(context.Background(), r.leaseID)
		return fmt.Errorf("put service: %w", err)
	}

	// 启动心跳保活
	keepAliveCh, err := lease.KeepAlive(r.ctx, r.leaseID)
	if err != nil {
		// 删除服务并撤销租约
		r.client.Delete(context.Background(), serviceKey)
		lease.Revoke(context.Background(), r.leaseID)
		return fmt.Errorf("start keepalive: %w", err)
	}

	r.keepAliveCh = keepAliveCh
	r.registered = true

	// 启动心跳监控
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

// Deregister 注销服务
func (r *EtcdRegistry) Deregister(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.registered {
		return ErrNotRegistered
	}

	// 停止心跳
	r.cancel()

	// 删除服务
	serviceKey := r.buildServiceKey(r.serviceInfo)
	_, err := r.client.Delete(ctx, serviceKey)
	if err != nil {
		r.logger.ErrorCtx(ctx, "Failed to delete service", zap.Error(err))
	}

	// 撤销租约
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

// UpdateMetadata 更新服务元数据
func (r *EtcdRegistry) UpdateMetadata(ctx context.Context, metadata map[string]string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if !r.registered {
		return ErrNotRegistered
	}

	// 更新本地元数据
	if r.serviceInfo.Metadata == nil {
		r.serviceInfo.Metadata = make(map[string]string)
	}
	for k, v := range metadata {
		r.serviceInfo.Metadata[k] = v
	}

	// 重新序列化并更新到 etcd
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

// IsRegistered 检查服务是否已注册
func (r *EtcdRegistry) IsRegistered() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.registered
}

// Close 关闭注册器
func (r *EtcdRegistry) Close() error {
	r.cancel()
	return r.client.Close()
}

// monitorKeepAlive 监控心跳续约
func (r *EtcdRegistry) monitorKeepAlive() {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	lastHeartbeat := time.Now()
	ctx := context.Background() // 创建后台 context

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
			// 🎯 超时检测：超过 10 秒未收到心跳响应
			if time.Since(lastHeartbeat) > 10*time.Second {
				r.logger.WarnCtx(ctx, "⚠️  Heartbeat timeout, possible network issue",
					zap.String("service", r.serviceInfo.ServiceName),
					zap.Duration("since_last", time.Since(lastHeartbeat)),
				)
			}
		}
	}
}

// handleKeepAliveFailure 处理心跳失败
func (r *EtcdRegistry) handleKeepAliveFailure() {
	ctx := context.Background()
	r.mu.Lock()
	r.registered = false
	r.mu.Unlock()

	r.logger.ErrorCtx(ctx, "❌ Heartbeat channel closed, starting retry registration",
		zap.String("service", r.serviceInfo.ServiceName))

	if r.retryEnabled {
		go r.retryRegister() // 启动重试流程
	} else {
		// 不启用重试，触发失败回调
		if r.onRegisterFailed != nil {
			r.onRegisterFailed(ErrKeepAliveFailed)
		}
	}
}

// retryRegister 重试注册（带指数退避）
func (r *EtcdRegistry) retryRegister() {
	ctx := context.Background()

	retryDelay := r.initialRetryDelay
	retryCount := 0

	for {
		// 检查是否被取消
		select {
		case <-r.ctx.Done():
			r.logger.DebugCtx(ctx, "Retry cancelled")
			return
		default:
		}

		// 检查重试次数限制
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

		// 🎯 关键步骤1：健康检查前置
		if !r.checkEtcdHealth(ctx) {
			r.logger.WarnCtx(ctx, "⚠️  etcd health check failed, waiting for next retry")
			retryDelay = r.calculateBackoff(retryDelay)
			continue
		}

		// 🎯 关键步骤2：尝试重新注册
		err := r.reRegister(ctx)
		if err == nil {
			r.logger.DebugCtx(ctx, "✅ Re-registration succeeded",
				zap.Int("attempts", retryCount))
			return
		}

		r.logger.WarnCtx(ctx, "⚠️  Re-registration failed",
			zap.Error(err),
			zap.Int("attempt", retryCount))

		// 🎯 关键步骤3：指数退避
		retryDelay = r.calculateBackoff(retryDelay)
	}
}

// buildServiceKey 构造服务key
// 格式: /services/{serviceName}/{instanceID}
func (r *EtcdRegistry) buildServiceKey(info *ServiceInfo) string {
	return fmt.Sprintf("/services/%s/%s", info.ServiceName, info.InstanceID)
}

// marshalServiceInfo 序列化服务信息
func (r *EtcdRegistry) marshalServiceInfo(info *ServiceInfo) (string, error) {
	data, err := json.Marshal(info)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// checkEtcdHealth 检查 etcd 健康状态
func (r *EtcdRegistry) checkEtcdHealth(ctx context.Context) bool {
	checkCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	// 尝试读取一个 key（不存在也算健康）
	_, err := r.client.Get(checkCtx, "/health-check")
	if err == nil {
		return true
	}

	// key 不存在说明 etcd 可访问
	if err.Error() == "etcdserver: key not found" {
		return true
	}

	r.logger.WarnCtx(ctx, "etcd health check failed", zap.Error(err))
	return false
}

// reRegister 重新执行注册流程
func (r *EtcdRegistry) reRegister(ctx context.Context) error {
	// 清理旧租约（如果存在）
	if r.leaseID > 0 {
		r.client.Revoke(context.Background(), r.leaseID)
		r.leaseID = 0
	}

	// 调用 Register 重新走一遍完整流程
	return r.Register(ctx, r.serviceInfo)
}

// calculateBackoff 计算指数退避延迟（带上限）
func (r *EtcdRegistry) calculateBackoff(current time.Duration) time.Duration {
	next := time.Duration(float64(current) * r.retryBackoff)
	if next > r.maxRetryDelay {
		return r.maxRetryDelay
	}
	return next
}
