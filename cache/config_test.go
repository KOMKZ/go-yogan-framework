package cache

import (
	"testing"
	"time"
)

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "disabled config",
			config: Config{
				Enabled: false,
			},
			wantErr: false,
		},
		{
			name: "valid config",
			config: Config{
				Enabled:      true,
				DefaultTTL:   5 * time.Minute,
				DefaultStore: "memory",
				Stores: map[string]StoreConfig{
					"memory": {Type: "memory", MaxSize: 1000},
				},
				Cacheables: []CacheableConfig{
					{Name: "user:getById", KeyPattern: "user:{0}"},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid store type",
			config: Config{
				Enabled: true,
				Stores: map[string]StoreConfig{
					"invalid": {Type: "invalid"},
				},
			},
			wantErr: true,
		},
		{
			name: "cacheable without name",
			config: Config{
				Enabled: true,
				Cacheables: []CacheableConfig{
					{KeyPattern: "user:{0}"},
				},
			},
			wantErr: true,
		},
		{
			name: "cacheable without key pattern",
			config: Config{
				Enabled: true,
				Cacheables: []CacheableConfig{
					{Name: "user:getById"},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestConfig_ApplyDefaults(t *testing.T) {
	t.Run("empty config", func(t *testing.T) {
		cfg := Config{}
		cfg.ApplyDefaults()

		if cfg.DefaultTTL != 5*time.Minute {
			t.Errorf("DefaultTTL = %v, want 5m", cfg.DefaultTTL)
		}
		if cfg.DefaultStore != "memory" {
			t.Errorf("DefaultStore = %v, want memory", cfg.DefaultStore)
		}
	})

	t.Run("cacheable defaults", func(t *testing.T) {
		cfg := Config{
			DefaultTTL:   10 * time.Minute,
			DefaultStore: "redis",
			Cacheables: []CacheableConfig{
				{Name: "test", KeyPattern: "test:{0}"},
			},
		}
		cfg.ApplyDefaults()

		if cfg.Cacheables[0].TTL != 10*time.Minute {
			t.Errorf("Cacheable TTL = %v, want 10m", cfg.Cacheables[0].TTL)
		}
		if cfg.Cacheables[0].Store != "redis" {
			t.Errorf("Cacheable Store = %v, want redis", cfg.Cacheables[0].Store)
		}
		if !cfg.Cacheables[0].Enabled {
			t.Error("Cacheable should be enabled by default")
		}
	})
}

func TestConfig_ValidateStoreWithMissingType(t *testing.T) {
	cfg := Config{
		Enabled: true,
		Stores: map[string]StoreConfig{
			"empty": {}, // 无 type
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Error("Validate() expected error for store without type")
	}
}
