package governance

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/KOMKZ/go-yogan-framework/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// EtcdDiscovery etcd 服务发现实现
type EtcdDiscovery struct {
	client      *etcdClient
	serviceName string
	instances   map[string]*ServiceInstance // key: instanceID
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *logger.CtxZapLogger
	watchCh     chan []*ServiceInstance
}

// NewEtcdDiscovery 创建 etcd 服务发现器
func NewEtcdDiscovery(client *etcdClient, log *logger.CtxZapLogger) *EtcdDiscovery {
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &EtcdDiscovery{
		client:    client,
		ctx:       ctx,
		cancel:    cancel,
		logger:    log,
		instances: make(map[string]*ServiceInstance),
		watchCh:   make(chan []*ServiceInstance, 10),
	}
}

// Discover 发现服务实例
func (d *EtcdDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.serviceName = serviceName
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	// 查询当前所有实例
	resp, err := d.client.GetClient().Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("查询服务失败: %w", err)
	}

	d.mu.Lock()
	d.instances = make(map[string]*ServiceInstance)

	for _, kv := range resp.Kvs {
		instance, err := d.parseServiceInstance(string(kv.Key), string(kv.Value))
		if err != nil {
			d.logger.WarnCtx(ctx, "解析服务实例失败",
				zap.String("key", string(kv.Key)),
				zap.Error(err))
			continue
		}
		d.instances[instance.ID] = instance
	}

	instances := d.getInstanceList()
	d.mu.Unlock()

	d.logger.DebugCtx(ctx, "✅ 服务发现成功",
		zap.String("service", serviceName),
		zap.Int("instances", len(instances)))

	return instances, nil
}

// Watch 监听服务变更
func (d *EtcdDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	// 先执行一次发现
	if _, err := d.Discover(ctx, serviceName); err != nil {
		return nil, err
	}

	// 启动后台监听
	go d.watchChanges(serviceName)

	return d.watchCh, nil
}

// watchChanges 监听服务变更
func (d *EtcdDiscovery) watchChanges(serviceName string) {
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	watchChan := d.client.GetClient().Watch(
		d.ctx,
		prefix,
		clientv3.WithPrefix(),
	)

	d.logger.DebugCtx(d.ctx, "🔍 开始监听服务变更", zap.String("service", serviceName))

	for {
		select {
		case <-d.ctx.Done():
			d.logger.DebugCtx(d.ctx, "停止服务监听", zap.String("service", serviceName))
			close(d.watchCh)
			return

		case watchResp, ok := <-watchChan:
			if !ok {
				d.logger.ErrorCtx(d.ctx, "Watch 通道关闭", zap.String("service", serviceName))
				close(d.watchCh)
				return
			}

			if watchResp.Err() != nil {
				d.logger.ErrorCtx(d.ctx, "Watch 错误",
					zap.String("service", serviceName),
					zap.Error(watchResp.Err()))
				continue
			}

			// 处理变更事件
			if d.handleWatchEvents(watchResp.Events) {
				// 发送更新后的实例列表
				d.mu.RLock()
				instances := d.getInstanceList()
				d.mu.RUnlock()

				select {
				case d.watchCh <- instances:
				case <-d.ctx.Done():
					return
				}
			}
		}
	}
}

// handleWatchEvents 处理 Watch 事件
func (d *EtcdDiscovery) handleWatchEvents(events []*clientv3.Event) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	changed := false

	for _, event := range events {
		key := string(event.Kv.Key)
		value := string(event.Kv.Value)

		switch event.Type {
		case clientv3.EventTypePut:
			// 服务上线或更新
			instance, err := d.parseServiceInstance(key, value)
			if err != nil {
				d.logger.WarnCtx(d.ctx, "解析服务实例失败",
					zap.String("key", key),
					zap.Error(err))
				continue
			}

			if _, exists := d.instances[instance.ID]; !exists {
				d.logger.DebugCtx(d.ctx, "🟢 服务实例上线",
					zap.String("service", d.serviceName),
					zap.String("instance", instance.ID),
					zap.String("address", instance.GetAddress()))
			}

			d.instances[instance.ID] = instance
			changed = true

		case clientv3.EventTypeDelete:
			// 服务下线
			instanceID := extractInstanceIDFromKey(key)
			if _, exists := d.instances[instanceID]; exists {
				d.logger.WarnCtx(d.ctx, "🔴 服务实例下线",
					zap.String("service", d.serviceName),
					zap.String("instance", instanceID))
				delete(d.instances, instanceID)
				changed = true
			}
		}
	}

	return changed
}

// parseServiceInstance 解析服务实例信息
func (d *EtcdDiscovery) parseServiceInstance(key, value string) (*ServiceInstance, error) {
	// 从 key 提取 instanceID
	// Key 格式: /services/{serviceName}/{instanceID}
	instanceID := extractInstanceIDFromKey(key)

	// 尝试解析 JSON 格式的 ServiceInfo
	var info ServiceInfo
	if err := json.Unmarshal([]byte(value), &info); err != nil {
		// 降级：如果不是 JSON，假设 value 就是地址
		return &ServiceInstance{
			ID:       instanceID,
			Service:  d.serviceName,
			Address:  parseAddress(value),
			Port:     parsePort(value),
			Metadata: make(map[string]string),
			Weight:   100,
			Healthy:  true,
		}, nil
	}

	// 解析成功，转换为 ServiceInstance
	return &ServiceInstance{
		ID:       instanceID,
		Service:  info.ServiceName,
		Address:  info.Address,
		Port:     info.Port,
		Metadata: info.Metadata,
		Weight:   100, // 默认权重
		Healthy:  true,
	}, nil
}

// Stop 停止服务发现
func (d *EtcdDiscovery) Stop() {
	d.cancel()
	d.logger.DebugCtx(context.Background(), "✅ 服务发现已停止", zap.String("service", d.serviceName))
}

// getInstanceList 获取实例列表（需要持有锁）
func (d *EtcdDiscovery) getInstanceList() []*ServiceInstance {
	instances := make([]*ServiceInstance, 0, len(d.instances))
	for _, inst := range d.instances {
		instances = append(instances, inst)
	}
	return instances
}

// extractInstanceIDFromKey 从 key 提取实例ID
// Key 格式: /services/{serviceName}/{instanceID}
func extractInstanceIDFromKey(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}

// parseAddress 从地址字符串解析 IP
// 格式: "127.0.0.1:9002" -> "127.0.0.1"
func parseAddress(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}

// parsePort 从地址字符串解析端口
// 格式: "127.0.0.1:9002" -> 9002
func parsePort(addr string) int {
	if idx := strings.LastIndex(addr, ":"); idx > 0 && idx < len(addr)-1 {
		portStr := addr[idx+1:]
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		return port
	}
	return 0
}
