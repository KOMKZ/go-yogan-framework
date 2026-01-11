package event

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestEvent 测试事件
type TestEvent struct {
	BaseEvent
	Data string
}

func NewTestEvent(name, data string) *TestEvent {
	return &TestEvent{
		BaseEvent: NewEvent(name),
		Data:      data,
	}
}

// ===== BaseEvent 测试 =====

func TestNewEvent(t *testing.T) {
	e := NewEvent("test.event")
	assert.Equal(t, "test.event", e.Name())
	assert.False(t, e.OccurredAt().IsZero())
}

func TestBaseEvent_Name(t *testing.T) {
	e := BaseEvent{name: "user.login"}
	assert.Equal(t, "user.login", e.Name())
}

func TestBaseEvent_OccurredAt(t *testing.T) {
	before := time.Now()
	e := NewEvent("test")
	after := time.Now()

	assert.True(t, e.OccurredAt().After(before) || e.OccurredAt().Equal(before))
	assert.True(t, e.OccurredAt().Before(after) || e.OccurredAt().Equal(after))
}

// ===== ListenerFunc 测试 =====

func TestListenerFunc_Handle(t *testing.T) {
	called := false
	var receivedEvent Event

	fn := ListenerFunc(func(ctx context.Context, e Event) error {
		called = true
		receivedEvent = e
		return nil
	})

	event := NewTestEvent("test", "data")
	err := fn.Handle(context.Background(), event)

	assert.NoError(t, err)
	assert.True(t, called)
	assert.Equal(t, event, receivedEvent)
}

func TestListenerFunc_Handle_Error(t *testing.T) {
	expectedErr := errors.New("handler error")
	fn := ListenerFunc(func(ctx context.Context, e Event) error {
		return expectedErr
	})

	err := fn.Handle(context.Background(), NewTestEvent("test", ""))
	assert.Equal(t, expectedErr, err)
}

// ===== Dispatcher 基础测试 =====

func TestNewDispatcher(t *testing.T) {
	d := NewDispatcher()
	require.NotNil(t, d)
	assert.NotNil(t, d.pool)
	assert.NotNil(t, d.listeners)
	d.Close()
}

func TestNewDispatcher_WithPoolSize(t *testing.T) {
	d := NewDispatcher(WithPoolSize(50))
	require.NotNil(t, d)
	assert.Equal(t, 50, d.poolSize)
	d.Close()
}

func TestDispatcher_Subscribe(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	listener := ListenerFunc(func(ctx context.Context, e Event) error {
		return nil
	})

	unsub := d.Subscribe("test.event", listener)
	assert.NotNil(t, unsub)
	assert.Equal(t, 1, d.ListenerCount("test.event"))
}

func TestDispatcher_Subscribe_EmptyEventName(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	listener := ListenerFunc(func(ctx context.Context, e Event) error {
		return nil
	})

	unsub := d.Subscribe("", listener)
	assert.NotNil(t, unsub)
	assert.Equal(t, 0, d.ListenerCount(""))
}

func TestDispatcher_Subscribe_NilListener(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	unsub := d.Subscribe("test.event", nil)
	assert.NotNil(t, unsub)
	assert.Equal(t, 0, d.ListenerCount("test.event"))
}

func TestDispatcher_Unsubscribe(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	listener := ListenerFunc(func(ctx context.Context, e Event) error {
		return nil
	})

	unsub := d.Subscribe("test.event", listener)
	assert.Equal(t, 1, d.ListenerCount("test.event"))

	unsub()
	assert.Equal(t, 0, d.ListenerCount("test.event"))
}

func TestDispatcher_Unsubscribe_Multiple(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	listener1 := ListenerFunc(func(ctx context.Context, e Event) error { return nil })
	listener2 := ListenerFunc(func(ctx context.Context, e Event) error { return nil })

	unsub1 := d.Subscribe("test.event", listener1)
	unsub2 := d.Subscribe("test.event", listener2)

	assert.Equal(t, 2, d.ListenerCount("test.event"))

	unsub1()
	assert.Equal(t, 1, d.ListenerCount("test.event"))

	unsub2()
	assert.Equal(t, 0, d.ListenerCount("test.event"))
}

