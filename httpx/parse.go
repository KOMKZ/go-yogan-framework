package httpx

import (
	"reflect"

	"github.com/gin-gonic/gin"
)

// Parse automatically extract request parameters (query + body + path)
// Supports form/json/uri tags
func Parse(c *gin.Context, req interface{}) error {
	if hasBindingTag(req, "uri") {
		if err := c.ShouldBindUri(req); err != nil {
			return err
		}
	}

	if hasBindingTag(req, "form") {
		if err := c.ShouldBindQuery(req); err != nil {
			return err
		}
	}

	// 3. Bind Body parameters (json tag)
	// Only attempt to bind the body when Content-Length > 0
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(req); err != nil {
			return err
		}
	}

	return nil
}

func hasBindingTag(req interface{}, tagName string) bool {
	t := reflect.TypeOf(req)
	if t == nil {
		return false
	}

	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	if t.Kind() != reflect.Struct {
		return false
	}

	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		if hasNonEmptyTag(f, tagName) {
			return true
		}
		if f.Anonymous {
			fieldType := f.Type
			if fieldType.Kind() == reflect.Ptr {
				fieldType = fieldType.Elem()
			}
			if fieldType.Kind() == reflect.Struct {
				if hasBindingTag(reflect.New(fieldType).Interface(), tagName) {
					return true
				}
			}
		}
	}

	return false
}

func hasNonEmptyTag(field reflect.StructField, tagName string) bool {
	tagValue, ok := field.Tag.Lookup(tagName)
	if !ok {
		return false
	}
	return tagValue != "" && tagValue != "-"
}
