package validator

import (
	"errors"
	"testing"

	"github.com/KOMKZ/go-yogan-framework/errcode"
	validation "github.com/go-ozzo/ozzo-validation/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockValidatable 实现 Validatable 接口用于测试
type MockValidatable struct {
	ShouldFail       bool
	ValidationErrors validation.Errors
	OtherError       error
}

func (m *MockValidatable) Validate() error {
	if m.OtherError != nil {
		return m.OtherError
	}
	if m.ShouldFail && m.ValidationErrors != nil {
		return m.ValidationErrors
	}
	if m.ShouldFail {
		return validation.Errors{
			"field1": errors.New("field1 is required"),
		}
	}
	return nil
}

func TestValidateRequest_Success(t *testing.T) {
	req := &MockValidatable{ShouldFail: false}
	err := ValidateRequest(req)
	assert.NoError(t, err)
}

func TestValidateRequest_ValidationError(t *testing.T) {
	req := &MockValidatable{
		ShouldFail: true,
		ValidationErrors: validation.Errors{
			"email":    errors.New("email format is invalid"),
			"password": errors.New("password is required"),
		},
	}

	err := ValidateRequest(req)
	require.Error(t, err)

	// 验证返回的是 LayeredError
	layeredErr, ok := err.(*errcode.LayeredError)
	require.True(t, ok, "expected LayeredError")

	// 验证错误码
	assert.Equal(t, 400, layeredErr.HTTPStatus())
	assert.Equal(t, "common", layeredErr.Module())

	// 验证包含字段错误信息
	data := layeredErr.Data()
	require.NotNil(t, data)
	fields, ok := data["fields"].(map[string]string)
	require.True(t, ok)
	assert.Contains(t, fields, "email")
	assert.Contains(t, fields, "password")
}

func TestValidateRequest_OtherError(t *testing.T) {
	customErr := errors.New("custom error")
	req := &MockValidatable{
		ShouldFail: true,
		OtherError: customErr,
	}

	err := ValidateRequest(req)
	require.Error(t, err)
	assert.Equal(t, customErr, err)
}

func TestConvertValidationError(t *testing.T) {
	t.Run("single field error", func(t *testing.T) {
		validationErrs := validation.Errors{
			"username": errors.New("username is required"),
		}

		err := ConvertValidationError(validationErrs)
		require.Error(t, err)

		layeredErr, ok := err.(*errcode.LayeredError)
		require.True(t, ok)

		data := layeredErr.Data()
		fields := data["fields"].(map[string]string)
		assert.Equal(t, "username is required", fields["username"])
	})

	t.Run("multiple field errors", func(t *testing.T) {
		validationErrs := validation.Errors{
			"name":  errors.New("name cannot be empty"),
			"age":   errors.New("age must be positive"),
			"email": errors.New("invalid email format"),
		}

		err := ConvertValidationError(validationErrs)
		require.Error(t, err)

		layeredErr, ok := err.(*errcode.LayeredError)
		require.True(t, ok)

		data := layeredErr.Data()
		fields := data["fields"].(map[string]string)
		assert.Len(t, fields, 3)
		assert.Equal(t, "name cannot be empty", fields["name"])
		assert.Equal(t, "age must be positive", fields["age"])
		assert.Equal(t, "invalid email format", fields["email"])
	})

	t.Run("nil field error is skipped", func(t *testing.T) {
		validationErrs := validation.Errors{
			"valid":   nil,
			"invalid": errors.New("field is invalid"),
		}

		err := ConvertValidationError(validationErrs)
		require.Error(t, err)

		layeredErr, ok := err.(*errcode.LayeredError)
		require.True(t, ok)

		data := layeredErr.Data()
		fields := data["fields"].(map[string]string)
		assert.Len(t, fields, 1)
		assert.NotContains(t, fields, "valid")
		assert.Contains(t, fields, "invalid")
	})

	t.Run("empty validation errors", func(t *testing.T) {
		validationErrs := validation.Errors{}

		err := ConvertValidationError(validationErrs)
		require.Error(t, err)

		layeredErr, ok := err.(*errcode.LayeredError)
		require.True(t, ok)

		data := layeredErr.Data()
		fields := data["fields"].(map[string]string)
		assert.Empty(t, fields)
	})
}

func TestValidatable_Interface(t *testing.T) {
	// 验证接口定义正确
	var _ Validatable = &MockValidatable{}
}
