// safe_msg.go 提供错误的安全出口转换（治理 ticket 000105 引入，000128 P0-3 修订）。
package errcode

import (
	"errors"
	"fmt"
)

// SafeMessageResult 是 SafeMessage 的 typed 返回结构（治理 ticket 000128 P0-3）。
// 拆分 public / private 语义，调用方无法再用 `_ = details` 静默丢弃诊断：
// 只要声明本结构就同时看到两个出口，错误选择哪个出口由字段名显式表达。
type SafeMessageResult struct {
	// PublicMessage 可直接给用户看的文案。
	// 仅当 err 链上存在注册业务码（Code()>0 && Module()!=""）的 LayeredError 时
	// 为其注册文案；裸 Capture / 动态 message / 普通 error 一律为固定内部文案。
	PublicMessage string
	// PublicData 显式标注可公开的业务数据（LayeredError.WithPublicData）；
	// 允许进入响应 DTO，其余数据禁止。
	PublicData map[string]any
	// Diagnostics 服务端排障字段（code/module/operation/origin/cause/root，已脱敏）。
	// 只允许写日志或内部诊断，永远不返回给客户端。
	Diagnostics map[string]any
}

// SafeMessage 把任意错误转成安全出口结构。
//
// 治理 ticket 000105 的核心安全函数：业务层需要把错误序列化进响应 DTO / DB 列 /
// CLI diagnostic / admin UI message 时必须经过 SafeMessage，禁止直接 err.Error()。
//
// 000128 P0-3 修订：
//   - 只有 Code()>0 && Module()!="" 的 LayeredError 才能作为 public message 来源；
//     裸 Capture 的动态 message（含 cause 原文）只能留在 Diagnostics，公开侧固定文案。
//   - Diagnostics 的 cause/root 文本统一经过 Redact 脱敏。
//   - PublicData 只承载显式标注的公开数据，与私有诊断彻底分离。
func SafeMessage(err error) SafeMessageResult {
	if err == nil {
		return SafeMessageResult{}
	}

	var le *LayeredError
	isLayered := errors.As(err, &le) && le != nil

	// 注册业务码 LayeredError：公开侧用注册文案 + 显式公开数据。
	if isLayered && le.Code() > 0 && le.Module() != "" {
		return SafeMessageResult{
			PublicMessage: le.Message(),
			PublicData:    le.PublicData(),
			Diagnostics:   diagnosticsFromLayered(le),
		}
	}

	// 裸 Capture / 未注册 LayeredError / 普通错误：固定内部文案，详情只进诊断。
	result := SafeMessageResult{PublicMessage: "内部错误，请稍后再试"}
	if isLayered {
		result.Diagnostics = diagnosticsFromLayered(le)
		return result
	}
	result.Diagnostics = map[string]any{
		"cause_type":    fmt.Sprintf("%T", err),
		"cause_message": Redact(err.Error()),
	}
	return result
}

// diagnosticsFromLayered 汇集服务端排障字段；cause/root 文本必须脱敏。
func diagnosticsFromLayered(le *LayeredError) map[string]any {
	details := map[string]any{
		"code":        le.Code(),
		"module":      le.Module(),
		"msg_key":     le.MsgKey(),
		"http_status": le.HTTPStatus(),
	}
	if op := le.Operation(); op != "" {
		details["operation"] = op
	}
	if origin := le.OriginStack(); origin != "" {
		details["origin_stack"] = origin
	}
	if cause := le.DiagnosticCause(); cause != nil {
		details["cause_type"] = fmt.Sprintf("%T", cause)
		details["cause_message"] = Redact(cause.Error())
	}
	if root := le.RootCause(); root != nil {
		details["root_type"] = fmt.Sprintf("%T", root)
		details["root_message"] = Redact(root.Error())
	}
	return details
}
