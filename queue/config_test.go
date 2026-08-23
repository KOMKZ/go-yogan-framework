package queue

import (
	"testing"
	"time"
)

func TestConfigApplyDefaults(t *testing.T) {
	cfg := Config{}

	cfg.ApplyDefaults()

	if cfg.Driver != DriverAsynq {
		t.Fatalf("driver = %q, want %q", cfg.Driver, DriverAsynq)
	}
	if cfg.Brokers[DefaultBroker].Redis.Addr == "" {
		t.Fatal("default broker redis addr should have default")
	}
	if cfg.DefaultTask.Queue != DefaultQueue {
		t.Fatalf("default queue = %q, want %q", cfg.DefaultTask.Queue, DefaultQueue)
	}
	if cfg.DefaultTask.Timeout != 5*time.Minute {
		t.Fatalf("timeout = %v, want 5m", cfg.DefaultTask.Timeout)
	}
	if _, ok := cfg.Profiles[DefaultProfile]; !ok {
		t.Fatal("default profile should be configured")
	}
}

func TestConfigValidateRejectsUnsupportedDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Driver = "rabbitmq"

	if err := cfg.Validate(); err == nil {
		t.Fatal("expected unsupported driver error")
	}
}

func TestTaskConfigMergesSpecificOverride(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Tasks = map[string]TaskConfig{
		"export.demo": {
			Queue: "export",
			Mode:  ModeSync,
		},
	}

	taskCfg := cfg.TaskConfig("export.demo")

	if taskCfg.Queue != "export" {
		t.Fatalf("queue = %q, want export", taskCfg.Queue)
	}
	if taskCfg.Mode != ModeSync {
		t.Fatalf("mode = %q, want sync", taskCfg.Mode)
	}
	if taskCfg.Timeout != cfg.DefaultTask.Timeout {
		t.Fatalf("timeout = %v, want inherited %v", taskCfg.Timeout, cfg.DefaultTask.Timeout)
	}
}

func TestTaskDefinitionAppliesTaskConfig(t *testing.T) {
	def := TaskDefinition[struct{ Name string }]{
		Type:  "export.demo",
		Queue: "low",
		Mode:  ModeQueue,
	}

	got := def.ApplyTaskConfig(TaskConfig{
		Queue: "critical",
		Mode:  ModeSync,
	})

	if got.Queue != "critical" || got.Mode != ModeSync {
		t.Fatalf("definition not merged: %#v", got)
	}
}

func TestProfileUsesDefaultName(t *testing.T) {
	cfg := DefaultConfig()

	profile, err := cfg.Profile("")
	if err != nil {
		t.Fatalf("profile default failed: %v", err)
	}
	if profile.Workers[DefaultBroker].Concurrency <= 0 {
		t.Fatal("profile worker concurrency should be positive")
	}
}

func TestQueueRouteUsesConfiguredBrokerAndPhysicalQueue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers["media"] = BrokerConfig{Redis: RedisConfig{Mode: "dedicated", Addr: "127.0.0.1:6380"}}
	cfg.Queues["media"] = QueueConfig{
		Broker:     "media",
		Name:       "default",
		MaxPending: 10,
	}

	route, err := cfg.QueueRoute("media")
	if err != nil {
		t.Fatalf("route failed: %v", err)
	}
	if route.Broker != "media" || route.PhysicalName != "default" || route.MaxPending != 10 {
		t.Fatalf("route = %+v", route)
	}
}

func TestProfileWorkersRejectsCrossBrokerQueue(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Brokers["media"] = BrokerConfig{Redis: RedisConfig{Mode: "dedicated", Addr: "127.0.0.1:6380"}}
	cfg.Queues["media"] = QueueConfig{Broker: "media", Name: "default"}
	cfg.Profiles["bad"] = ProfileConfig{
		Workers: map[string]ProfileWorkerConfig{
			DefaultBroker: {
				Concurrency: 1,
				Queues:      map[string]int{"media": 1},
			},
		},
	}

	if _, err := cfg.ProfileWorkers("bad"); err == nil {
		t.Fatal("expected cross broker queue error")
	}
}
