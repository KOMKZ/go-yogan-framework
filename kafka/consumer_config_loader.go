package kafka

import (
	"context"
	"time"

	"github.com/spf13/viper"
)

// ConsumerConfigEntry 单个消费者配置项
type ConsumerConfigEntry struct {
	// Topics 订阅的 Topic 列表
	Topics []string `mapstructure:"topics"`

	// GroupID 消费者组 ID
	GroupID string `mapstructure:"group_id"`

	// Workers 并发消费者数量
	Workers int `mapstructure:"workers"`

	// OffsetInitial 初始 Offset：-1=Newest, -2=Oldest
	OffsetInitial int64 `mapstructure:"offset_initial"`

	// AutoCommit 是否自动提交
	AutoCommit bool `mapstructure:"auto_commit"`

	// AutoCommitInterval 自动提交间隔
	AutoCommitInterval time.Duration `mapstructure:"auto_commit_interval"`

	// MaxProcessingTime 单条消息最大处理时间
	MaxProcessingTime time.Duration `mapstructure:"max_processing_time"`

	// SessionTimeout 会话超时
	SessionTimeout time.Duration `mapstructure:"session_timeout"`

	// HeartbeatInterval 心跳间隔
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
}

// ConsumersConfig 消费者配置映射
type ConsumersConfig map[string]ConsumerConfigEntry

// ConfigLoader 配置加载器接口
type ConfigLoader interface {
	GetStringSlice(key string) []string
	GetString(key string) string
	GetInt(key string) int
	GetInt64(key string) int64
	GetBool(key string) bool
	GetDuration(key string) time.Duration
	IsSet(key string) bool
	Sub(key string) *viper.Viper
}

// LoadConsumerRunnerConfig 从配置加载器读取消费者运行配置
// 配置路径: kafka.consumers.<name>
func LoadConsumerRunnerConfig(loader ConfigLoader, name string) ConsumerRunnerConfig {
	prefix := "kafka.consumers." + name

	cfg := ConsumerRunnerConfig{}

	if loader.IsSet(prefix + ".group_id") {
		cfg.GroupID = loader.GetString(prefix + ".group_id")
	}

	if loader.IsSet(prefix + ".workers") {
		cfg.Workers = loader.GetInt(prefix + ".workers")
	}

	if loader.IsSet(prefix + ".offset_initial") {
		cfg.OffsetInitial = loader.GetInt64(prefix + ".offset_initial")
	}

	if loader.IsSet(prefix + ".auto_commit") {
		cfg.AutoCommit = loader.GetBool(prefix + ".auto_commit")
	} else {
		cfg.AutoCommit = true // 默认开启
	}

	if loader.IsSet(prefix + ".auto_commit_interval") {
		cfg.AutoCommitInterval = loader.GetDuration(prefix + ".auto_commit_interval")
	}

	if loader.IsSet(prefix + ".max_processing_time") {
		cfg.MaxProcessingTime = loader.GetDuration(prefix + ".max_processing_time")
	}

	if loader.IsSet(prefix + ".session_timeout") {
		cfg.SessionTimeout = loader.GetDuration(prefix + ".session_timeout")
	}

	if loader.IsSet(prefix + ".heartbeat_interval") {
		cfg.HeartbeatInterval = loader.GetDuration(prefix + ".heartbeat_interval")
	}

	return cfg
}

// LoadConsumerTopics 从配置加载消费者订阅的 Topics
// 如果配置中定义了 topics，则使用配置中的值
// 否则使用 handler 中定义的值
func LoadConsumerTopics(loader ConfigLoader, name string) []string {
	prefix := "kafka.consumers." + name + ".topics"
	if loader.IsSet(prefix) {
		return loader.GetStringSlice(prefix)
	}
	return nil
}

// MergeConfigWithHandler 合并配置和 Handler 定义
// Handler 中的 Topics 可被配置覆盖
func MergeConfigWithHandler(handler ConsumerHandler, loader ConfigLoader) ([]string, ConsumerRunnerConfig) {
	name := handler.Name()

	// 加载配置
	cfg := LoadConsumerRunnerConfig(loader, name)

	// Topics：配置优先，Handler 兜底
	topics := LoadConsumerTopics(loader, name)
	if len(topics) == 0 {
		topics = handler.Topics()
	}

	return topics, cfg
}

// ConsumerConfigOverride 消费者配置覆盖器
// 用于在运行时覆盖 Handler 的 Topics
type ConsumerConfigOverride struct {
	handler ConsumerHandler
	topics  []string
}

// NewConsumerConfigOverride 创建配置覆盖器
func NewConsumerConfigOverride(handler ConsumerHandler, topics []string) *ConsumerConfigOverride {
	return &ConsumerConfigOverride{
		handler: handler,
		topics:  topics,
	}
}

// Name 返回消费者名称
func (o *ConsumerConfigOverride) Name() string {
	return o.handler.Name()
}

// Topics 返回覆盖后的 Topics
func (o *ConsumerConfigOverride) Topics() []string {
	if len(o.topics) > 0 {
		return o.topics
	}
	return o.handler.Topics()
}

// Handle 代理调用原始 Handler
func (o *ConsumerConfigOverride) Handle(ctx context.Context, msg *ConsumedMessage) error {
	return o.handler.Handle(ctx, msg)
}

// Ensure ConsumerConfigOverride implements ConsumerHandler
var _ ConsumerHandler = (*ConsumerConfigOverride)(nil)
