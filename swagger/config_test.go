package swagger

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.False(t, cfg.Enabled, "默认应禁用 Swagger")
	assert.Equal(t, "/swagger/*any", cfg.UIPath)
	assert.Equal(t, "/openapi.json", cfg.SpecPath)
	assert.True(t, cfg.DeepLinking)
	assert.True(t, cfg.PersistAuthorization)
}

func TestConfig_ApplyDefaults(t *testing.T) {
	tests := []struct {
		name     string
		cfg      Config
		expected Config
	}{
		{
			name: "空配置应用默认值",
			cfg:  Config{},
			expected: Config{
				UIPath:   "/swagger/*any",
				SpecPath: "/openapi.json",
			},
		},
		{
			name: "已有值不覆盖",
			cfg: Config{
				UIPath:   "/docs/*any",
				SpecPath: "/api-spec.json",
			},
			expected: Config{
				UIPath:   "/docs/*any",
				SpecPath: "/api-spec.json",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.cfg.ApplyDefaults()
			assert.Equal(t, tt.expected.UIPath, tt.cfg.UIPath)
			assert.Equal(t, tt.expected.SpecPath, tt.cfg.SpecPath)
		})
	}
}

func TestConfig_Validate(t *testing.T) {
	cfg := Config{
		Enabled: true,
		UIPath:  "/swagger/*any",
	}

	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestDefaultSwaggerInfo(t *testing.T) {
	info := DefaultSwaggerInfo()

	assert.Equal(t, "API Documentation", info.Title)
	assert.Equal(t, "1.0.0", info.Version)
	assert.Equal(t, "/api", info.BasePath)
	assert.Contains(t, info.Schemes, "http")
	assert.Contains(t, info.Schemes, "https")
}
