package kafka

import (
	"context"
	"crypto/tls"
	"fmt"
	"sync"
	"time"

	"github.com/IBM/sarama"
	"go.uber.org/zap"
)

// Manager Kafka 管理器
type Manager struct {
	config       Config
	saramaConfig *sarama.Config
	logger       *zap.Logger

	client        sarama.Client    // Sarama 客户端
	producer      Producer
	asyncProducer *AsyncProducer
	consumers     map[string]*ConsumerGroup

	mu     sync.RWMutex
	closed bool
}

// NewManager 创建 Kafka 管理器
func NewManager(cfg Config, logger *zap.Logger) (*Manager, error) {
	if logger == nil {
		return nil, fmt.Errorf("logger cannot be nil")
	}

	// 应用默认值
	cfg.ApplyDefaults()

	// 验证配置
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("config validation failed: %w", err)
	}

	// 创建 Sarama 配置
	saramaCfg, err := buildSaramaConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("build sarama config failed: %w", err)
	}

	m := &Manager{
		config:       cfg,
		saramaConfig: saramaCfg,
		logger:       logger,
		consumers:    make(map[string]*ConsumerGroup),
	}

	return m, nil
}

// buildSaramaConfig 构建 Sarama 配置
func buildSaramaConfig(cfg Config) (*sarama.Config, error) {
	saramaCfg := sarama.NewConfig()

	// 解析版本
	version, err := sarama.ParseKafkaVersion(cfg.Version)
	if err != nil {
		return nil, fmt.Errorf("parse kafka version failed: %w", err)
	}
	saramaCfg.Version = version

	// 客户端 ID
	saramaCfg.ClientID = cfg.ClientID

	// 生产者配置
	if cfg.Producer.Enabled {
		saramaCfg.Producer.Return.Successes = true
		saramaCfg.Producer.Return.Errors = true

		switch cfg.Producer.RequiredAcks {
		case 0:
			saramaCfg.Producer.RequiredAcks = sarama.NoResponse
		case 1:
			saramaCfg.Producer.RequiredAcks = sarama.WaitForLocal
		case -1:
			saramaCfg.Producer.RequiredAcks = sarama.WaitForAll
		}

		saramaCfg.Producer.Timeout = cfg.Producer.Timeout
		saramaCfg.Producer.Retry.Max = cfg.Producer.RetryMax
		saramaCfg.Producer.Retry.Backoff = cfg.Producer.RetryBackoff
		saramaCfg.Producer.MaxMessageBytes = cfg.Producer.MaxMessageBytes
		saramaCfg.Producer.Idempotent = cfg.Producer.Idempotent

		// 压缩
		switch cfg.Producer.Compression {
		case "gzip":
			saramaCfg.Producer.Compression = sarama.CompressionGZIP
		case "snappy":
			saramaCfg.Producer.Compression = sarama.CompressionSnappy
		case "lz4":
			saramaCfg.Producer.Compression = sarama.CompressionLZ4
		case "zstd":
			saramaCfg.Producer.Compression = sarama.CompressionZSTD
		default:
			saramaCfg.Producer.Compression = sarama.CompressionNone
		}

		// 批量发送
		saramaCfg.Producer.Flush.Frequency = cfg.Producer.FlushFrequency
		saramaCfg.Producer.Flush.Messages = cfg.Producer.BatchSize
	}

	// 消费者配置
	if cfg.Consumer.Enabled {
		saramaCfg.Consumer.Return.Errors = true

		if cfg.Consumer.OffsetInitial == -2 {
			saramaCfg.Consumer.Offsets.Initial = sarama.OffsetOldest
		} else {
			saramaCfg.Consumer.Offsets.Initial = sarama.OffsetNewest
		}

		saramaCfg.Consumer.Offsets.AutoCommit.Enable = cfg.Consumer.AutoCommit
		saramaCfg.Consumer.Offsets.AutoCommit.Interval = cfg.Consumer.AutoCommitInterval

		saramaCfg.Consumer.Group.Session.Timeout = cfg.Consumer.SessionTimeout
		saramaCfg.Consumer.Group.Heartbeat.Interval = cfg.Consumer.HeartbeatInterval
		saramaCfg.Consumer.MaxProcessingTime = cfg.Consumer.MaxProcessingTime

		saramaCfg.Consumer.Fetch.Min = cfg.Consumer.FetchMin
		saramaCfg.Consumer.Fetch.Max = cfg.Consumer.FetchMax
		saramaCfg.Consumer.Fetch.Default = cfg.Consumer.FetchDefault

		// 再平衡策略
		switch cfg.Consumer.RebalanceStrategy {
		case "roundrobin":
			saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRoundRobin()}
		case "sticky":
			saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategySticky()}
		default:
			saramaCfg.Consumer.Group.Rebalance.GroupStrategies = []sarama.BalanceStrategy{sarama.NewBalanceStrategyRange()}
		}
	}

	// SASL 配置
	if cfg.SASL != nil && cfg.SASL.Enabled {
		saramaCfg.Net.SASL.Enable = true
		saramaCfg.Net.SASL.User = cfg.SASL.Username
		saramaCfg.Net.SASL.Password = cfg.SASL.Password

		switch cfg.SASL.Mechanism {
		case "SCRAM-SHA-256":
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA256}
			}
		case "SCRAM-SHA-512":
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			saramaCfg.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient {
				return &XDGSCRAMClient{HashGeneratorFcn: SHA512}
			}
		default:
			saramaCfg.Net.SASL.Mechanism = sarama.SASLTypePlaintext
		}
	}

	// TLS 配置
	if cfg.TLS != nil && cfg.TLS.Enabled {
		saramaCfg.Net.TLS.Enable = true
		saramaCfg.Net.TLS.Config = &tls.Config{
			InsecureSkipVerify: cfg.TLS.InsecureSkipVerify,
		}
	}

	return saramaCfg, nil
}

