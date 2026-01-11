package governance

import (
	"context"
	"fmt"
	"time"

	"github.com/KOMKZ/go-yogan-framework/logger"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

// etcdClientConfig etcd 客户端配置
type etcdClientConfig struct {
	Endpoints   []string
	DialTimeout time.Duration
	Username    string
	Password    string
}

// defaultEtcdClientConfig 返回默认配置
func defaultEtcdClientConfig() etcdClientConfig {
	return etcdClientConfig{
		Endpoints:   []string{"127.0.0.1:2379"},
		DialTimeout: 5 * time.Second,
	}
}

// etcdClient etcd 客户端封装
type etcdClient struct {
	client *clientv3.Client
	config etcdClientConfig
	logger *logger.CtxZapLogger
}

// newEtcdClient 创建 etcd 客户端
func newEtcdClient(cfg etcdClientConfig, log *logger.CtxZapLogger) (*etcdClient, error) {
	if log == nil {
		log = logger.GetLogger("yogan")
	}

	// 应用默认值
	if cfg.DialTimeout == 0 {
		cfg.DialTimeout = 5 * time.Second
	}

	// 创建 etcd 客户端配置
	clientCfg := clientv3.Config{
		Endpoints:   cfg.Endpoints,
		DialTimeout: cfg.DialTimeout,
	}

	// 如果提供了认证信息
	if cfg.Username != "" {
		clientCfg.Username = cfg.Username
		clientCfg.Password = cfg.Password
	}

	// 连接 etcd
	client, err := clientv3.New(clientCfg)
	if err != nil {
		return nil, fmt.Errorf("连接 etcd 失败: %w", err)
	}

	// 测试连接
	ctx, cancel := context.WithTimeout(context.Background(), cfg.DialTimeout)
	defer cancel()

	_, err = client.Status(ctx, cfg.Endpoints[0])
	if err != nil {
		client.Close()
		return nil, fmt.Errorf("etcd 健康检查失败: %w", err)
	}

	log.DebugCtx(ctx, "✅ etcd 连接成功",
		zap.Strings("endpoints", cfg.Endpoints),
	)

	return &etcdClient{
		client: client,
		config: cfg,
		logger: log,
	}, nil
}

// GetClient 获取原生 etcd 客户端
func (c *etcdClient) GetClient() *clientv3.Client {
	return c.client
}

// Close 关闭连接
func (c *etcdClient) Close() error {
	if c.client != nil {
		c.logger.DebugCtx(context.Background(), "关闭 etcd 连接")
		return c.client.Close()
	}
	return nil
}

// Put 设置键值
func (c *etcdClient) Put(ctx context.Context, key, value string) error {
	_, err := c.client.Put(ctx, key, value)
	if err != nil {
		return fmt.Errorf("etcd Put 失败: %w", err)
	}
	return nil
}

// Get 获取键值
func (c *etcdClient) Get(ctx context.Context, key string) (string, error) {
	resp, err := c.client.Get(ctx, key)
	if err != nil {
		return "", fmt.Errorf("etcd Get 失败: %w", err)
	}

	if len(resp.Kvs) == 0 {
		return "", fmt.Errorf("key not found: %s", key)
	}

	return string(resp.Kvs[0].Value), nil
}

// Delete 删除键
func (c *etcdClient) Delete(ctx context.Context, key string) error {
	_, err := c.client.Delete(ctx, key)
	if err != nil {
		return fmt.Errorf("etcd Delete 失败: %w", err)
	}
	return nil
}

// GetWithPrefix 获取指定前缀的所有键值
func (c *etcdClient) GetWithPrefix(ctx context.Context, prefix string) (map[string]string, error) {
	resp, err := c.client.Get(ctx, prefix, clientv3.WithPrefix())
	if err != nil {
		return nil, fmt.Errorf("etcd GetWithPrefix 失败: %w", err)
	}

	result := make(map[string]string)
	for _, kv := range resp.Kvs {
		result[string(kv.Key)] = string(kv.Value)
	}

	return result, nil
}
