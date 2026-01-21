package flagx

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test DTO
type TestRequest struct {
	Name   string `flag:"name"`
	Email  string `flag:"email"`
	Age    int    `flag:"age"`
	Active bool   `flag:"active"`
}

func TestParseFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("name", "n", "", "用户名")
	cmd.Flags().StringP("email", "e", "", "邮箱")
	cmd.Flags().IntP("age", "a", 0, "年龄")
	cmd.Flags().BoolP("active", "", false, "是否激活")

	cmd.Flags().Set("name", "张三")
	cmd.Flags().Set("email", "zhangsan@test.com")
	cmd.Flags().Set("age", "25")
	cmd.Flags().Set("active", "true")

	var req TestRequest
	err := ParseFlags(cmd, &req)

	require.NoError(t, err)
	assert.Equal(t, "张三", req.Name)
	assert.Equal(t, "zhangsan@test.com", req.Email)
	assert.Equal(t, 25, req.Age)
	assert.Equal(t, true, req.Active)
}

func TestParseFlags_NonPointer(t *testing.T) {
	cmd := &cobra.Command{}
	var req TestRequest
	err := ParseFlags(cmd, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pointer to struct")
}

func TestParseFlags_NonStruct(t *testing.T) {
	cmd := &cobra.Command{}
	var s string
	err := ParseFlags(cmd, &s)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pointer to struct")
}

func TestParseFlags_UintTypes(t *testing.T) {
	type UintRequest struct {
		Count uint `flag:"count"`
	}

	cmd := &cobra.Command{}
	cmd.Flags().Uint("count", 0, "count")
	cmd.Flags().Set("count", "42")

	var req UintRequest
	err := ParseFlags(cmd, &req)
	require.NoError(t, err)
	assert.Equal(t, uint(42), req.Count)
}

func TestParseFlags_FloatTypes(t *testing.T) {
	type FloatRequest struct {
		Price float64 `flag:"price"`
	}

	cmd := &cobra.Command{}
	cmd.Flags().Float64("price", 0, "price")
	cmd.Flags().Set("price", "99.99")

	var req FloatRequest
	err := ParseFlags(cmd, &req)
	require.NoError(t, err)
	assert.Equal(t, 99.99, req.Price)
}

func TestParseFlags_StringSlice(t *testing.T) {
	type SliceRequest struct {
		Tags []string `flag:"tags"`
	}

	cmd := &cobra.Command{}
	cmd.Flags().StringSlice("tags", nil, "tags")
	cmd.Flags().Set("tags", "go,rust,python")

	var req SliceRequest
	err := ParseFlags(cmd, &req)
	require.NoError(t, err)
	assert.Equal(t, []string{"go", "rust", "python"}, req.Tags)
}

func TestParseFlags_IntSlice(t *testing.T) {
	type IntSliceRequest struct {
		IDs []int `flag:"ids"`
	}

	cmd := &cobra.Command{}
	cmd.Flags().IntSlice("ids", nil, "ids")
	cmd.Flags().Set("ids", "1,2,3")

	var req IntSliceRequest
	err := ParseFlags(cmd, &req)
	require.NoError(t, err)
	assert.Equal(t, []int{1, 2, 3}, req.IDs)
}

func TestParseFlags_UnsupportedSliceType(t *testing.T) {
	type UnsupportedSliceRequest struct {
		Data []float64 `flag:"data"`
	}

	cmd := &cobra.Command{}
	cmd.Flags().Float64("data", 0, "data")

	var req UnsupportedSliceRequest
	err := ParseFlags(cmd, &req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported slice element type")
}

func TestParseFlags_UnsupportedType(t *testing.T) {
	type UnsupportedRequest struct {
		Data complex128 `flag:"data"`
	}

	cmd := &cobra.Command{}
	var req UnsupportedRequest
	err := ParseFlags(cmd, &req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported field type")
}

func TestParseFlags_NoFlagTag(t *testing.T) {
	type NoTagRequest struct {
		Name  string `flag:"name"`
		Other string
	}

	cmd := &cobra.Command{}
	cmd.Flags().String("name", "", "name")
	cmd.Flags().Set("name", "test")

	var req NoTagRequest
	err := ParseFlags(cmd, &req)
	require.NoError(t, err)
	assert.Equal(t, "test", req.Name)
	assert.Equal(t, "", req.Other)
}

func TestBindFlags(t *testing.T) {
	cmd := &cobra.Command{}

	type BindRequest struct {
		Name  string `flag:"name,n" usage:"用户名（必填）" required:"true"`
		Email string `flag:"email,e" usage:"邮箱（必填）" required:"true"`
		Age   int    `flag:"age,a" usage:"年龄" default:"18"`
	}

	var req BindRequest
	err := BindFlags(cmd, &req)

	require.NoError(t, err)

	nameFlag := cmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "用户名（必填）", nameFlag.Usage)

	emailFlag := cmd.Flags().Lookup("email")
	assert.NotNil(t, emailFlag)

	ageFlag := cmd.Flags().Lookup("age")
	assert.NotNil(t, ageFlag)
	assert.Equal(t, "18", ageFlag.DefValue)
}

func TestBindFlags_NonPointer(t *testing.T) {
	cmd := &cobra.Command{}
	type Req struct{}
	var req Req
	err := BindFlags(cmd, req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "pointer to struct")
}

func TestBindFlags_BoolType(t *testing.T) {
	cmd := &cobra.Command{}

	type BoolRequest struct {
		Verbose bool `flag:"verbose,v" usage:"verbose mode" default:"true"`
	}

	var req BoolRequest
	err := BindFlags(cmd, &req)
	require.NoError(t, err)

	flag := cmd.Flags().Lookup("verbose")
	assert.NotNil(t, flag)
	assert.Equal(t, "true", flag.DefValue)
}

func TestBindFlags_SliceTypes(t *testing.T) {
	cmd := &cobra.Command{}

	type SliceRequest struct {
		Tags []string `flag:"tags,t" usage:"tags"`
		IDs  []int    `flag:"ids,i" usage:"ids"`
	}

	var req SliceRequest
	err := BindFlags(cmd, &req)
	require.NoError(t, err)

	tagsFlag := cmd.Flags().Lookup("tags")
	assert.NotNil(t, tagsFlag)

	idsFlag := cmd.Flags().Lookup("ids")
	assert.NotNil(t, idsFlag)
}

func TestBindFlags_UnsupportedType(t *testing.T) {
	cmd := &cobra.Command{}

	type UnsupportedRequest struct {
		Data complex128 `flag:"data"`
	}

	var req UnsupportedRequest
	err := BindFlags(cmd, &req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported field type")
}

func TestBindFlags_NoShortName(t *testing.T) {
	cmd := &cobra.Command{}

	type SimpleRequest struct {
		Name string `flag:"name" usage:"name"`
	}

	var req SimpleRequest
	err := BindFlags(cmd, &req)
	require.NoError(t, err)

	flag := cmd.Flags().Lookup("name")
	assert.NotNil(t, flag)
}