// ===== Dispatch 同步分发测试 =====

func TestDispatcher_Dispatch(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var received string
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		received = e.(*TestEvent).Data
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", "hello"))
	assert.NoError(t, err)
	assert.Equal(t, "hello", received)
}

func TestDispatcher_Dispatch_NilEvent(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	err := d.Dispatch(context.Background(), nil)
	assert.NoError(t, err)
}

func TestDispatcher_Dispatch_NoListeners(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	err := d.Dispatch(context.Background(), NewTestEvent("unknown.event", ""))
	assert.NoError(t, err)
}

func TestDispatcher_Dispatch_MultipleListeners(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var order []int
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, 1)
		return nil
	}))
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, 2)
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2}, order)
}

func TestDispatcher_Dispatch_Error_StopsExecution(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	expectedErr := errors.New("listener error")
	var called []int

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		called = append(called, 1)
		return expectedErr
	}))
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		called = append(called, 2)
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.Equal(t, expectedErr, err)
	assert.Equal(t, []int{1}, called) // 第二个监听器未执行
}

func TestDispatcher_Dispatch_StopPropagation(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var called []int

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		called = append(called, 1)
		return ErrStopPropagation
	}))
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		called = append(called, 2)
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err) // ErrStopPropagation 不视为错误
	assert.Equal(t, []int{1}, called)
}

// ===== Priority 优先级测试 =====

func TestDispatcher_Priority(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var order []int

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, 3)
		return nil
	}), WithPriority(30))

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, 1)
		return nil
	}), WithPriority(10))

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, 2)
		return nil
	}), WithPriority(20))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, order)
}

// ===== WithOnce 一次性监听器测试 =====

func TestDispatcher_WithOnce(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	callCount := 0
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		callCount++
		return nil
	}), WithOnce())

	// 第一次分发
	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)

	// 第二次分发 - 监听器应该已被移除
	err = d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)
	assert.Equal(t, 1, callCount)
	assert.Equal(t, 0, d.ListenerCount("test.event"))
}

// ===== WithAsync 异步监听器测试 =====

func TestDispatcher_WithAsync_InSyncDispatch(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var wg sync.WaitGroup
	var asyncCalled int32

	wg.Add(1)
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		defer wg.Done()
		atomic.AddInt32(&asyncCalled, 1)
		return nil
	}), WithAsync())

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)

	// 等待异步执行完成
	wg.Wait()
	assert.Equal(t, int32(1), atomic.LoadInt32(&asyncCalled))
}

// ===== DispatchAsync 异步分发测试 =====

func TestDispatcher_DispatchAsync(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var wg sync.WaitGroup
	var received string
	var mu sync.Mutex

	wg.Add(1)
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		defer wg.Done()
		mu.Lock()
		received = e.(*TestEvent).Data
		mu.Unlock()
		return nil
	}))

	d.DispatchAsync(context.Background(), NewTestEvent("test.event", "async-data"))

	wg.Wait()
	mu.Lock()
	assert.Equal(t, "async-data", received)
	mu.Unlock()
}

func TestDispatcher_DispatchAsync_NilEvent(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	// 不应 panic
	d.DispatchAsync(context.Background(), nil)
}

func TestDispatcher_DispatchAsync_Error_NotReturned(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var wg sync.WaitGroup
	wg.Add(1)

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		defer wg.Done()
		return errors.New("listener error")
	}))

	// 异步分发不返回错误
	d.DispatchAsync(context.Background(), NewTestEvent("test.event", ""))
	wg.Wait()
}

func TestDispatcher_DispatchAsync_PreservesTraceID(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var wg sync.WaitGroup
	var receivedTraceID interface{}

	wg.Add(1)
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		defer wg.Done()
		receivedTraceID = ctx.Value("trace_id")
		return nil
	}))

	ctx := context.WithValue(context.Background(), "trace_id", "test-trace-123")
	d.DispatchAsync(ctx, NewTestEvent("test.event", ""))

	wg.Wait()
	assert.Equal(t, "test-trace-123", receivedTraceID)
}