// Connect 连接 Kafka
func (m *Manager) Connect(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return fmt.Errorf("manager is closed")
	}

	// 测试连接
	if err := m.testConnection(); err != nil {
		return fmt.Errorf("test connection failed: %w", err)
	}

	// 创建生产者
	if m.config.Producer.Enabled {
		producer, err := NewSyncProducer(m.config.Brokers, m.config.Producer, m.saramaConfig, m.logger)
		if err != nil {
			return fmt.Errorf("create producer failed: %w", err)
		}
		m.producer = producer
		m.logger.Debug("producer created")
	}

	m.logger.Info("kafka manager connected")

	return nil
}

// testConnection 测试连接并保持客户端
func (m *Manager) testConnection() error {
	client, err := sarama.NewClient(m.config.Brokers, m.saramaConfig)
	if err != nil {
		return fmt.Errorf("create client failed: %w", err)
	}

	// 获取 Broker 列表验证连接
	brokers := client.Brokers()
	if len(brokers) == 0 {
		client.Close()
		return fmt.Errorf("no brokers available")
	}

	// 保存客户端引用
	m.client = client

	return nil
}

// GetProducer 获取生产者
func (m *Manager) GetProducer() Producer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.producer
}

// GetAsyncProducer 获取异步生产者（按需创建）
func (m *Manager) GetAsyncProducer() (*AsyncProducer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.asyncProducer != nil {
		return m.asyncProducer, nil
	}

	producer, err := NewAsyncProducer(m.config.Brokers, m.config.Producer, m.saramaConfig, m.logger)
	if err != nil {
		return nil, fmt.Errorf("create async producer failed: %w", err)
	}

	m.asyncProducer = producer
	return m.asyncProducer, nil
}

// CreateConsumer 创建消费者
func (m *Manager) CreateConsumer(name string, cfg ConsumerConfig) (*ConsumerGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil, fmt.Errorf("manager is closed")
	}

	if _, exists := m.consumers[name]; exists {
		return nil, fmt.Errorf("consumer %s already exists", name)
	}

	// 应用默认值并验证
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("consumer config invalid: %w", err)
	}

	// 为消费者创建新的 Sarama 配置
	consumerSaramaCfg := *m.saramaConfig
	consumerSaramaCfg.Consumer.Offsets.AutoCommit.Enable = cfg.AutoCommit
	consumerSaramaCfg.Consumer.Offsets.AutoCommit.Interval = cfg.AutoCommitInterval

	consumer, err := NewConsumerGroup(m.config.Brokers, cfg, &consumerSaramaCfg, m.logger)
	if err != nil {
		return nil, fmt.Errorf("create consumer group failed: %w", err)
	}

	m.consumers[name] = consumer
	return consumer, nil
}

// GetConsumer 获取消费者
func (m *Manager) GetConsumer(name string) *ConsumerGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.consumers[name]
}

// Ping 检查 Kafka 连接
func (m *Manager) Ping(ctx context.Context) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.closed {
		return fmt.Errorf("manager is closed")
	}

	client, err := sarama.NewClient(m.config.Brokers, m.saramaConfig)
	if err != nil {
		return fmt.Errorf("create client failed: %w", err)
	}
	defer client.Close()

	// 设置超时
	done := make(chan error, 1)
	go func() {
		// 检查 Controller 是否可用
		controller, err := client.Controller()
		if err != nil {
			done <- fmt.Errorf("get controller failed: %w", err)
			return
		}
		connected, err := controller.Connected()
		if err != nil {
			done <- fmt.Errorf("check controller connected failed: %w", err)
			return
		}
		if !connected {
			err := controller.Open(m.saramaConfig)
			if err != nil {
				done <- fmt.Errorf("connect to controller failed: %w", err)
				return
			}
		}
		done <- nil
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		return fmt.Errorf("ping timeout")
	}
}

