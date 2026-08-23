package queue

import "fmt"

type BrokerConfig struct {
	Redis RedisConfig `mapstructure:"redis"`
}

type QueueConfig struct {
	Broker     string `mapstructure:"broker"`
	Name       string `mapstructure:"name"`
	MaxPending int64  `mapstructure:"max_pending"`
}

type ProfileWorkerConfig struct {
	Concurrency    int            `mapstructure:"concurrency"`
	Queues         map[string]int `mapstructure:"queues"`
	StrictPriority bool           `mapstructure:"strict_priority"`
}

type QueueRoute struct {
	LogicalQueue string
	Broker       string
	PhysicalName string
	MaxPending   int64
}

type BrokerWorker struct {
	Broker string
	ProfileWorkerConfig
}

func (c *Config) applyBrokerDefaults() {
	if len(c.Brokers) == 0 {
		c.Brokers = map[string]BrokerConfig{
			DefaultBroker: {
				Redis: RedisConfig{
					Mode: "dedicated",
					Addr: "127.0.0.1:6379",
				},
			},
		}
	}
	for name, broker := range c.Brokers {
		broker.Redis.ApplyDefaults()
		c.Brokers[name] = broker
	}
}

func (c *Config) applyQueueDefaults() {
	if len(c.Queues) == 0 {
		c.Queues = map[string]QueueConfig{
			DefaultQueue: {
				Broker: DefaultBroker,
				Name:   DefaultQueue,
			},
		}
	}
	for name, queue := range c.Queues {
		if queue.Name == "" {
			queue.Name = name
		}
		c.Queues[name] = queue
	}
}

func (b BrokerConfig) Validate(name string) error {
	if name == "" {
		return fmt.Errorf("queue broker name is required")
	}
	if err := b.Redis.Validate(); err != nil {
		return fmt.Errorf("queue broker %q redis config invalid: %w", name, err)
	}
	return nil
}

func (q QueueConfig) Validate(name string, brokers map[string]BrokerConfig) error {
	if name == "" {
		return fmt.Errorf("queue name is required")
	}
	if q.Broker == "" {
		return fmt.Errorf("queue %q broker is required", name)
	}
	if _, ok := brokers[q.Broker]; !ok {
		return fmt.Errorf("queue %q references unknown broker %q", name, q.Broker)
	}
	if q.MaxPending < 0 {
		return fmt.Errorf("queue %q max_pending must be greater than or equal to zero", name)
	}
	return nil
}

func (c Config) QueueRoute(queueName string) (QueueRoute, error) {
	queue, ok := c.Queues[queueName]
	if !ok {
		return QueueRoute{}, fmt.Errorf("queue %q is not configured", queueName)
	}
	physicalName := queue.Name
	if physicalName == "" {
		physicalName = queueName
	}
	return QueueRoute{
		LogicalQueue: queueName,
		Broker:       queue.Broker,
		PhysicalName: physicalName,
		MaxPending:   queue.MaxPending,
	}, nil
}

func (c Config) ProfileWorkers(profileName string) ([]BrokerWorker, error) {
	profile, err := c.Profile(profileName)
	if err != nil {
		return nil, err
	}
	if len(profile.Workers) == 0 {
		return nil, fmt.Errorf("queue profile %q workers are required", profileName)
	}
	workers := make([]BrokerWorker, 0, len(profile.Workers))
	for brokerName, worker := range profile.Workers {
		if _, ok := c.Brokers[brokerName]; !ok {
			return nil, fmt.Errorf("queue profile %q references unknown broker %q", profileName, brokerName)
		}
		if worker.Concurrency <= 0 {
			return nil, fmt.Errorf("queue profile %q broker %q concurrency must be greater than zero", profileName, brokerName)
		}
		if len(worker.Queues) == 0 {
			return nil, fmt.Errorf("queue profile %q broker %q queues are required", profileName, brokerName)
		}
		for logicalQueue := range worker.Queues {
			route, err := c.QueueRoute(logicalQueue)
			if err != nil {
				return nil, err
			}
			if route.Broker != brokerName {
				return nil, fmt.Errorf("queue profile %q broker %q cannot consume queue %q from broker %q", profileName, brokerName, logicalQueue, route.Broker)
			}
		}
		workers = append(workers, BrokerWorker{Broker: brokerName, ProfileWorkerConfig: worker})
	}
	return workers, nil
}
