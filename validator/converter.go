// Package validator 提供统一的参数校验和错误转换
package validator

import (
	"github.com/KOMKZ/go-yogan-framework/errcode"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Validatable 可校验接口
type Validatable interface {
	Validate() error
}

// ValidateRequest 通用校验函数
// 将 ozzo-validation 错误转换为 LayeredError
func ValidateRequest(req Validatable) error {
	err := req.Validate()
	if err == nil {
		return nil
	}

	// 判断是否为 ozzo-validation 错误
	if validationErrs, ok := err.(validation.Errors); ok {
		return ConvertValidationError(validationErrs)
	}

	// 其他错误直接返回
	return err
}

// ConvertValidationError 将 ozzo-validation 错误转换为 LayeredError
func ConvertValidationError(validationErrs validation.Errors) error {
	// 提取字段级错误
	fields := make(map[string]string)
	for field, fieldErr := range validationErrs {
		if fieldErr != nil {
			fields[field] = fieldErr.Error()
		}
	}

	// 返回带详细信息的 LayeredError (使用通用错误码)
	// 注意：这里使用 errcode 包创建一个通用的校验失败错误
	// 具体的业务错误码应该在 errdef 中定义
	return errcode.New(
		1, 1010, // 模块码 1 (common), 业务码 1010 (validation)
		"common",
		"error.common.validation_failed",
		"参数校验失败",
		400,
	).WithData("fields", fields)
}
