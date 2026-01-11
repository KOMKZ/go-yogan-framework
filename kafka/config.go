package kafka

import (
	"fmt"
	"time"
)

// Config Kafka 配置
type Config struct {
	// Brokers Kafka 集群地址列表
	Brokers []string `mapstructure:"brokers"`

	// Version Kafka 版本（如 "3.8.0"）
	Version string `mapstructure:"version"`

	// ClientID 客户端标识
	ClientID string `mapstructure:"client_id"`

	// Producer 生产者配置
	Producer ProducerConfig `mapstructure:"producer"`

	// Consumer 消费者配置
	Consumer ConsumerConfig `mapstructure:"consumer"`

	// SASL 认证配置（可选）
	SASL *SASLConfig `mapstructure:"sasl"`

	// TLS 配置（可选）
	TLS *TLSConfig `mapstructure:"tls"`
}

// ProducerConfig 生产者配置
type ProducerConfig struct {
	// Enabled 是否启用生产者
	Enabled bool `mapstructure:"enabled"`

	// RequiredAcks 确认级别：0=NoResponse, 1=WaitForLocal, -1=WaitForAll
	RequiredAcks int `mapstructure:"required_acks"`

	// Timeout 生产超时时间
	Timeout time.Duration `mapstructure:"timeout"`

	// RetryMax 最大重试次数
	RetryMax int `mapstructure:"retry_max"`

	// RetryBackoff 重试间隔
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`

	// MaxMessageBytes 单条消息最大字节数
	MaxMessageBytes int `mapstructure:"max_message_bytes"`

	// Compression 压缩算法：none, gzip, snappy, lz4, zstd
	Compression string `mapstructure:"compression"`

	// Idempotent 是否启用幂等生产者
	Idempotent bool `mapstructure:"idempotent"`

	// BatchSize 批量发送大小
	BatchSize int `mapstructure:"batch_size"`

	// FlushFrequency 刷新频率
	FlushFrequency time.Duration `mapstructure:"flush_frequency"`
}

// ConsumerConfig 消费者配置
type ConsumerConfig struct {
	// Enabled 是否启用消费者
	Enabled bool `mapstructure:"enabled"`

	// GroupID 消费者组 ID
	GroupID string `mapstructure:"group_id"`

	// Topics 订阅的 Topic 列表
	Topics []string `mapstructure:"topics"`

	// OffsetInitial 初始 Offset：-1=Newest, -2=Oldest
	OffsetInitial int64 `mapstructure:"offset_initial"`

	// AutoCommit 是否自动提交 Offset
	AutoCommit bool `mapstructure:"auto_commit"`

	// AutoCommitInterval 自动提交间隔
	AutoCommitInterval time.Duration `mapstructure:"auto_commit_interval"`

	// SessionTimeout 会话超时
	SessionTimeout time.Duration `mapstructure:"session_timeout"`

	// HeartbeatInterval 心跳间隔
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`

	// MaxProcessingTime 单条消息最大处理时间
	MaxProcessingTime time.Duration `mapstructure:"max_processing_time"`

	// FetchMin 最小拉取字节数
	FetchMin int32 `mapstructure:"fetch_min"`

	// FetchMax 最大拉取字节数
	FetchMax int32 `mapstructure:"fetch_max"`

	// FetchDefault 默认拉取字节数
	FetchDefault int32 `mapstructure:"fetch_default"`

	// RebalanceStrategy 再平衡策略：range, roundrobin, sticky
	RebalanceStrategy string `mapstructure:"rebalance_strategy"`
}

// SASLConfig SASL 认证配置
type SASLConfig struct {
	// Enabled 是否启用
	Enabled bool `mapstructure:"enabled"`

	// Mechanism 认证机制：PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	Mechanism string `mapstructure:"mechanism"`

	// Username 用户名
	Username string `mapstructure:"username"`

	// Password 密码
	Password string `mapstructure:"password"`
}

// TLSConfig TLS 配置
type TLSConfig struct {
	// Enabled 是否启用 TLS
	Enabled bool `mapstructure:"enabled"`

	// CertFile 证书文件路径
	CertFile string `mapstructure:"cert_file"`

	// KeyFile 密钥文件路径
	KeyFile string `mapstructure:"key_file"`

	// CAFile CA 证书文件路径
	CAFile string `mapstructure:"ca_file"`

	// InsecureSkipVerify 是否跳过证书验证
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

// Validate 验证配置
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("brokers cannot be empty")
	}

	for _, broker := range c.Brokers {
		if broker == "" {
			return fmt.Errorf("broker address cannot be empty")
		}
	}

	// 验证生产者配置
	if c.Producer.Enabled {
		if err := c.Producer.Validate(); err != nil {
			return fmt.Errorf("producer config invalid: %w", err)
		}
	}

	// 验证消费者配置
	if c.Consumer.Enabled {
		if err := c.Consumer.Validate(); err != nil {
			return fmt.Errorf("consumer config invalid: %w", err)
		}
	}

	// 验证 SASL 配置
	if c.SASL != nil && c.SASL.Enabled {
		if err := c.SASL.Validate(); err != nil {
			return fmt.Errorf("sasl config invalid: %w", err)
		}
	}

	return nil
}

