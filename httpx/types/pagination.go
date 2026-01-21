// Package types 提供 HTTP 请求/响应的通用类型
package types

// PageQuery 分页查询基类
type PageQuery struct {
	Current int `form:"current" json:"current"`
	Size    int `form:"size" json:"size"`
}

// ApplyDefaults 应用默认分页值
func (p *PageQuery) ApplyDefaults() {
	if p.Current <= 0 {
		p.Current = 1
	}
	if p.Size <= 0 {
		p.Size = 10
	}
	if p.Size > 100 {
		p.Size = 100
	}
}

// Offset 计算偏移量
func (p *PageQuery) Offset() int {
	return (p.Current - 1) * p.Size
}

// PageMeta 分页元数据
type PageMeta struct {
	Total   int64 `json:"total"`
	Size    int   `json:"size"`
	Current int   `json:"current"`
	Pages   int   `json:"pages"`
}

// NewPageMeta 创建分页元数据
func NewPageMeta(total int64, current, size int) PageMeta {
	pages := int(total) / size
	if int(total)%size > 0 {
		pages++
	}
	return PageMeta{
		Total:   total,
		Size:    size,
		Current: current,
		Pages:   pages,
	}
}
