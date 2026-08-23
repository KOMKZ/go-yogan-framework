package queue

import (
	"fmt"
	"time"
)

const (
	DriverAsynq    = "asynq"
	DefaultBroker  = "default"
	DefaultQueue   = "default"
	DefaultProfile = "default"
)

// Config describes the framework-level queue component.
type Config struct {
	Enabled         bool                     `mapstructure:"enabled"`
	Driver          string                   `mapstructure:"driver"`
	Brokers         map[string]BrokerConfig  `mapstructure:"brokers"`
	Queues          map[string]QueueConfig   `mapstructure:"queues"`
	DefaultTask     TaskConfig               `mapstructure:"default_task"`
	Tasks           map[string]TaskConfig    `mapstructure:"tasks"`
	Profiles        map[string]ProfileConfig `mapstructure:"profiles"`
	ShutdownTimeout time.Duration            `mapstructure:"shutdown_timeout"`
}

type RedisConfig struct {
	Mode         string        `mapstructure:"mode"`
	Addr         string        `mapstructure:"addr"`
	Username     string        `mapstructure:"username"`
	Password     string        `mapstructure:"password"`
	DB           int           `mapstructure:"db"`
	DialTimeout  time.Duration `mapstructure:"dial_timeout"`
	ReadTimeout  time.Duration `mapstructure:"read_timeout"`
	WriteTimeout time.Duration `mapstructure:"write_timeout"`
	PoolSize     int           `mapstructure:"pool_size"`
}

type TaskConfig struct {
	Queue     string        `mapstructure:"queue"`
	Mode      string        `mapstructure:"mode"`
	Timeout   time.Duration `mapstructure:"timeout"`
	Retention time.Duration `mapstructure:"retention"`
	UniqueTTL time.Duration `mapstructure:"unique_ttl"`
}

type ProfileConfig struct {
	Workers map[string]ProfileWorkerConfig `mapstructure:"workers"`
}

func DefaultConfig() Config {
	return Config{
		Enabled: true,
		Driver:  DriverAsynq,
		Brokers: map[string]BrokerConfig{
			DefaultBroker: {
				Redis: RedisConfig{
					Mode: "dedicated",
					Addr: "127.0.0.1:6379",
				},
			},
		},
		Queues: map[string]QueueConfig{
			DefaultQueue: {
				Broker: DefaultBroker,
				Name:   DefaultQueue,
			},
		},
		DefaultTask: TaskConfig{
			Queue:   DefaultQueue,
			Mode:    ModeQueue,
			Timeout: 5 * time.Minute,
		},
		Profiles: map[string]ProfileConfig{
			DefaultProfile: {
				Workers: map[string]ProfileWorkerConfig{
					DefaultBroker: {
						Concurrency: 5,
						Queues: map[string]int{
							DefaultQueue: 1,
						},
					},
				},
			},
		},
		ShutdownTimeout: 10 * time.Second,
	}
}

func (c *Config) ApplyDefaults() {
	defaults := DefaultConfig()
	if c.Driver == "" {
		c.Driver = defaults.Driver
	}
	c.applyBrokerDefaults()
	c.applyQueueDefaults()
	if c.DefaultTask.Queue == "" {
		c.DefaultTask.Queue = defaults.DefaultTask.Queue
	}
	if c.DefaultTask.Mode == "" {
		c.DefaultTask.Mode = defaults.DefaultTask.Mode
	}
	if c.DefaultTask.Timeout == 0 {
		c.DefaultTask.Timeout = defaults.DefaultTask.Timeout
	}
	if c.ShutdownTimeout == 0 {
		c.ShutdownTimeout = defaults.ShutdownTimeout
	}
	if c.Tasks == nil {
		c.Tasks = map[string]TaskConfig{}
	}
	if len(c.Profiles) == 0 {
		c.Profiles = defaults.Profiles
	}
}

func (c Config) Validate() error {
	if !c.Enabled {
		return nil
	}
	if c.Driver != DriverAsynq {
		return fmt.Errorf("unsupported queue driver %q", c.Driver)
	}
	for name, broker := range c.Brokers {
		if err := broker.Validate(name); err != nil {
			return err
		}
	}
	if c.DefaultTask.Mode != "" && c.DefaultTask.Mode != ModeQueue && c.DefaultTask.Mode != ModeSync {
		return fmt.Errorf("queue default task mode must be %q or %q", ModeQueue, ModeSync)
	}
	if len(c.Profiles) == 0 {
		return fmt.Errorf("queue profiles are required")
	}
	for name := range c.Profiles {
		if _, err := c.ProfileWorkers(name); err != nil {
			return err
		}
	}
	for taskType, task := range c.Tasks {
		if task.Mode != "" && task.Mode != ModeQueue && task.Mode != ModeSync {
			return fmt.Errorf("queue task %q mode must be %q or %q", taskType, ModeQueue, ModeSync)
		}
	}
	for queueName, queue := range c.Queues {
		if err := queue.Validate(queueName, c.Brokers); err != nil {
			return err
		}
	}
	return nil
}

func (c Config) TaskConfig(taskType string) TaskConfig {
	taskCfg := c.DefaultTask
	if configured, ok := c.Tasks[taskType]; ok {
		taskCfg = mergeTaskConfig(taskCfg, configured)
	}
	return taskCfg
}

func (c Config) Profile(name string) (ProfileConfig, error) {
	if name == "" {
		name = DefaultProfile
	}
	profile, ok := c.Profiles[name]
	if !ok {
		return ProfileConfig{}, fmt.Errorf("queue profile %q is not configured", name)
	}
	return profile, nil
}

func mergeTaskConfig(base TaskConfig, override TaskConfig) TaskConfig {
	if override.Queue != "" {
		base.Queue = override.Queue
	}
	if override.Mode != "" {
		base.Mode = override.Mode
	}
	if override.Timeout != 0 {
		base.Timeout = override.Timeout
	}
	if override.Retention != 0 {
		base.Retention = override.Retention
	}
	if override.UniqueTTL != 0 {
		base.UniqueTTL = override.UniqueTTL
	}
	return base
}
