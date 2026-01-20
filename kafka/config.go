package kafka

import (
	"fmt"
	"time"
)

// Configure Kafka settings
type Config struct {
	// List of Kafka cluster addresses for brokers
	Brokers []string `mapstructure:"brokers"`

	// Kafka version (e.g., "3.8.0")
	Version string `mapstructure:"version"`

	// ClientID client identifier
	ClientID string `mapstructure:"client_id"`

	// Producer configuration
	Producer ProducerConfig `mapstructure:"producer"`

	// Consumer configuration
	Consumer ConsumerConfig `mapstructure:"consumer"`

	// SASL authentication configuration (optional)
	SASL *SASLConfig `mapstructure:"sasl"`

	// TLS configuration (optional)
	TLS *TLSConfig `mapstructure:"tls"`
}

// ProducerConfig producer configuration
type ProducerConfig struct {
	// Enabled whether the producer is activated
	Enabled bool `mapstructure:"enabled"`

	// RequiredAcks acknowledgment level: 0=NoResponse, 1=WaitForLocal, -1=WaitForAll
	RequiredAcks int `mapstructure:"required_acks"`

	// Timeout production timeout duration
	Timeout time.Duration `mapstructure:"timeout"`

	// Maximum number of retry attempts
	RetryMax int `mapstructure:"retry_max"`

	// RetryBackoff retry interval
	RetryBackoff time.Duration `mapstructure:"retry_backoff"`

	// Maximum message byte size for a single message
	MaxMessageBytes int `mapstructure:"max_message_bytes"`

	// Compression algorithm: none, gzip, snappy, lz4, zstd
	Compression string `mapstructure:"compression"`

	// Whether the idempotent producer is enabled
	Idempotent bool `mapstructure:"idempotent"`

	// BatchSize batch sending size
	BatchSize int `mapstructure:"batch_size"`

	// FlushFrequency flush frequency
	FlushFrequency time.Duration `mapstructure:"flush_frequency"`
}

// ConsumerConfig consumer configuration
type ConsumerConfig struct {
	// Enabled whether the consumer is activated
	Enabled bool `mapstructure:"enabled"`

	// GroupID consumer group ID
	GroupID string `mapstructure:"group_id"`

	// List of Topics subscribed to by Topics
	Topics []string `mapstructure:"topics"`

	// OffsetInitial Initial Offset: -1=Newest, -2=Oldest
	OffsetInitial int64 `mapstructure:"offset_initial"`

	// Whether auto-commit of Offset is enabled
	AutoCommit bool `mapstructure:"auto_commit"`

	// AutoCommitInterval auto commit interval
	AutoCommitInterval time.Duration `mapstructure:"auto_commit_interval"`

	// Session timeout
	SessionTimeout time.Duration `mapstructure:"session_timeout"`

	// HeartbeatInterval heartbeat interval
	HeartbeatInterval time.Duration `mapstructure:"heartbeat_interval"`

	// Maximum processing time for individual message
	MaxProcessingTime time.Duration `mapstructure:"max_processing_time"`

	// FetchMin minimum fetch byte count
	FetchMin int32 `mapstructure:"fetch_min"`

	// FetchMax maximum fetch byte count
	FetchMax int32 `mapstructure:"fetch_max"`

	// FetchDefault: Default bytes to fetch
	FetchDefault int32 `mapstructure:"fetch_default"`

	// RebalanceStrategy rebalancing strategy: range, roundrobin, sticky
	RebalanceStrategy string `mapstructure:"rebalance_strategy"`
}

// SASLConfig SASL authentication configuration
type SASLConfig struct {
	// Enabled whether to enable
	Enabled bool `mapstructure:"enabled"`

	// Authentication mechanism: PLAIN, SCRAM-SHA-256, SCRAM-SHA-512
	Mechanism string `mapstructure:"mechanism"`

	// Username
	Username string `mapstructure:"username"`

	// PasswordPASSWORD
	Password string `mapstructure:"password"`
}

// TLS configuration
type TLSConfig struct {
	// Enabled whether TLS is enabled
	Enabled bool `mapstructure:"enabled"`

	// CertFile certificate file path
	CertFile string `mapstructure:"cert_file"`

	// Path of the key file
	KeyFile string `mapstructure:"key_file"`

	// CA file path for the CA certificate
	CAFile string `mapstructure:"ca_file"`

	// Whether to skip certificate verification
	InsecureSkipVerify bool `mapstructure:"insecure_skip_verify"`
}

// Validate configuration
func (c *Config) Validate() error {
	if len(c.Brokers) == 0 {
		return fmt.Errorf("brokers cannot be empty")
	}

	for _, broker := range c.Brokers {
		if broker == "" {
			return fmt.Errorf("broker address cannot be empty")
		}
	}

	// Validate producer configuration
	if c.Producer.Enabled {
		if err := c.Producer.Validate(); err != nil {
			return fmt.Errorf("producer config invalid: %w", err)
		}
	}

	// Validate consumer configuration
	if c.Consumer.Enabled {
		if err := c.Consumer.Validate(); err != nil {
			return fmt.Errorf("consumer config invalid: %w", err)
		}
	}

	// Validate SASL configuration
	if c.SASL != nil && c.SASL.Enabled {
		if err := c.SASL.Validate(); err != nil {
			return fmt.Errorf("sasl config invalid: %w", err)
		}
	}

	return nil
}

// Validate producer configuration
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

// Validate consumer configuration
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

// Validate SASL configuration
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

// ApplyDefaults Apply default values
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

// Apply defaults for producer values
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

// Apply defaults for consumer values
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

