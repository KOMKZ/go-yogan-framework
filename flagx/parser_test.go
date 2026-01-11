package flagx

import (
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// 测试用的 DTO
type TestRequest struct {
	Name  string `flag:"name"`
	Email string `flag:"email"`
	Age   int    `flag:"age"`
	Active bool  `flag:"active"`
}

func TestParseFlags(t *testing.T) {
	cmd := &cobra.Command{}
	cmd.Flags().StringP("name", "n", "", "用户名")
	cmd.Flags().StringP("email", "e", "", "邮箱")
	cmd.Flags().IntP("age", "a", 0, "年龄")
	cmd.Flags().BoolP("active", "", false, "是否激活")

	// 设置 flag 值
	cmd.Flags().Set("name", "张三")
	cmd.Flags().Set("email", "zhangsan@test.com")
	cmd.Flags().Set("age", "25")
	cmd.Flags().Set("active", "true")

	// 解析到结构体
	var req TestRequest
	err := ParseFlags(cmd, &req)

	require.NoError(t, err)
	assert.Equal(t, "张三", req.Name)
	assert.Equal(t, "zhangsan@test.com", req.Email)
	assert.Equal(t, 25, req.Age)
	assert.Equal(t, true, req.Active)
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

	// 验证 flags 已注册
	nameFlag := cmd.Flags().Lookup("name")
	assert.NotNil(t, nameFlag)
	assert.Equal(t, "用户名（必填）", nameFlag.Usage)

	emailFlag := cmd.Flags().Lookup("email")
	assert.NotNil(t, emailFlag)

	ageFlag := cmd.Flags().Lookup("age")
	assert.NotNil(t, ageFlag)
	assert.Equal(t, "18", ageFlag.DefValue)
}

