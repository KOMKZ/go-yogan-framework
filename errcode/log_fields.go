// log_fields.go 提供三出口（HTTP / Queue / CLI）共用的统一错误日志字段生成器
// （治理 ticket 000128 P1-1）。
//
// 之前 httpx.HandleError 与 queue asynq ErrorHandler 各自手拼同一套字段，
// 字段漂移无人发现，且 cause/root/chain 原文未脱敏就写日志。统一从这里生成：
// 请求/任务上下文（request_id / user_id / client_ip / task_id / task_type / queue）
// 仍由各出口自行追加。
package errcode

import (
	"errors"
	"fmt"

	"go.uber.org/zap"
)

// ErrorLogFields 把任意 error 转成统一 schema 的日志字段。
//
// LayeredError 输出：error_code（仅 >0）、error_msg（注册文案；未注册消息脱敏）、
// error_operation、error_origin_stack、error_cause_type/message（脱敏）、
// error_root_type/message（脱敏）、error_chain（脱敏）、error_data（私有诊断，仅日志）。
// 普通 error 输出：error_chain（脱敏）。
func ErrorLogFields(err error) []zap.Field {
	if err == nil {
		return nil
	}

	var le *LayeredError
	if !errors.As(err, &le) || le == nil {
		return []zap.Field{zap.String("error_chain", Redact(err.Error()))}
	}

	fields := make([]zap.Field, 0, 10)
	if le.Code() > 0 {
		fields = append(fields,
			zap.Int("error_code", le.Code()),
			zap.String("error_msg", PublicMessageOf(err)),
		)
	}
	if data := le.Data(); len(data) > 0 {
		fields = append(fields, zap.Any("error_data", data))
	}
	if originStack := le.OriginStack(); originStack != "" {
		fields = append(fields, zap.String("error_origin_stack", originStack))
	}
	if operation := le.Operation(); operation != "" {
		fields = append(fields, zap.String("error_operation", operation))
	}
	if cause := le.DiagnosticCause(); cause != nil {
		fields = append(fields,
			zap.String("error_cause_type", fmt.Sprintf("%T", cause)),
			zap.String("error_cause_message", Redact(cause.Error())),
		)
	}
	if root := le.RootCause(); root != nil {
		fields = append(fields,
			zap.String("error_root_type", fmt.Sprintf("%T", root)),
			zap.String("error_root_message", Redact(root.Error())),
		)
	}
	fields = append(fields, zap.String("error_chain", Redact(le.String())))
	return fields
}

// PublicMessageOf 返回可直接给客户端/任务详情的公开文案：
// 注册业务码用注册文案，其余（裸 Capture / 动态 message / 普通 error）固定内部文案。
// 出口层不得绕过本函数直接取 Message() 拼响应（治理 ticket 000128 P0-3）。
func PublicMessageOf(err error) string {
	res := SafeMessage(err)
	return res.PublicMessage
}
