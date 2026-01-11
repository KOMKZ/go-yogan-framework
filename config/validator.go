package config

// Validator 配置验证接口（各模块实现）
type Validator interface {
	Validate() error
}

// ValidateAll 批量验证多个配置
func ValidateAll(validators ...Validator) error {
	for _, v := range validators {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

