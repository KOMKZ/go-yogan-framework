// Package validator provides unified parameter validation and error conversion
package validator

import (
	"github.com/KOMKZ/go-yogan-framework/errcode"
	validation "github.com/go-ozzo/ozzo-validation/v4"
)

// Validatable interface
type Validatable interface {
	Validate() error
}

// ValidateRequest general validation function
// Convert ozzo-validation errors to LayeredError
func ValidateRequest(req Validatable) error {
	err := req.Validate()
	if err == nil {
		return nil
	}

	// Check if it is an ozzo-validation error
	if validationErrs, ok := err.(validation.Errors); ok {
		return ConvertValidationError(validationErrs)
	}

	// Return other errors directly
	return err
}

// ConvertValidationError converts ozzo-validation errors to LayeredError
func ConvertValidationError(validationErrs validation.Errors) error {
	// Extract field-level errors
	fields := make(map[string]string)
	for field, fieldErr := range validationErrs {
		if fieldErr != nil {
			fields[field] = fieldErr.Error()
		}
	}

	// Return a LayeredError with detailed information (using generic error codes)
	// Note: A generic validation failure error is created using the errcode package here.
	// Specific business error codes should be defined in errdef
	return errcode.New(
		1, 1010, // Module code 1 (common), business code 1010 (validation)
		"common",
		"error.common.validation_failed",
		"参数校验失败",
		400,
	).WithData("fields", fields)
}