// ListTopics 列出所有 Topic
func (m *Manager) ListTopics(ctx context.Context) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	client, err := sarama.NewClient(m.config.Brokers, m.saramaConfig)
	if err != nil {
		return nil, fmt.Errorf("create client failed: %w", err)
	}
	defer client.Close()

	topics, err := client.Topics()
	if err != nil {
		return nil, fmt.Errorf("list topics failed: %w", err)
	}

	return topics, nil
}

// GetConfig 获取配置
func (m *Manager) GetConfig() Config {
	return m.config
}

// Close 关闭管理器
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.closed {
		return nil
	}
	m.closed = true

	var errs []error

	// 关闭消费者
	for name, consumer := range m.consumers {
		if err := consumer.Stop(); err != nil {
			m.logger.Error("close consumer failed",
				zap.String("name", name),
				zap.Error(err))
			errs = append(errs, err)
		}
	}

	// 关闭异步生产者
	if m.asyncProducer != nil {
		if err := m.asyncProducer.Close(); err != nil {
			m.logger.Error("close async producer failed", zap.Error(err))
			errs = append(errs, err)
		}
	}

	// 关闭同步生产者
	if m.producer != nil {
		if err := m.producer.Close(); err != nil {
			m.logger.Error("close producer failed", zap.Error(err))
			errs = append(errs, err)
		}
	}

	// 关闭客户端
	if m.client != nil {
		if err := m.client.Close(); err != nil {
			m.logger.Error("close client failed", zap.Error(err))
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("close manager with %d errors", len(errs))
	}

	m.logger.Info("kafka manager closed")
	return nil
}

// Shutdown 实现 samber/do.Shutdownable 接口
func (m *Manager) Shutdown() error {
	return m.Close()
}

// ================== Topic 管理方法 ==================

// TopicInfo Topic 信息
type TopicInfo struct {
	NumPartitions     int32
	ReplicationFactor int16
	Partitions        []PartitionInfo
}

// PartitionInfo 分区信息
type PartitionInfo struct {
	ID       int32
	Leader   int32
	Replicas []int32
	ISR      []int32
}

