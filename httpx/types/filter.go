package types

import (
	"reflect"
	"strings"

	"gorm.io/gorm"
)

// FilterConfig 过滤配置
type FilterConfig struct {
	// FieldMapping 字段名 -> 数据库列名映射（可选）
	FieldMapping map[string]string
	// Operators 字段名 -> 操作符（eq, like, gt, lt, gte, lte）
	Operators map[string]string
}

// ApplyFilter 动态应用过滤条件
// filter: 指针类型的 Filter 结构体
// 只处理非 nil 指针字段
func ApplyFilter(db *gorm.DB, filter any, config ...FilterConfig) *gorm.DB {
	v := reflect.ValueOf(filter)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return db
	}

	t := v.Type()
	cfg := FilterConfig{}
	if len(config) > 0 {
		cfg = config[0]
	}

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// 跳过嵌入类型（如 DateRange）
		if fieldType.Anonymous {
			continue
		}

		// 只处理指针类型且非 nil
		if field.Kind() != reflect.Ptr || field.IsNil() {
			continue
		}

		// 获取列名
		column := toSnakeCase(fieldType.Name)
		if mapped, ok := cfg.FieldMapping[fieldType.Name]; ok {
			column = mapped
		}

		// 获取操作符
		op := "eq"
		if opCfg, ok := cfg.Operators[fieldType.Name]; ok {
			op = opCfg
		}

		value := field.Elem().Interface()

		// 根据操作符构建查询
		switch op {
		case "like":
			if str, ok := value.(string); ok && str != "" {
				db = db.Where(column+" LIKE ?", "%"+str+"%")
			}
		case "gt":
			db = db.Where(column+" > ?", value)
		case "lt":
			db = db.Where(column+" < ?", value)
		case "gte":
			db = db.Where(column+" >= ?", value)
		case "lte":
			db = db.Where(column+" <= ?", value)
		default: // eq
			db = db.Where(column+" = ?", value)
		}
	}

	return db
}

// toSnakeCase 将驼峰命名转换为蛇形命名
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteByte('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
