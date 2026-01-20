package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// etcdClientConfig etcd client configuration
type etcdClientConfig struct {
	Endpoints   []string
	DialTimeout time.Duration
	Username    string
	Password    string
}

// returns the default configuration for Etcd client
func defaultEtcdClientConfig() etcdClientConfig {
	return etcdClientConfig{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	}
}

// etcdClient encapsulation of etcd client
type etcdClient struct {
	client *clientv3.Client
	config etcdClientConfig
	logger *logger.CtxZapLogger
}

// Create etcd client
func newEtcdClient(cfg etcdClientConfig, log *logger.CtxZapLogger) (*etcdClient, error) {
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// Apply default values
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	// Create etcd client configuration
	clientCfg := clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	}

	// If authentication information is provided
	if cfg.Username != "" {
		clientCfg.Username = cfg.Username
		clientCfg.Password = cfg.Password
	}

	// Connect to etcd
	client, err := clientv3.New(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("Failed to connect to etcd: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	_, err = client.Status(ctx, cfg.Endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("etcd health check failed: %w", err)
	}

	log.DebugCtx(ctx, "✅ etcd ✅ etcd connection successful",
		zap.Strings("endpoints", cfg.Endpoints),
	)

	return &etcdClient{
		client: client,
		config: cfg,
		logger: log,
	}, nil
}

// GetClient Obtain the native etcd client
func (c *etcdClient) GetClient() *clientv3.Client {
	return c.client
}

// Close connection
func (c *etcdClient) Close() error {
	if c.client != nil {
		c.logger.DebugCtx(context.Background(), "English: Close etcd connection etcd English: Close etcd connection")
		return c.client.Close()
	}
	return nil
}

// Set key-value pair
func (c *etcdClient) Put(ctx context.Context, key, value string) error {
	_, err := c.client.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("etcd Put failed: %w", err)
	}
	return nil
}

// Get key value
func (c *etcdClient) Get(ctx context.Context, key string) (string, error) {
	resp, err := c.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("etcd Get failed: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key not found: %s", key)
	}

	return string(resp.Kvs[0].Value), nil
}

// Delete key
func (c *etcdClient) Delete(ctx context.Context, key string) error {
	_, err := c.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd Delete failed: %w", err)
	}
	return nil
}

// GetWithPrefix Retrieve all key-value pairs with the specified prefix
func (c *etcdClient) GetWithPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd GetWithPrefix failed: %w", err)
	}

	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}

	return result, nil
}
