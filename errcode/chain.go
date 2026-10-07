// chain.go 提供 LayeredError 错误链遍历（治理 ticket 000395 批次一；000395 整改 P1-1 修订）。
//
// 嵌套 %w（fmt.Errorf / errors.Join）会把深层 LayeredError 埋进普通包装层：
// origin 保真（Capture / cloneOrigin）与受控诊断传播（DiagnosticFields）都需要沿
// Unwrap 链找出全部 LayeredError。遍历只做只读收集，不合并任何诊断数据——
// 合并与脱敏由 diag.go 的词表约束负责。
//
// 整改 P1-1：不再用 map[error]bool 做环防护——error 的动态值可能是 slice/map 等
// 不可比较类型，作为 map key 会 panic。全棵遍历共享节点预算，同时限制线性环
// 和 Unwrap() []error 的分支环/高分叉 DAG，保证总工作量有界。
package errcode

// maxChainNodes 是一次错误链查询允许访问的最大节点数。
// 真实错误链通常是个位数；预算耗尽后保留已收集结果并立即停止展开。
const maxChainNodes = 64

// layeredErrorChain 沿 Unwrap 链按外层→内层顺序返回全部 *LayeredError。
// 支持标准 Unwrap() error 与 errors.Join 的 Unwrap() []error（DFS 展开）；
// 任意 error 实现（含不可比较动态值）都可安全遍历，总访问量在 maxChainNodes 截断。
func layeredErrorChain(err error) []*LayeredError {
	layers := make([]*LayeredError, 0, 4)
	remaining := maxChainNodes

	var walk func(current error)
	walk = func(current error) {
		for current != nil && remaining > 0 {
			remaining--
			if layered, ok := current.(*LayeredError); ok {
				layers = append(layers, layered)
			}
			switch unwrapper := current.(type) {
			case interface{ Unwrap() error }:
				current = unwrapper.Unwrap()
			case interface{ Unwrap() []error }:
				for _, joined := range unwrapper.Unwrap() {
					walk(joined)
					if remaining == 0 {
						break
					}
				}
				return
			default:
				return
			}
		}
	}
	walk(err)
	return layers
}

// innermostLayeredError 返回链上最内层（DFS 最后一个）的 LayeredError；
// 链上没有 LayeredError 时返回 nil。errors.Join 场景取最后一个分支，
// 保持确定性；两个分支的原文仍完整保留在错误链文本中。
func innermostLayeredError(err error) *LayeredError {
	layers := layeredErrorChain(err)
	if len(layers) == 0 {
		return nil
	}
	return layers[len(layers)-1]
}

// outermostRegisteredError 返回链上第一个注册业务码（Code()>0 且 Module()!=""）的
// LayeredError，语义与 errors.As 的最外层匹配一致：出口对外的业务码来自链上最外层
// 注册错误，嵌套更深的注册码视为其内部成因。没有注册码时返回 nil。
func outermostRegisteredError(err error) *LayeredError {
	for _, layered := range layeredErrorChain(err) {
		if layered.Code() > 0 && layered.Module() != "" {
			return layered
		}
	}
	return nil
}
