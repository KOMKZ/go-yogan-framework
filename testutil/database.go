package testutil

import (
	"gorm.io/gorm"
)

// DBHelper database test utility
type DBHelper struct {
	DB *gorm.DB
}

// NewDBHelper Create database helper utility
func NewDBHelper(db *gorm.DB) *DBHelper {
	return &DBHelper{DB: db}
}

// TruncateTable truncate table data (retain auto-increment ID)
func (h *DBHelper) TruncateTable(tableName string) error {
	// Disable foreign key checks
	if err := h.DB.Exec("SET FOREIGN_KEY_CHECKS = 0").Error; err != nil {
		return err
	}

	// Clear table
	if err := h.DB.Exec("TRUNCATE TABLE " + tableName).Error; err != nil {
		return err
	}

	// Enable foreign key checks
	return h.DB.Exec("SET FOREIGN_KEY_CHECKS = 1").Error
}

// TruncateTables truncate multiple tables
func (h *DBHelper) TruncateTables(tableNames ...string) error {
	for _, table := range tableNames {
		if err := h.TruncateTable(table); err != nil {
			return err
		}
	}
	return nil
}

// DeleteAll deletes all data in the table (does not reset auto-increment ID)
func (h *DBHelper) DeleteAll(tableName string) error {
	return h.DB.Exec("DELETE FROM " + tableName).Error
}

// Count Statistic record count (excluding soft-deleted records)
func (h *DBHelper) Count(tableName string) (int64, error) {
	var count int64
	err := h.DB.Table(tableName).Where("deleted_at IS NULL").Count(&count).Error
	return count, err
}

// CountWhere count records that meet the condition
func (h *DBHelper) CountWhere(tableName string, where string, args ...interface{}) (int64, error) {
	var count int64
	err := h.DB.Table(tableName).Where(where, args...).Count(&count).Error
	return count, err
}

// Exists Check if record exists
func (h *DBHelper) Exists(tableName string, where string, args ...interface{}) (bool, error) {
	count, err := h.CountWhere(tableName, where, args...)
	return count > 0, err
}

// Seed insert seed data
func (h *DBHelper) Seed(data interface{}) error {
	return h.DB.Create(data).Error
}

// SeedMultiple batch insert seed data
func (h *DBHelper) SeedMultiple(data interface{}) error {
	return h.DB.CreateInBatches(data, 100).Error
}

// FindOne query for a single record
func (h *DBHelper) FindOne(dest interface{}, where string, args ...interface{}) error {
	return h.DB.Where(where, args...).First(dest).Error
}

// FindAll query all records
func (h *DBHelper) FindAll(dest interface{}, tableName string) error {
	return h.DB.Table(tableName).Find(dest).Error
}

