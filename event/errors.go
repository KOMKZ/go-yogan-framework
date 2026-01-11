package event

import "errors"

// ErrStopPropagation 停止事件传播（不视为错误）
// 监听器返回此错误时，后续监听器不再执行，但 Dispatch 不返回错误
var ErrStopPropagation = errors.New("stop propagation")

