package config

// Validator configuration validation interface (implemented by each module)
type Validator interface {
	Validate() error
}

// ValidateAll batch validate multiple configurations
func ValidateAll(validators ...Validator) error {
	for _, v := range validators {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

