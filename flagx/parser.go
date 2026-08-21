package flagx

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// ParseFlags parses flags from cobra.Command to a struct (similar to Gin's ShouldBind)
//
// Usage:
//
//	var req dto.CreateUserRequest
//	if err := flagx.ParseFlags(cmd, &req); err != nil {
//	    return err
//	}
//
// DTO definition (supporting struct tags):
//
//	type CreateUserRequest struct {
//	    Name  string `flag:"name"`
//	    Email string `flag:"email"`
//	    Age   int    `flag:"age"`
//	}
//
// Supported tags:
// - flag: flag name (mandatory)
// - default: Default value (optional)
func ParseFlags(cmd *cobra.Command, target interface{}) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !field.CanSet() {
			continue
		}

		// Get flag tag
		flagTag := fieldType.Tag.Get("flag")
		if flagTag == "" {
			continue
		}

		// Parse flag names (support short names, e.g., "name,n")
		flagName := strings.Split(flagTag, ",")[0]

		// Parse the corresponding flag based on the field type
		if err := setFieldValue(cmd, field, flagName); err != nil {
			return fmt.Errorf("parse field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// set field value
// 🎯 Reads the raw flag string and converts explicitly: lookup and parse
// errors are returned instead of silently zeroing the field (previously a
// missing or mistyped flag left the field at zero with no feedback).
func setFieldValue(cmd *cobra.Command, field reflect.Value, flagName string) error {
	switch field.Kind() {
	case reflect.String, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Bool, reflect.Float32, reflect.Float64, reflect.Slice:
		// supported
	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	if field.Kind() == reflect.Slice {
		return setSliceValue(cmd, field, flagName)
	}

	// Read the raw string via Lookup (pflag's GetString type-checks the
	// flag kind, so it cannot read non-string flags).
	f := cmd.Flags().Lookup(flagName)
	if f == nil {
		return fmt.Errorf("flag accessed but not defined: %s", flagName)
	}
	raw := f.Value.String()

	switch field.Kind() {
	case reflect.String:
		field.SetString(raw)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, err := strconv.ParseInt(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid value %q for flag %s: %w", raw, flagName, err)
		}
		field.SetInt(val)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, err := strconv.ParseUint(raw, 10, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid value %q for flag %s: %w", raw, flagName, err)
		}
		field.SetUint(val)

	case reflect.Bool:
		val, err := strconv.ParseBool(raw)
		if err != nil {
			return fmt.Errorf("invalid value %q for flag %s: %w", raw, flagName, err)
		}
		field.SetBool(val)

	case reflect.Float32, reflect.Float64:
		val, err := strconv.ParseFloat(raw, field.Type().Bits())
		if err != nil {
			return fmt.Errorf("invalid value %q for flag %s: %w", raw, flagName, err)
		}
		field.SetFloat(val)
	}

	return nil
}

// Set slice type field value
func setSliceValue(cmd *cobra.Command, field reflect.Value, flagName string) error {
	switch field.Type().Elem().Kind() {
	case reflect.String:
		val, err := cmd.Flags().GetStringSlice(flagName)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(val))

	case reflect.Int:
		val, err := cmd.Flags().GetIntSlice(flagName)
		if err != nil {
			return err
		}
		field.Set(reflect.ValueOf(val))

	default:
		return fmt.Errorf("unsupported slice element type: %s", field.Type().Elem().Kind())
	}

	return nil
}

// BindFlags automatically registers flags for struct fields (similar to Gin's automatic binding)
//
// Usage:
//
//	cmd := &cobra.Command{...}
//	var req dto.CreateUserRequest
//	flagx.BindFlags(cmd, &req)
//
// DTO definition (complete tags):
//
//	type CreateUserRequest struct {
// Name string `flag:"name,n" usage:"username (required)" required:"true"`
// Email string `flag:"email,e" usage:"email (required)" required:"true"`
// Age   int    `flag:"age,a" usage:"age" default:"0"`
//	}
func BindFlags(cmd *cobra.Command, target interface{}) error {
	v := reflect.ValueOf(target)
	if v.Kind() != reflect.Ptr || v.Elem().Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	v = v.Elem()
	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		fieldType := t.Field(i)

		// Get tag information
		flagTag := fieldType.Tag.Get("flag")
		if flagTag == "" {
			continue
		}

		// Parse flag name and short name
		parts := strings.Split(flagTag, ",")
		flagName := parts[0]
		shortName := ""
		if len(parts) > 1 {
			shortName = parts[1]
		}

		usage := fieldType.Tag.Get("usage")
		defaultVal := fieldType.Tag.Get("default")
		required := fieldType.Tag.Get("required") == "true"

		// Register the corresponding flag based on the field type
		if err := registerFlag(cmd, fieldType, flagName, shortName, usage, defaultVal); err != nil {
			return err
		}

		// Mark as required
		if required {
			cmd.MarkFlagRequired(flagName)
		}
	}

	return nil
}

// registerFlag Register flag
// 🎯 The supported type matrix matches setFieldValue: int/uint/float families
// are registered as 64-bit flags and range-checked at parse time, so the
// same DTO behaves identically in BindFlags and ParseFlags.
func registerFlag(cmd *cobra.Command, field reflect.StructField, name, short, usage, defaultVal string) error {
	switch field.Type.Kind() {
	case reflect.String:
		cmd.Flags().StringP(name, short, defaultVal, usage)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		def := int64(0)
		if defaultVal != "" {
			var err error
			def, err = strconv.ParseInt(defaultVal, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid default %q for flag %s: %w", defaultVal, name, err)
			}
		}
		cmd.Flags().Int64P(name, short, def, usage)

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		def := uint64(0)
		if defaultVal != "" {
			var err error
			def, err = strconv.ParseUint(defaultVal, 10, 64)
			if err != nil {
				return fmt.Errorf("invalid default %q for flag %s: %w", defaultVal, name, err)
			}
		}
		cmd.Flags().Uint64P(name, short, def, usage)

	case reflect.Float32, reflect.Float64:
		def := float64(0)
		if defaultVal != "" {
			var err error
			def, err = strconv.ParseFloat(defaultVal, 64)
			if err != nil {
				return fmt.Errorf("invalid default %q for flag %s: %w", defaultVal, name, err)
			}
		}
		cmd.Flags().Float64P(name, short, def, usage)

	case reflect.Bool:
		def := false
		if defaultVal != "" {
			var err error
			def, err = strconv.ParseBool(defaultVal)
			if err != nil {
				return fmt.Errorf("invalid default %q for flag %s: %w", defaultVal, name, err)
			}
		}
		cmd.Flags().BoolP(name, short, def, usage)

	case reflect.Slice:
		if field.Type.Elem().Kind() == reflect.String {
			cmd.Flags().StringSliceP(name, short, nil, usage)
		} else if field.Type.Elem().Kind() == reflect.Int {
			cmd.Flags().IntSliceP(name, short, nil, usage)
		}

	default:
		return fmt.Errorf("unsupported field type: %s", field.Type.Kind())
	}

	return nil
}