// Validate 验证生产者配置
func (c *ProducerConfig) Validate() error {
	if c.RequiredAcks < -1 || c.RequiredAcks > 1 {
		return fmt.Errorf("required_acks must be -1, 0, or 1, got: %d", c.RequiredAcks)
	}

	if c.MaxMessageBytes < 0 {
		return fmt.Errorf("max_message_bytes must be >= 0, got: %d", c.MaxMessageBytes)
	}

	validCompressions := map[string]bool{
		"":       true,
		"none":   true,
		"gzip":   true,
		"snappy": true,
		"lz4":    true,
		"zstd":   true,
	}
	if !validCompressions[c.Compression] {
		return fmt.Errorf("invalid compression: %s", c.Compression)
	}

	return nil
}

// Validate 验证消费者配置
func (c *ConsumerConfig) Validate() error {
	if c.GroupID == "" {
		return fmt.Errorf("group_id cannot be empty")
	}

	if len(c.Topics) == 0 {
		return fmt.Errorf("topics cannot be empty")
	}

	for _, topic := range c.Topics {
		if topic == "" {
			return fmt.Errorf("topic name cannot be empty")
		}
	}

	validStrategies := map[string]bool{
		"":           true,
		"range":      true,
		"roundrobin": true,
		"sticky":     true,
	}
	if !validStrategies[c.RebalanceStrategy] {
		return fmt.Errorf("invalid rebalance_strategy: %s", c.RebalanceStrategy)
	}

	return nil
}

// Validate 验证 SASL 配置
func (c *SASLConfig) Validate() error {
	if c.Username == "" {
		return fmt.Errorf("username cannot be empty")
	}

	if c.Password == "" {
		return fmt.Errorf("password cannot be empty")
	}

	validMechanisms := map[string]bool{
		"PLAIN":          true,
		"SCRAM-SHA-256":  true,
		"SCRAM-SHA-512":  true,
	}
	if !validMechanisms[c.Mechanism] {
		return fmt.Errorf("invalid mechanism: %s", c.Mechanism)
	}

	return nil
}

// ApplyDefaults 应用默认值
func (c *Config) ApplyDefaults() {
	if c.Version == "" {
		c.Version = "3.8.0"
	}

	if c.ClientID == "" {
		c.ClientID = "yogan-kafka-client"
	}

	c.Producer.ApplyDefaults()
	c.Consumer.ApplyDefaults()
}

// ApplyDefaults 应用生产者默认值
func (c *ProducerConfig) ApplyDefaults() {
	if c.RequiredAcks == 0 && !c.Idempotent {
		c.RequiredAcks = 1 // WaitForLocal
	}

	if c.Timeout == 0 {
		c.Timeout = 10 * time.Second
	}

	if c.RetryMax == 0 {
		c.RetryMax = 3
	}

	if c.RetryBackoff == 0 {
		c.RetryBackoff = 100 * time.Millisecond
	}

	if c.MaxMessageBytes == 0 {
		c.MaxMessageBytes = 1048576 // 1MB
	}

	if c.Compression == "" {
		c.Compression = "none"
	}

	if c.BatchSize == 0 {
		c.BatchSize = 100
	}

	if c.FlushFrequency == 0 {
		c.FlushFrequency = 100 * time.Millisecond
	}
}

// ApplyDefaults 应用消费者默认值
func (c *ConsumerConfig) ApplyDefaults() {
	if c.OffsetInitial == 0 {
		c.OffsetInitial = -1 // Newest
	}

	if c.AutoCommitInterval == 0 {
		c.AutoCommitInterval = 1 * time.Second
	}

	if c.SessionTimeout == 0 {
		c.SessionTimeout = 10 * time.Second
	}

	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 3 * time.Second
	}

	if c.MaxProcessingTime == 0 {
		c.MaxProcessingTime = 100 * time.Millisecond
	}

	if c.FetchMin == 0 {
		c.FetchMin = 1
	}

	if c.FetchMax == 0 {
		c.FetchMax = 10485760 // 10MB
	}

	if c.FetchDefault == 0 {
		c.FetchDefault = 1048576 // 1MB
	}

	if c.RebalanceStrategy == "" {
		c.RebalanceStrategy = "range"
	}
}

