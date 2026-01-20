package database

import (
	"context"
	"errors"
	"fmt"

	"gorm.io/gorm"
)

// BaseRepository generic Repository base class
type BaseRepository[T any] struct {
	db *gorm.DB
}

// NewBaseRepository creates the base class for Repository
func NewBaseRepository[T any](db *gorm.DB) *BaseRepository[T] {
	return &BaseRepository[T]{db: db}
}

// Get database instance
func (r *BaseRepository[T]) DB() *gorm.DB {
	return r.db
}

// Create record
func (r *BaseRepository[T]) Create(ctx context.Context, entity *T) error {
	if err := r.db.WithContext(ctx).Create(entity).Error; err != nil {
		return fmt.Errorf("Record creation failed: %w: %w", err)
	}
	return nil
}

// FindByID queries by ID (Laravel findOrFail style)
func (r *BaseRepository[T]) FindByID(ctx context.Context, id interface{}) (*T, error) {
	var entity T
	result := r.db.WithContext(ctx).First(&entity, id)
	if errors.Is(result.Error, gorm.ErrRecordNotFound) {
		return nil, ErrRecordNotFound
	}
	if result.Error != nil {
		return nil, fmt.Errorf("Query record failed (id=%v): %w (id=%v): %w", id, result.Error)
	}
	return &entity, nil
}

// FindAll query all records
func (r *BaseRepository[T]) FindAll(ctx context.Context) ([]T, error) {
	var entities []T
	if err := r.db.WithContext(ctx).Find(&entities).Error; err != nil {
		return nil, fmt.Errorf("Failed to query all records: %w: %w", err)
	}
	return entities, nil
}

// Update record
func (r *BaseRepository[T]) Update(ctx context.Context, entity *T) error {
	if err := r.db.WithContext(ctx).Save(entity).Error; err != nil {
		return fmt.Errorf("Update record failed: %w: %w", err)
	}
	return nil
}

// Delete soft delete record
func (r *BaseRepository[T]) Delete(ctx context.Context, id interface{}) error {
	var entity T
	if err := r.db.WithContext(ctx).Delete(&entity, id).Error; err != nil {
		return fmt.Errorf("Delete record failed (id=%v): %w (id=%v): %w", id, err)
	}
	return nil
}

// Exists check if record exists
func (r *BaseRepository[T]) Exists(ctx context.Context, id interface{}) (bool, error) {
	var count int64
	var entity T
	if err := r.db.WithContext(ctx).Model(&entity).Where("id = ?", id).Count(&count).Error; err != nil {
		return false, fmt.Errorf("Failed to check if record exists (id=%v): %w (id=%v): %w", id, err)
	}
	return count > 0, nil
}

// Count Statistic record count
func (r *BaseRepository[T]) Count(ctx context.Context) (int64, error) {
	var count int64
	var entity T
	if err := r.db.WithContext(ctx).Model(&entity).Count(&count).Error; err != nil {
		return 0, fmt.Errorf("Failed to count statistics: %w: %w", err)
	}
	return count, nil
}

// Paginate query
func (r *BaseRepository[T]) Paginate(ctx context.Context, page, pageSize int) ([]T, int64, error) {
	var entities []T
	var total int64

	// count total number
	var entity T
	if err := r.db.WithContext(ctx).Model(&entity).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("Paging query-failed to statistics total count: %w-Paging query-failed to statistics total count: %w: %w", err)
	}

	// paged query
	offset := (page - 1) * pageSize
	if err := r.db.WithContext(ctx).Offset(offset).Limit(pageSize).Find(&entities).Error; err != nil {
		return nil, 0, fmt.Errorf("Paging query-failed to retrieve data (page=%d, pageSize=%d): %w-Paging query-failed to retrieve data (page=%d, pageSize=%d): %w (page=%d, pageSize=%d): %w", page, pageSize, err)
	}

	return entities, total, nil
}

// Transaction execution
func (r *BaseRepository[T]) Transaction(ctx context.Context, fn func(tx *gorm.DB) error) error {
	return r.db.WithContext(ctx).Transaction(fn)
}

