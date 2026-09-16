package errcode

import (
	"errors"
	"fmt"
)

// SafeMessage 返回可对外展示的安全消息 + 排障详细。
//
// 这是治理方案 ticket 000105 的核心安全函数：
// 业务层需要把错误序列化进响应 DTO / DB 列 / CLI diagnostic / admin UI message 时，
// 必须经过 SafeMessage，禁止直接用 err.Error()。
//
// 返回：
//   - publicMsg：可直接给用户看的中文文案（LayeredError 用注册文案；其它 err 用固定"内部错误，请稍后再试"）。
//   - logDetails：写服务端日志需要的字段（type + message），永远不返回给客户端。
//
// 设计原则：
//   - 不允许 err.Error() 直接泄露给客户端。
//   - 即使是 LayeredError 也只暴露注册时审核过的 Message，不暴露 cause / origin / chain（这些留在 logDetails 服务端）。
//   - logDetails 永远不返回给客户端（仅用于服务端日志）。
func SafeMessage(err error) (publicMsg string, logDetails map[string]any) {
	if err == nil {
		return "", nil
	}

	logDetails = make(map[string]any, 6)

	var le *LayeredError
	if errors.As(err, &le) && le != nil {
		publicMsg = le.Message()
		logDetails["code"] = le.Code()
		logDetails["module"] = le.Module()
		logDetails["msg_key"] = le.MsgKey()
		logDetails["http_status"] = le.HTTPStatus()
		if op := le.Operation(); op != "" {
			logDetails["operation"] = op
		}
		if origin := le.OriginStack(); origin != "" {
			logDetails["origin_stack"] = origin
		}
		if cause := le.DiagnosticCause(); cause != nil {
			logDetails["cause_type"] = fmt.Sprintf("%T", cause)
			logDetails["cause_message"] = cause.Error()
		}
		if root := le.RootCause(); root != nil {
			logDetails["root_type"] = fmt.Sprintf("%T", root)
			logDetails["root_message"] = root.Error()
		}
		return publicMsg, logDetails
	}

	// 非 LayeredError：固定文案 + 详细打日志
	publicMsg = "内部错误，请稍后再试"
	logDetails["cause_type"] = fmt.Sprintf("%T", err)
	logDetails["cause_message"] = err.Error()
	return publicMsg, logDetails
}
