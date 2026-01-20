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
func setFieldValue(cmd *cobra.Command, field reflect.Value, flagName string) error {
	switch field.Kind() {
	case reflect.String:
		val, _ := cmd.Flags().GetString(flagName)
		field.SetString(val)

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		val, _ := cmd.Flags().GetInt(flagName)
		field.SetInt(int64(val))

	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		val, _ := cmd.Flags().GetUint(flagName)
		field.SetUint(uint64(val))

	case reflect.Bool:
		val, _ := cmd.Flags().GetBool(flagName)
		field.SetBool(val)

	case reflect.Float32, reflect.Float64:
		val, _ := cmd.Flags().GetFloat64(flagName)
		field.SetFloat(val)

	case reflect.Slice:
		return setSliceValue(cmd, field, flagName)

	default:
		return fmt.Errorf("unsupported field type: %s", field.Kind())
	}

	return nil
}

// Set slice type field value
func setSliceValue(cmd *cobra.Command, field reflect.Value, flagName string) error {
	switch field.Type().Elem().Kind() {
	case reflect.String:
		val, _ := cmd.Flags().GetStringSlice(flagName)
		field.Set(reflect.ValueOf(val))

	case reflect.Int:
		val, _ := cmd.Flags().GetIntSlice(flagName)
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
func registerFlag(cmd *cobra.Command, field reflect.StructField, name, short, usage, defaultVal string) error {
	switch field.Type.Kind() {
	case reflect.String:
		cmd.Flags().StringP(name, short, defaultVal, usage)

	case reflect.Int:
		def := 0
		if defaultVal != "" {
			def, _ = strconv.Atoi(defaultVal)
		}
		cmd.Flags().IntP(name, short, def, usage)

	case reflect.Bool:
		def := false
		if defaultVal != "" {
			def, _ = strconv.ParseBool(defaultVal)
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
