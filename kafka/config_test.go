package kafka

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config with producer",
			config: Config{
				Brokers: []string{"localhost:9092"},
				Producer: ProducerConfig{
					Enabled:      true,
					RequiredAcks: 1,
				},
			},
			wantErr: false,
		},
		{
			name: "valid config with consumer",
			config: Config{
				Brokers: []string{"localhost:9092"},
				Consumer: ConsumerConfig{
					Enabled: true,
					GroupID: "test-group",
					Topics:  []string{"test-topic"},
				},
			},
			wantErr: false,
		},
		{
			name: "empty brokers",
			config: Config{
				Brokers: []string{},
			},
			wantErr: true,
			errMsg:  "brokers cannot be empty",
		},
		{
			name: "empty broker address",
			config: Config{
				Brokers: []string{"localhost:9092", ""},
			},
			wantErr: true,
			errMsg:  "broker address cannot be empty",
		},
		{
			name: "invalid producer config",
			config: Config{
				Brokers: []string{"localhost:9092"},
				Producer: ProducerConfig{
					Enabled:      true,
					RequiredAcks: 5, // invalid
				},
			},
			wantErr: true,
			errMsg:  "producer config invalid",
		},
		{
			name: "invalid consumer config",
			config: Config{
				Brokers: []string{"localhost:9092"},
				Consumer: ConsumerConfig{
					Enabled: true,
					GroupID: "", // empty
					Topics:  []string{"test"},
				},
			},
			wantErr: true,
			errMsg:  "consumer config invalid",
		},
		{
			name: "invalid sasl config",
			config: Config{
				Brokers: []string{"localhost:9092"},
				SASL: &SASLConfig{
					Enabled:   true,
					Mechanism: "INVALID",
					Username:  "user",
					Password:  "pass",
				},
			},
			wantErr: true,
			errMsg:  "sasl config invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProducerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ProducerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: ProducerConfig{
				RequiredAcks:    1,
				MaxMessageBytes: 1048576,
				Compression:     "gzip",
			},
			wantErr: false,
		},
		{
			name: "valid acks -1",
			config: ProducerConfig{
				RequiredAcks: -1,
			},
			wantErr: false,
		},
		{
			name: "valid acks 0",
			config: ProducerConfig{
				RequiredAcks: 0,
			},
			wantErr: false,
		},
		{
			name: "invalid acks",
			config: ProducerConfig{
				RequiredAcks: 2,
			},
			wantErr: true,
			errMsg:  "required_acks must be -1, 0, or 1",
		},
		{
			name: "negative max message bytes",
			config: ProducerConfig{
				RequiredAcks:    1,
				MaxMessageBytes: -1,
			},
			wantErr: true,
			errMsg:  "max_message_bytes must be >= 0",
		},
		{
			name: "invalid compression",
			config: ProducerConfig{
				RequiredAcks: 1,
				Compression:  "invalid",
			},
			wantErr: true,
			errMsg:  "invalid compression",
		},
		{
			name: "valid compression snappy",
			config: ProducerConfig{
				RequiredAcks: 1,
				Compression:  "snappy",
			},
			wantErr: false,
		},
		{
			name: "valid compression lz4",
			config: ProducerConfig{
				RequiredAcks: 1,
				Compression:  "lz4",
			},
			wantErr: false,
		},
		{
			name: "valid compression zstd",
			config: ProducerConfig{
				RequiredAcks: 1,
				Compression:  "zstd",
			},
			wantErr: false,
		},
		{
			name: "valid compression none",
			config: ProducerConfig{
				RequiredAcks: 1,
				Compression:  "none",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConsumerConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ConsumerConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: ConsumerConfig{
				GroupID: "test-group",
				Topics:  []string{"topic1", "topic2"},
			},
			wantErr: false,
		},
		{
			name: "empty group id",
			config: ConsumerConfig{
				GroupID: "",
				Topics:  []string{"topic1"},
			},
			wantErr: true,
			errMsg:  "group_id cannot be empty",
		},
		{
			name: "empty topics",
			config: ConsumerConfig{
				GroupID: "test-group",
				Topics:  []string{},
			},
			wantErr: true,
			errMsg:  "topics cannot be empty",
		},
		{
			name: "empty topic name",
			config: ConsumerConfig{
				GroupID: "test-group",
				Topics:  []string{"topic1", ""},
			},
			wantErr: true,
			errMsg:  "topic name cannot be empty",
		},
		{
			name: "invalid rebalance strategy",
			config: ConsumerConfig{
				GroupID:           "test-group",
				Topics:            []string{"topic1"},
				RebalanceStrategy: "invalid",
			},
			wantErr: true,
			errMsg:  "invalid rebalance_strategy",
		},
		{
			name: "valid rebalance strategy range",
			config: ConsumerConfig{
				GroupID:           "test-group",
				Topics:            []string{"topic1"},
				RebalanceStrategy: "range",
			},
			wantErr: false,
		},
		{
			name: "valid rebalance strategy roundrobin",
			config: ConsumerConfig{
				GroupID:           "test-group",
				Topics:            []string{"topic1"},
				RebalanceStrategy: "roundrobin",
			},
			wantErr: false,
		},
		{
			name: "valid rebalance strategy sticky",
			config: ConsumerConfig{
				GroupID:           "test-group",
				Topics:            []string{"topic1"},
				RebalanceStrategy: "sticky",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSASLConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  SASLConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid PLAIN",
			config: SASLConfig{
				Enabled:   true,
				Mechanism: "PLAIN",
				Username:  "user",
				Password:  "pass",
			},
			wantErr: false,
		},
		{
			name: "valid SCRAM-SHA-256",
			config: SASLConfig{
				Enabled:   true,
				Mechanism: "SCRAM-SHA-256",
				Username:  "user",
				Password:  "pass",
			},
			wantErr: false,
		},
		{
			name: "valid SCRAM-SHA-512",
			config: SASLConfig{
				Enabled:   true,
				Mechanism: "SCRAM-SHA-512",
				Username:  "user",
				Password:  "pass",
			},
			wantErr: false,
		},
		{
			name: "empty username",
			config: SASLConfig{
				Enabled:   true,
				Mechanism: "PLAIN",
				Username:  "",
				Password:  "pass",
			},
			wantErr: true,
			errMsg:  "username cannot be empty",
		},
		{
			name: "empty password",
			config: SASLConfig{
				Enabled:   true,
				Mechanism: "PLAIN",
				Username:  "user",
				Password:  "",
			},
			wantErr: true,
			errMsg:  "password cannot be empty",
		},
		{
			name: "invalid mechanism",
			config: SASLConfig{
				Enabled:   true,
				Mechanism: "INVALID",
				Username:  "user",
				Password:  "pass",
			},
			wantErr: true,
			errMsg:  "invalid mechanism",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	cfg := Config{}
	cfg.ApplyDefaults()

	assert.Equal(t, "3.8.0", cfg.Version)
	assert.Equal(t, "yogan-kafka-client", cfg.ClientID)
}

func TestProducerConfig_ApplyDefaults(t *testing.T) {
	cfg := ProducerConfig{}
	cfg.ApplyDefaults()

	assert.Equal(t, 1, cfg.RequiredAcks)
	assert.Equal(t, 10*time.Second, cfg.Timeout)
	assert.Equal(t, 3, cfg.RetryMax)
	assert.Equal(t, 100*time.Millisecond, cfg.RetryBackoff)
	assert.Equal(t, 1048576, cfg.MaxMessageBytes)
	assert.Equal(t, "none", cfg.Compression)
	assert.Equal(t, 100, cfg.BatchSize)
	assert.Equal(t, 100*time.Millisecond, cfg.FlushFrequency)
}

func TestProducerConfig_ApplyDefaults_Idempotent(t *testing.T) {
	cfg := ProducerConfig{
		Idempotent: true,
	}
	cfg.ApplyDefaults()

	// Idempotent mode RequiredAcks remains 0 (will be set to -1 in sarama)
	assert.Equal(t, 0, cfg.RequiredAcks)
}

func TestConsumerConfig_ApplyDefaults(t *testing.T) {
	cfg := ConsumerConfig{}
	cfg.ApplyDefaults()

	assert.Equal(t, int64(-1), cfg.OffsetInitial)
	assert.Equal(t, 1*time.Second, cfg.AutoCommitInterval)
	assert.Equal(t, 10*time.Second, cfg.SessionTimeout)
	assert.Equal(t, 3*time.Second, cfg.HeartbeatInterval)
	assert.Equal(t, 100*time.Millisecond, cfg.MaxProcessingTime)
	assert.Equal(t, int32(1), cfg.FetchMin)
	assert.Equal(t, int32(10485760), cfg.FetchMax)
	assert.Equal(t, int32(1048576), cfg.FetchDefault)
	assert.Equal(t, "range", cfg.RebalanceStrategy)
}

