package kafka

import (
	"context"
	"time"

	"github.com/spf13/viper"
)

// ConsumerConfigEntry individual consumer configuration item
type ConsumerConfigEntry struct {
	// Topics subscription list for Topic names
	Topics []string `mapstructure:"topics"`

	// GroupID consumer group ID
	GroupID string `mapstructure:"group_id"`

	// Number of concurrent consumer workers
	Workers int `mapstructure:"workers"`

	// OffsetInitial Initial Offset: -1=Newest, -2=Oldest
	OffsetInitial int64 `mapstructure:"offset_initial"`

	// Whether auto-commit is enabled
	AutoCommit bool `mapstructure:"auto_commit"`

	// AutoCommitInterval automatic commit interval
	AutoCommitInterval time.Duration `mapstructure:"auto_commit_interval"`

	// Maximum processing time for individual messages
	MaxProcessingTime time.Duration `mapstructure:"max_processing_time"`

	// Session timeout
	SessionTimeout time.Duration `mapstructure:"session_timeout"`

	// HeartbeatInterval heartbeat interval
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`
}

// ConsumersConfig consumer configuration mapping
type ConsumersConfig map[string]ConsumerConfigEntry

// ConfigLoader configuration loader interface
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

// Load consumer runner configuration from the config loader
// Configuration path: kafka consumers.<name>
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
		cfg.AutoCommit = true // Default enabled
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

// LoadConsumerTopics loads the Topics subscribed by consumers from the configuration
// If topics are defined in the configuration, use the values from the configuration
// Otherwise use the value defined in the handler
func LoadConsumerTopics(loader ConfigLoader, name string) []string {
	prefix := "kafka.consumers." + name + ".topics"
	if loader.IsSet(prefix) {
		return loader.GetStringSlice(prefix)
	}
	return nil
}

// MergeConfigWithHandler merges configuration with Handler definition
// The Topics in the Handler can be configuration-overridden
func MergeConfigWithHandler(handler ConsumerHandler, loader ConfigLoader) ([]string, ConsumerRunnerConfig) {
	name := handler.Name()

	// Load configuration
	cfg := LoadConsumerRunnerConfig(loader, name)

	// Topics: Configuration priority, Handler as fallback
	topics := LoadConsumerTopics(loader, name)
	if len(topics) == 0 {
		topics = handler.Topics()
	}

	return topics, cfg
}

// ConsumerConfigOverride Consumer configuration override
// For runtime override of Handler's Topics
type ConsumerConfigOverride struct {
	handler ConsumerHandler
	topics  []string
}

// Create configuration override for new consumer config
func NewConsumerConfigOverride(handler ConsumerHandler, topics []string) *ConsumerConfigOverride {
	return &ConsumerConfigOverride{
		handler: handler,
		topics:  topics,
	}
}

// Returns the consumer name
func (o *ConsumerConfigOverride) Name() string {
	return o.handler.Name()
}

// Topics return the updated Topics
func (o *ConsumerConfigOverride) Topics() []string {
	if len(o.topics) > 0 {
		return o.topics
	}
	return o.handler.Topics()
}

// Handle proxy call to original Handler
func (o *ConsumerConfigOverride) Handle(ctx context.Context, msg *ConsumedMessage) error {
	return o.handler.Handle(ctx, msg)
}

// Ensure ConsumerConfigOverride implements ConsumerHandler
var _ ConsumerHandler = (*ConsumerConfigOverride)(nil)
