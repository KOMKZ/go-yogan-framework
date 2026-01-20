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

// EtcdDiscovery etcd service discovery implementation
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

// Create etcd service discoverer NewEtcdDiscovery
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

// Discover service instances
func (d *EtcdDiscovery) Discover(ctx context.Context, serviceName string) ([]*ServiceInstance, error) {
	d.serviceName = serviceName
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	// Query all current instances
	resp, err := d.client.GetClient().Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("Query service failed: %w: %w", err)
	}

	d.mu.Lock()
	d.instances = make(map[string]*ServiceInstance)

	for _, kv := range resp.Kvs {
		instance, err := d.parseServiceInstance(string(kv.Key), string(kv.Value))
		if err != nil {
			d.logger.WarnCtx(ctx, "English: Failed to parse service instance",
				zap.String("key", string(kv.Key)),
				zap.Error(err))
			continue
		}
		d.instances[instance.ID] = instance
	}

	instances := d.getInstanceList()
	d.mu.Unlock()

	d.logger.DebugCtx(ctx, "✅ English: Service discovery successful",
		zap.String("service", serviceName),
		zap.Int("instances", len(instances)))

	return instances, nil
}

// Watch for service changes
func (d *EtcdDiscovery) Watch(ctx context.Context, serviceName string) (<-chan []*ServiceInstance, error) {
	// Perform discovery first
	if _, err := d.Discover(ctx, serviceName); err != nil {
		return nil, err
	}

	// Start background listening
	go d.watchChanges(serviceName)

	return d.watchCh, nil
}

// watchChanges listen for service changes
func (d *EtcdDiscovery) watchChanges(serviceName string) {
	prefix := fmt.Sprintf("/services/%s/", serviceName)

	watchChan := d.client.GetClient().Watch(
		d.ctx,
		prefix,
		clientv3.WithPrefix(),
	)

	d.logger.DebugCtx(d.ctx, "🔍 🔍 Starting to monitor service changes", zap.String("service", serviceName))

	for {
		select {
		case <-d.ctx.Done():
			d.logger.DebugCtx(d.ctx, "English: Stop service listener", zap.String("service", serviceName))
			close(d.watchCh)
			return

		case watchResp, ok := <-watchChan:
			if !ok {
				d.logger.ErrorCtx(d.ctx, "Watch English: Watch Channel closed", zap.String("service", serviceName))
				close(d.watchCh)
				return
			}

			if watchResp.Err() != nil {
				d.logger.ErrorCtx(d.ctx, "Watch Watch error",
					zap.String("service", serviceName),
					zap.Error(watchResp.Err()))
				continue
			}

			// Handle change events
			if d.handleWatchEvents(watchResp.Events) {
				// Send updated instance list
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

// handleWatchEvents handles Watch events
func (d *EtcdDiscovery) handleWatchEvents(events []*clientv3.Event) bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	changed := false

	for _, event := range events {
		key := string(event.Kv.Key)
		value := string(event.Kv.Value)

		switch event.Type {
		case clientv3.EventTypePut:
			// service launch or update
			instance, err := d.parseServiceInstance(key, value)
			if err != nil {
				d.logger.WarnCtx(d.ctx, "English: Failed to parse service instance",
					zap.String("key", key),
					zap.Error(err))
				continue
			}

			if _, exists := d.instances[instance.ID]; !exists {
				d.logger.DebugCtx(d.ctx, "🟢 GREEN Service instance online",
					zap.String("service", d.serviceName),
					zap.String("instance", instance.ID),
					zap.String("address", instance.GetAddress()))
			}

			d.instances[instance.ID] = instance
			changed = true

		case clientv3.EventTypeDelete:
			// service offline
			instanceID := extractInstanceIDFromKey(key)
			if _, exists := d.instances[instanceID]; exists {
				d.logger.WarnCtx(d.ctx, "🔴 English: ⚫ Service instance offline",
					zap.String("service", d.serviceName),
					zap.String("instance", instanceID))
				delete(d.instances, instanceID)
				changed = true
			}
		}
	}

	return changed
}

// parseServiceInstance Parse service instance information
func (d *EtcdDiscovery) parseServiceInstance(key, value string) (*ServiceInstance, error) {
	// Extract instanceID from key
	// Key format: /services/{ serviceName }/{ instanceID }
	instanceID := extractInstanceIDFromKey(key)

	// Try to parse the ServiceInfo in JSON format
	var info ServiceInfo
	if err := json.Unmarshal([]byte(value), &info); err != nil {
		// Degradation: If not JSON, assume value is the address
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

	// Parsing successful, convert to ServiceInstance
	return &ServiceInstance{
		ID:       instanceID,
		Service:  info.ServiceName,
		Address:  info.Address,
		Port:     info.Port,
		Metadata: info.Metadata,
		Weight:   100, // Default weight
		Healthy:  true,
	}, nil
}

// Stop Service discovery
func (d *EtcdDiscovery) Stop() {
	d.cancel()
	d.logger.DebugCtx(context.Background(), "✅ English: Service discovery has stopped", zap.String("service", d.serviceName))
}

// get instance list (lock required)
func (d *EtcdDiscovery) getInstanceList() []*ServiceInstance {
	instances := make([]*ServiceInstance, 0, len(d.instances))
	for _, inst := range d.instances {
		instances = append(instances, inst)
	}
	return instances
}

// extract instance ID from key
// Key format: /services/{ serviceName }/{ instanceID }
func extractInstanceIDFromKey(key string) string {
	parts := strings.Split(key, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return key
}

// parseAddress parses the IP from the address string
// Format: "127.0.0.1:9002" -> "127.0.0.1"
func parseAddress(addr string) string {
	if idx := strings.LastIndex(addr, ":"); idx > 0 {
		return addr[:idx]
	}
	return addr
}

// parsePort parses the port from the address string
// Format: "127.0.0.1:9002" -> 9002
func parsePort(addr string) int {
	if idx := strings.LastIndex(addr, ":"); idx > 0 && idx < len(addr)-1 {
		portStr := addr[idx+1:]
		var port int
		fmt.Sscanf(portStr, "%d", &port)
		return port
	}
	return 0
}
