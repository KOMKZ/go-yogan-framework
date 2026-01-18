package event

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 注意：testEvent 和 mockKafkaPublisher 已在 dispatch_options_test.go 中定义

func TestDispatcher_RouteToKafka(t *testing.T) {
	// 创建路由器
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// 创建模拟发布者
	publisher := &mockKafkaPublisher{}

	// 创建分发器
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// 发送事件（不指定选项，走路由）
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	require.NoError(t, err)

	// 验证发送到 Kafka
	messages := publisher.getMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "events.order", messages[0].Topic)
}

func TestDispatcher_RouteToMemory(t *testing.T) {
	// 创建路由器（无匹配规则）
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// 创建分发器
	d := NewDispatcher(WithRouter(router))
	defer d.Close()

	// 订阅内存事件
	var received int32
	d.Subscribe("user.login", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// 发送事件（不匹配路由，走内存）
	err := d.Dispatch(context.Background(), &testEvent{name: "user.login"})
	require.NoError(t, err)

	// 验证走了内存
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
}

func TestDispatcher_CodeOptionOverridesRoute(t *testing.T) {
	// 创建路由器（配置走 Kafka）
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverKafka, Topic: "events.order"},
	})

	// 创建模拟发布者
	publisher := &mockKafkaPublisher{}

	// 创建分发器
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// 订阅内存事件
	var received int32
	d.Subscribe("order.created", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// 使用 WithMemory() 强制走内存，覆盖路由配置
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"}, WithMemory())
	require.NoError(t, err)

	// 验证走了内存（不是 Kafka）
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
	assert.Len(t, publisher.getMessages(), 0) // Kafka 未收到
}

func TestDispatcher_CodeKafkaOverridesRoute(t *testing.T) {
	// 创建路由器（配置走内存）
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"order.*": {Driver: DriverMemory},
	})

	// 创建模拟发布者
	publisher := &mockKafkaPublisher{}

	// 创建分发器
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// 订阅内存事件
	var received int32
	d.Subscribe("order.created", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// 使用 WithKafka() 强制走 Kafka，覆盖路由配置
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"}, WithKafka("custom.topic"))
	require.NoError(t, err)

	// 验证走了 Kafka（不是内存）
	assert.Equal(t, int32(0), atomic.LoadInt32(&received)) // 内存未收到
	messages := publisher.getMessages()
	assert.Len(t, messages, 1)
	assert.Equal(t, "custom.topic", messages[0].Topic)
}

func TestDispatcher_NoRouterDefaultsToMemory(t *testing.T) {
	// 创建分发器（无路由器）
	d := NewDispatcher()
	defer d.Close()

	// 订阅内存事件
	var received int32
	d.Subscribe("order.created", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&received, 1)
		return nil
	}))

	// 发送事件（无路由，走默认内存）
	err := d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	require.NoError(t, err)

	// 验证走了内存
	assert.Equal(t, int32(1), atomic.LoadInt32(&received))
}

func TestDispatcher_RouteWithUniversalWildcard(t *testing.T) {
	// 创建路由器（通用通配符）
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*": {Driver: DriverKafka, Topic: "events.all"},
	})

	// 创建模拟发布者
	publisher := &mockKafkaPublisher{}

	// 创建分发器
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// 发送多个事件
	_ = d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	_ = d.Dispatch(context.Background(), &testEvent{name: "user.login"})
	_ = d.Dispatch(context.Background(), &testEvent{name: "anything.else"})

	// 所有事件都应该发送到 Kafka
	messages := publisher.getMessages()
	assert.Len(t, messages, 3)
	for _, msg := range messages {
		assert.Equal(t, "events.all", msg.Topic)
	}
}

func TestDispatcher_RoutePriority(t *testing.T) {
	// 创建路由器（多个规则）
	router := NewRouter()
	router.LoadRoutes(map[string]RouteConfig{
		"*":             {Driver: DriverKafka, Topic: "events.all"},
		"order.*":       {Driver: DriverKafka, Topic: "events.order"},
		"order.created": {Driver: DriverKafka, Topic: "events.order.created"},
	})

	// 创建模拟发布者
	publisher := &mockKafkaPublisher{}

	// 创建分发器
	d := NewDispatcher(
		WithRouter(router),
		WithKafkaPublisher(publisher),
	)
	defer d.Close()

	// 精确匹配
	_ = d.Dispatch(context.Background(), &testEvent{name: "order.created"})
	messages := publisher.getMessages()
	assert.Equal(t, "events.order.created", messages[0].Topic)

	// 通配符匹配
	_ = d.Dispatch(context.Background(), &testEvent{name: "order.updated"})
	messages = publisher.getMessages()
	assert.Equal(t, "events.order", messages[1].Topic)

	// 通用通配符
	_ = d.Dispatch(context.Background(), &testEvent{name: "user.login"})
	messages = publisher.getMessages()
	assert.Equal(t, "events.all", messages[2].Topic)
}
