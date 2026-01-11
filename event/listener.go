package event

import "context"

// Listener 监听器接口
type Listener interface {
	// Handle 处理事件
	// 返回 error 时，同步分发会停止后续监听器执行
	// 返回 ErrStopPropagation 时，停止传播但不视为错误
	Handle(ctx context.Context, event Event) error
}

// ListenerFunc 函数式监听器适配器
type ListenerFunc func(ctx context.Context, event Event) error

// Handle 实现 Listener 接口
func (f ListenerFunc) Handle(ctx context.Context, event Event) error {
	return f(ctx, event)
}