// CreateTopic 创建 Topic
func (m *Manager) CreateTopic(ctx context.Context, name string, partitions int32, replication int16) error {
	if m.client == nil {
		return fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	topicDetail := &sarama.TopicDetail{
		NumPartitions:     partitions,
		ReplicationFactor: replication,
	}

	err = admin.CreateTopic(name, topicDetail, false)
	if err != nil {
		return fmt.Errorf("create topic failed: %w", err)
	}

	m.logger.Info("topic created",
		zap.String("topic", name),
		zap.Int32("partitions", partitions),
		zap.Int16("replication", replication))
	return nil
}

// DeleteTopic 删除 Topic
func (m *Manager) DeleteTopic(ctx context.Context, name string) error {
	if m.client == nil {
		return fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	err = admin.DeleteTopic(name)
	if err != nil {
		return fmt.Errorf("delete topic failed: %w", err)
	}

	m.logger.Info("topic deleted", zap.String("topic", name))
	return nil
}

// DescribeTopic 获取 Topic 详情
func (m *Manager) DescribeTopic(ctx context.Context, name string) (*TopicInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return nil, fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	metadata, err := admin.DescribeTopics([]string{name})
	if err != nil {
		return nil, fmt.Errorf("describe topic failed: %w", err)
	}

	if len(metadata) == 0 {
		return nil, fmt.Errorf("topic not found: %s", name)
	}

	topicMeta := metadata[0]
	if topicMeta.Err != sarama.ErrNoError {
		return nil, fmt.Errorf("topic error: %v", topicMeta.Err)
	}

	info := &TopicInfo{
		NumPartitions: int32(len(topicMeta.Partitions)),
		Partitions:    make([]PartitionInfo, len(topicMeta.Partitions)),
	}

	for i, p := range topicMeta.Partitions {
		info.Partitions[i] = PartitionInfo{
			ID:       p.ID,
			Leader:   p.Leader,
			Replicas: p.Replicas,
			ISR:      p.Isr,
		}
		if i == 0 {
			info.ReplicationFactor = int16(len(p.Replicas))
		}
	}

	return info, nil
}

// ================== Offset 管理方法 ==================

// ResetOffset 重置消费者组的 Offset
func (m *Manager) ResetOffset(ctx context.Context, groupID, topic string, offset int64) error {
	if m.client == nil {
		return fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	// 获取分区列表
	partitions, err := m.client.Partitions(topic)
	if err != nil {
		return fmt.Errorf("get partitions failed: %w", err)
	}

	// 构建 offset 映射
	offsets := make(map[string]map[int32]int64)
	offsets[topic] = make(map[int32]int64)

	for _, partition := range partitions {
		var targetOffset int64
		if offset == -2 { // earliest
			targetOffset, err = m.client.GetOffset(topic, partition, sarama.OffsetOldest)
		} else if offset == -1 { // latest
			targetOffset, err = m.client.GetOffset(topic, partition, sarama.OffsetNewest)
		} else {
			targetOffset = offset
		}
		if err != nil {
			return fmt.Errorf("get offset failed for partition %d: %w", partition, err)
		}
		offsets[topic][partition] = targetOffset
	}

	// 使用 offset commit 的方式重置
	offsetManager, err := sarama.NewOffsetManagerFromClient(groupID, m.client)
	if err != nil {
		return fmt.Errorf("create offset manager failed: %w", err)
	}
	defer offsetManager.Close()

	for _, partition := range partitions {
		pom, err := offsetManager.ManagePartition(topic, partition)
		if err != nil {
			return fmt.Errorf("manage partition %d failed: %w", partition, err)
		}
		pom.MarkOffset(offsets[topic][partition], "reset")
		pom.Close()
	}

	// 提交 offset
	offsetManager.Commit()

	m.logger.Info("offset reset",
		zap.String("group", groupID),
		zap.String("topic", topic),
		zap.Int64("offset", offset))
	return nil
}

// GetOffset 获取消费者组的 Offset
func (m *Manager) GetOffset(ctx context.Context, groupID, topic string) (map[int32]int64, error) {
	if m.client == nil {
		return nil, fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return nil, fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	// 获取分区列表
	partitions, err := m.client.Partitions(topic)
	if err != nil {
		return nil, fmt.Errorf("get partitions failed: %w", err)
	}

	// 构建查询映射
	topicPartitions := map[string][]int32{
		topic: partitions,
	}

	// 获取 offset
	response, err := admin.ListConsumerGroupOffsets(groupID, topicPartitions)
	if err != nil {
		return nil, fmt.Errorf("list offsets failed: %w", err)
	}

	result := make(map[int32]int64)
	if block, ok := response.Blocks[topic]; ok {
		for partition, offsetBlock := range block {
			result[partition] = offsetBlock.Offset
		}
	}

	return result, nil
}

// ================== Consumer Group 管理方法 ==================

// ConsumerGroupInfo 消费者组信息
type ConsumerGroupInfo struct {
	State        string
	ProtocolType string
	Members      []ConsumerGroupMember
}

// ConsumerGroupMember 消费者组成员
type ConsumerGroupMember struct {
	MemberID   string
	ClientID   string
	ClientHost string
}

// ListConsumerGroups 列出所有消费者组
func (m *Manager) ListConsumerGroups(ctx context.Context) ([]string, error) {
	if m.client == nil {
		return nil, fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return nil, fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	groups, err := admin.ListConsumerGroups()
	if err != nil {
		return nil, fmt.Errorf("list consumer groups failed: %w", err)
	}

	result := make([]string, 0, len(groups))
	for group := range groups {
		result = append(result, group)
	}

	return result, nil
}

// DescribeConsumerGroup 获取消费者组详情
func (m *Manager) DescribeConsumerGroup(ctx context.Context, groupID string) (*ConsumerGroupInfo, error) {
	if m.client == nil {
		return nil, fmt.Errorf("kafka client not connected")
	}

	admin, err := sarama.NewClusterAdminFromClient(m.client)
	if err != nil {
		return nil, fmt.Errorf("create admin client failed: %w", err)
	}
	defer admin.Close()

	groups, err := admin.DescribeConsumerGroups([]string{groupID})
	if err != nil {
		return nil, fmt.Errorf("describe consumer group failed: %w", err)
	}

	if len(groups) == 0 {
		return nil, fmt.Errorf("consumer group not found: %s", groupID)
	}

	group := groups[0]
	info := &ConsumerGroupInfo{
		State:        group.State,
		ProtocolType: group.ProtocolType,
		Members:      make([]ConsumerGroupMember, len(group.Members)),
	}

	idx := 0
	for _, member := range group.Members {
		info.Members[idx] = ConsumerGroupMember{
			MemberID:   member.MemberId,
			ClientID:   member.ClientId,
			ClientHost: member.ClientHost,
		}
		idx++
	}

	return info, nil
}

