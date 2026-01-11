package flagx

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

// ParseFlags 从 cobra.Command 解析 flags 到结构体（类似 Gin 的 ShouldBind）
//
// 使用方式：
//
//	var req dto.CreateUserRequest
//	if err := flagx.ParseFlags(cmd, &req); err != nil {
//	    return err
//	}
//
// DTO 定义（支持 struct tags）：
//
//	type CreateUserRequest struct {
//	    Name  string `flag:"name"`
//	    Email string `flag:"email"`
//	    Age   int    `flag:"age"`
//	}
//
// 支持的 tag：
//   - flag: flag 名称（必须）
//   - default: 默认值（可选）
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

		// 跳过未导出的字段
		if !field.CanSet() {
			continue
		}

		// 获取 flag tag
		flagTag := fieldType.Tag.Get("flag")
		if flagTag == "" {
			continue
		}

		// 解析 flag 名称（支持短名称，如 "name,n"）
		flagName := strings.Split(flagTag, ",")[0]

		// 根据字段类型解析对应的 flag
		if err := setFieldValue(cmd, field, flagName); err != nil {
			return fmt.Errorf("parse field %s: %w", fieldType.Name, err)
		}
	}

	return nil
}

// setFieldValue 设置字段值
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

// setSliceValue 设置切片类型字段
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

// BindFlags 自动为结构体字段注册 flags（类似 Gin 的自动绑定）
//
// 使用方式：
//
//	cmd := &cobra.Command{...}
//	var req dto.CreateUserRequest
//	flagx.BindFlags(cmd, &req)
//
// DTO 定义（完整 tags）：
//
//	type CreateUserRequest struct {
//	    Name  string `flag:"name,n" usage:"用户名（必填）" required:"true"`
//	    Email string `flag:"email,e" usage:"邮箱（必填）" required:"true"`
//	    Age   int    `flag:"age,a" usage:"年龄" default:"0"`
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

		// 获取 tag 信息
		flagTag := fieldType.Tag.Get("flag")
		if flagTag == "" {
			continue
		}

		// 解析 flag 名称和短名称
		parts := strings.Split(flagTag, ",")
		flagName := parts[0]
		shortName := ""
		if len(parts) > 1 {
			shortName = parts[1]
		}

		usage := fieldType.Tag.Get("usage")
		defaultVal := fieldType.Tag.Get("default")
		required := fieldType.Tag.Get("required") == "true"

		// 根据字段类型注册对应的 flag
		if err := registerFlag(cmd, fieldType, flagName, shortName, usage, defaultVal); err != nil {
			return err
		}

		// 标记为必填
		if required {
			cmd.MarkFlagRequired(flagName)
		}
	}

	return nil
}

// registerFlag 注册 flag
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
