package types

import "github.com/jinzhu/copier"

// CopyOption 复制选项
type CopyOption struct {
	IgnoreEmpty bool // 忽略零值字段
	DeepCopy    bool // 深拷贝（slice/map）
}

// Copy 复制结构体（字段名匹配）
// 用户可选择使用此工具，也可手动赋值
func Copy(dst, src interface{}) error {
	return copier.Copy(dst, src)
}

// CopyWithOption 带选项复制
func CopyWithOption(dst, src interface{}, opt CopyOption) error {
	return copier.CopyWithOption(dst, src, copier.Option{
		IgnoreEmpty: opt.IgnoreEmpty,
		DeepCopy:    opt.DeepCopy,
	})
}

// CopySlice 复制切片（泛型版本）
func CopySlice[S, D any](src []S, converter func(S) D) []D {
	if src == nil {
		return nil
	}
	dst := make([]D, len(src))
	for i, s := range src {
		dst[i] = converter(s)
	}
	return dst
}
