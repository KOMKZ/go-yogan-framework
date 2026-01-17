package cache

import (
	"testing"
)

func TestErrors(t *testing.T) {
	errors := []*struct {
		err     error
		wantMsg string
	}{
		{ErrCacheMiss, "缓存未命中"},
		{ErrStoreNotFound, "存储后端未找到"},
		{ErrLoaderNotFound, "缓存加载器未注册"},
		{ErrSerialize, "序列化失败"},
		{ErrDeserialize, "反序列化失败"},
		{ErrStoreGet, "存储获取失败"},
		{ErrStoreSet, "存储设置失败"},
		{ErrStoreDelete, "存储删除失败"},
		{ErrConfigInvalid, "缓存配置无效"},
		{ErrCacheableNotFound, "缓存项未配置"},
	}

	for _, tt := range errors {
		t.Run(tt.wantMsg, func(t *testing.T) {
			if tt.err.Error() != tt.wantMsg {
				t.Errorf("Error() = %v, want %v", tt.err.Error(), tt.wantMsg)
			}
		})
	}
}

func TestErrorCodes(t *testing.T) {
	if ModuleCode != 70 {
		t.Errorf("ModuleCode = %d, want 70", ModuleCode)
	}
}
