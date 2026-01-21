package types

import "time"

// DateFormat 标准日期格式
const DateFormat = "2006-01-02"

// DateRange 日期范围查询
type DateRange struct {
	StartDate string `form:"startDate" json:"startDate"`
	EndDate   string `form:"endDate" json:"endDate"`
}

// ParseStart 解析开始日期
func (d *DateRange) ParseStart() *time.Time {
	if d.StartDate == "" {
		return nil
	}
	t, err := time.Parse(DateFormat, d.StartDate)
	if err != nil {
		return nil
	}
	return &t
}

// ParseEnd 解析结束日期（当天 23:59:59）
func (d *DateRange) ParseEnd() *time.Time {
	if d.EndDate == "" {
		return nil
	}
	t, err := time.Parse(DateFormat, d.EndDate)
	if err != nil {
		return nil
	}
	t = t.Add(24*time.Hour - time.Second)
	return &t
}
