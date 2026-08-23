package queue

import (
	"fmt"
	"sync"

	"github.com/hibiken/asynq"
)

type BrokerManager struct {
	cfg Config

	mu         sync.Mutex
	clients    map[string]*asynq.Client
	inspectors map[string]*asynq.Inspector
}

func NewBrokerManager(cfg Config) (*BrokerManager, error) {
	cfg.ApplyDefaults()
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return nil, nil
	}
	return &BrokerManager{
		cfg:        cfg,
		clients:    map[string]*asynq.Client{},
		inspectors: map[string]*asynq.Inspector{},
	}, nil
}

func (m *BrokerManager) Client(brokerName string) (*asynq.Client, error) {
	if m == nil {
		return nil, ErrDisabled
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if client := m.clients[brokerName]; client != nil {
		return client, nil
	}
	broker, ok := m.cfg.Brokers[brokerName]
	if !ok {
		return nil, fmt.Errorf("queue broker %q is not configured", brokerName)
	}
	client := asynq.NewClient(redisClientOpt(broker.Redis))
	m.clients[brokerName] = client
	return client, nil
}

func (m *BrokerManager) Inspector(brokerName string) (*asynq.Inspector, error) {
	if m == nil {
		return nil, ErrDisabled
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if inspector := m.inspectors[brokerName]; inspector != nil {
		return inspector, nil
	}
	broker, ok := m.cfg.Brokers[brokerName]
	if !ok {
		return nil, fmt.Errorf("queue broker %q is not configured", brokerName)
	}
	inspector := asynq.NewInspector(redisClientOpt(broker.Redis))
	m.inspectors[brokerName] = inspector
	return inspector, nil
}

func (m *BrokerManager) Close() error {
	if m == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	var firstErr error
	for name, client := range m.clients {
		if client == nil {
			continue
		}
		if err := client.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close queue broker %q client: %w", name, err)
		}
	}
	for name, inspector := range m.inspectors {
		if inspector == nil {
			continue
		}
		if err := inspector.Close(); err != nil && firstErr == nil {
			firstErr = fmt.Errorf("close queue broker %q inspector: %w", name, err)
		}
	}
	m.clients = map[string]*asynq.Client{}
	m.inspectors = map[string]*asynq.Inspector{}
	return firstErr
}
