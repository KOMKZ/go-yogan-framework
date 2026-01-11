package testutil

import (
	"gorm.io/gorm"
)

// DBHelper 数据库测试辅助工具
type DBHelper struct {
	DB *gorm.DB
}

// NewDBHelper 创建数据库辅助工具
func NewDBHelper(db *gorm.DB) *DBHelper {
	return &DBHelper{DB: db}
}

// TruncateTable 清空表数据（保留自增ID）
func (h *DBHelper) TruncateTable(tableName string) error {
	// 禁用外键检查
	if err := h.DB.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return err
	}

	// 清空表
	if err := h.DB.Exec("TRUNCATE TABLE " + tableName).Error; err != nil {
		return err
	}

	// 启用外键检查
	return h.DB.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
}

// TruncateTables 清空多个表
func (h *DBHelper) TruncateTables(tableNames ...string) error {
	for _, table := range tableNames {
		if err := h.TruncateTable(table); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAll 删除表中所有数据（不重置自增ID）
func (h *DBHelper) DeleteAll(tableName string) error {
	return h.DB.Exec("DELETE FROM " + tableName).Error
}

// Count 统计记录数（排除软删除记录）
func (h *DBHelper) Count(tableName string) (int64, error) {
	var count int64
	err := h.DB.Table(tableName).Where("deleted_at IS NULL").Count(&count).Error
	return count, err
}

// CountWhere 统计符合条件的记录数
func (h *DBHelper) CountWhere(tableName string, where string, args ...interface{}) (int64, error) {
	var count int64
	err := h.DB.Table(tableName).Where(where, args...).Count(&count).Error
	return count, err
}

// Exists 检查记录是否存在
func (h *DBHelper) Exists(tableName string, where string, args ...interface{}) (bool, error) {
	count, err := h.CountWhere(tableName, where, args...)
	return count > 0, err
}

// Seed 插入种子数据
func (h *DBHelper) Seed(data interface{}) error {
	return h.DB.Create(data).Error
}

// SeedMultiple 批量插入种子数据
func (h *DBHelper) SeedMultiple(data interface{}) error {
	return h.DB.CreateInBatches(data, 100).Error
}

// FindOne 查询单条记录
func (h *DBHelper) FindOne(dest interface{}, where string, args ...interface{}) error {
	return h.DB.Where(where, args...).First(dest).Error
}

// FindAll 查询所有记录
func (h *DBHelper) FindAll(dest interface{}, tableName string) error {
	return h.DB.Table(tableName).Find(dest).Error
}

