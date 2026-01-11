package event

import "context"

// Next 继续执行下一个拦截器/监听器
type Next func(ctx context.Context, event Event) error

// Interceptor 事件拦截器
// 可用于日志记录、错误处理、事件过滤等
type Interceptor func(ctx context.Context, event Event, next Next) error

