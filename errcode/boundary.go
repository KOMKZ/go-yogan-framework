// Package errcode 提供 boundary 工具，让 service 入口用 defer 给返回值补 origin stack + operation。
// 这是治理方案 ticket 000105 的核心能力：消除 service 层"裸 return err"导致的 cause 丢失 / 无 origin。
package errcode

// CaptureInto 在 service 入口 defer 调用，给尚未捕获的错误补 origin stack + operation。
//
// 使用模式：
//
//	func (s *FooService) Bar(ctx context.Context, in BarInput) (*BarResult, error) {
//	    var err error
//	    defer errcode.CaptureInto(&err, "foo.service.bar")
//	    ...
//	}
//
// 语义：
//   - errp 为 nil 或 *errp == nil：直接返回，不做处理。
//   - *errp 已经是带 originStack 的 *LayeredError：不覆盖已有 origin（保留最内层 I/O 边界的栈）。
//   - *errp 是 *LayeredError 但没 originStack：补 operation 并重算 originStack（少见，但可能发生在配置错手工构造的场景）。
//   - *errp 是普通 error：替换为 Capture(err, operation) 的 *LayeredError。
//
// 设计取舍：保留"最内层 origin"而不是"service 入口 origin"——
// 因为排查时第一现场永远是 I/O 边界（DB / OSS / RPC / SDK），service 只是包装。
// service 入口的 operation 字段会写到日志的 error_operation，能稳定定位业务层。
func CaptureInto(errp *error, operation string) {
	if errp == nil {
		return
	}
	if *errp == nil {
		return
	}

	switch e := (*errp).(type) {
	case *LayeredError:
		if e.OriginStack() != "" {
			// 已被 I/O 边界捕获；只补 operation（如果原本没有）。
			if e.Operation() == "" {
				clone := *e
				clone.operation = operation
				*errp = &clone
			}
			return
		}
		// LayeredError 但缺 origin（手工构造 / 配置校验）；补 operation + 算 stack。
		clone := *e
		clone.operation = operation
		clone.originStack = errorOriginStack(e)
		*errp = &clone
	default:
		*errp = Capture(e, operation)
	}
}

// Boundary 组合 CaptureInto + 标准 panic recover，返回值 err 总是带 origin + operation。
// 适用于"必须不丢错"的入口，例如 worker executor / cron tick / 消息消费循环。
//
// 使用模式：
//
//	func (s *Worker) Run(ctx context.Context, job Job) (err error) {
//	    defer errcode.Boundary(&err, "worker.job.run")
//	    ...
//	}
//
// 当前不引入 panic recover（治理方案未要求），保留为简单语义，未来扩展点。
func Boundary(errp *error, operation string) {
	CaptureInto(errp, operation)
}