// ===== Interceptor 拦截器测试 =====

func TestDispatcher_Use_Interceptor(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var order []string

	d.Use(func(ctx context.Context, e Event, next Next) error {
		order = append(order, "interceptor-before")
		err := next(ctx, e)
		order = append(order, "interceptor-after")
		return err
	})

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, "listener")
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)
	assert.Equal(t, []string{"interceptor-before", "listener", "interceptor-after"}, order)
}

func TestDispatcher_Use_MultipleInterceptors(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var order []string

	d.Use(func(ctx context.Context, e Event, next Next) error {
		order = append(order, "i1-before")
		err := next(ctx, e)
		order = append(order, "i1-after")
		return err
	})

	d.Use(func(ctx context.Context, e Event, next Next) error {
		order = append(order, "i2-before")
		err := next(ctx, e)
		order = append(order, "i2-after")
		return err
	})

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		order = append(order, "listener")
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.NoError(t, err)
	assert.Equal(t, []string{"i1-before", "i2-before", "listener", "i2-after", "i1-after"}, order)
}

func TestDispatcher_Interceptor_CanStopExecution(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	listenerCalled := false
	interceptorErr := errors.New("interceptor blocked")

	d.Use(func(ctx context.Context, e Event, next Next) error {
		return interceptorErr // 不调用 next
	})

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		listenerCalled = true
		return nil
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.Equal(t, interceptorErr, err)
	assert.False(t, listenerCalled)
}

func TestDispatcher_Interceptor_CanModifyError(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	customErr := errors.New("custom error")

	d.Use(func(ctx context.Context, e Event, next Next) error {
		_ = next(ctx, e)
		return customErr // 替换错误
	})

	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		return errors.New("original error")
	}))

	err := d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
	assert.Equal(t, customErr, err)
}

// ===== 并发测试 =====

func TestDispatcher_Concurrent_Subscribe(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
				return nil
			}))
		}()
	}
	wg.Wait()

	assert.Equal(t, 100, d.ListenerCount("test.event"))
}

func TestDispatcher_Concurrent_Dispatch(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	var counter int32
	d.Subscribe("test.event", ListenerFunc(func(ctx context.Context, e Event) error {
		atomic.AddInt32(&counter, 1)
		return nil
	}))

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = d.Dispatch(context.Background(), NewTestEvent("test.event", ""))
		}()
	}
	wg.Wait()

	assert.Equal(t, int32(100), atomic.LoadInt32(&counter))
}

// ===== Close 测试 =====

func TestDispatcher_Close(t *testing.T) {
	d := NewDispatcher()
	d.Close()

	// 关闭后异步分发应该被忽略
	d.DispatchAsync(context.Background(), NewTestEvent("test.event", ""))
}

// ===== ErrStopPropagation 测试 =====

func TestErrStopPropagation(t *testing.T) {
	assert.NotNil(t, ErrStopPropagation)
	assert.Equal(t, "stop propagation", ErrStopPropagation.Error())
}

// ===== 边界测试 =====

func TestDispatcher_Unsubscribe_NotExists(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	// 取消不存在的订阅不应 panic
	d.unsubscribe("nonexistent", 999)
}

func TestDispatcher_ListenerCount_NonExistent(t *testing.T) {
	d := NewDispatcher()
	defer d.Close()

	assert.Equal(t, 0, d.ListenerCount("nonexistent"))
}

// ===== Options 测试 =====

func TestWithPriority(t *testing.T) {
	entry := &listenerEntry{}
	WithPriority(100)(entry)
	assert.Equal(t, 100, entry.priority)
}

func TestWithAsync(t *testing.T) {
	entry := &listenerEntry{}
	WithAsync()(entry)
	assert.True(t, entry.async)
}

func TestWithOnce(t *testing.T) {
	entry := &listenerEntry{}
	WithOnce()(entry)
	assert.True(t, entry.once)
}

func TestWithPoolSize(t *testing.T) {
	d := &dispatcher{}
	WithPoolSize(200)(d)
	assert.Equal(t, 200, d.poolSize)
}

