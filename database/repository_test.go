package database

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// TestRepoModel test model for repository tests
type TestRepoModel struct {
	ID        uint      `gorm:"primaryKey"`
	Name      string    `gorm:"size:100"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

// TableName specify table name to avoid conflicts
func (TestRepoModel) TableName() string {
	return "yogan_repo_test"
}

func setupRepoTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(mysql.Open(testDSN), &gorm.Config{})
	if err != nil {
		t.Skipf("Skipping test: MySQL not available: %v", err)
	}

	// AutoMigrate test table
	err = db.AutoMigrate(&TestRepoModel{})
	require.NoError(t, err)

	// Clean test data
	db.Exec("TRUNCATE TABLE yogan_repo_test")

	return db
}

// TestNewBaseRepository tests repository creation
func TestNewBaseRepository(t *testing.T) {
	db := setupRepoTestDB(t)

	repo := NewBaseRepository[TestRepoModel](db)
	assert.NotNil(t, repo)
	assert.NotNil(t, repo.DB())
}

// TestBaseRepository_Create tests creating records
func TestBaseRepository_Create(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)

	ctx := context.Background()
	entity := &TestRepoModel{Name: "test-create"}

	err := repo.Create(ctx, entity)
	assert.NoError(t, err)
	assert.NotZero(t, entity.ID)
}

// TestBaseRepository_FindByID tests finding by ID
func TestBaseRepository_FindByID(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test record
	entity := &TestRepoModel{Name: "test-find"}
	err := repo.Create(ctx, entity)
	require.NoError(t, err)

	// Find by ID
	found, err := repo.FindByID(ctx, entity.ID)
	assert.NoError(t, err)
	assert.NotNil(t, found)
	assert.Equal(t, entity.Name, found.Name)

	// Find non-existent ID
	notFound, err := repo.FindByID(ctx, 99999)
	assert.Error(t, err)
	assert.Nil(t, notFound)
	assert.Equal(t, ErrRecordNotFound, err)
}

// TestBaseRepository_FindAll tests finding all records
func TestBaseRepository_FindAll(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test records
	for i := 0; i < 3; i++ {
		entity := &TestRepoModel{Name: "test-findall"}
		err := repo.Create(ctx, entity)
		require.NoError(t, err)
	}

	// Find all
	entities, err := repo.FindAll(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(entities), 3)
}

// TestBaseRepository_Update tests updating records
func TestBaseRepository_Update(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test record
	entity := &TestRepoModel{Name: "test-update"}
	err := repo.Create(ctx, entity)
	require.NoError(t, err)

	// Update record
	entity.Name = "test-updated"
	err = repo.Update(ctx, entity)
	assert.NoError(t, err)

	// Verify update
	found, err := repo.FindByID(ctx, entity.ID)
	assert.NoError(t, err)
	assert.Equal(t, "test-updated", found.Name)
}

// TestBaseRepository_Delete tests deleting records
func TestBaseRepository_Delete(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test record
	entity := &TestRepoModel{Name: "test-delete"}
	err := repo.Create(ctx, entity)
	require.NoError(t, err)

	// Delete record
	err = repo.Delete(ctx, entity.ID)
	assert.NoError(t, err)

	// Verify deletion
	found, err := repo.FindByID(ctx, entity.ID)
	assert.Error(t, err)
	assert.Nil(t, found)
}

// TestBaseRepository_Exists tests checking existence
func TestBaseRepository_Exists(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test record
	entity := &TestRepoModel{Name: "test-exists"}
	err := repo.Create(ctx, entity)
	require.NoError(t, err)

	// Check existing record
	exists, err := repo.Exists(ctx, entity.ID)
	assert.NoError(t, err)
	assert.True(t, exists)

	// Check non-existent record
	exists, err = repo.Exists(ctx, 99999)
	assert.NoError(t, err)
	assert.False(t, exists)
}

// TestBaseRepository_Count tests counting records
func TestBaseRepository_Count(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test records
	for i := 0; i < 5; i++ {
		entity := &TestRepoModel{Name: "test-count"}
		err := repo.Create(ctx, entity)
		require.NoError(t, err)
	}

	// Count records
	count, err := repo.Count(ctx)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, count, int64(5))
}

// TestBaseRepository_Paginate tests pagination
func TestBaseRepository_Paginate(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Create test records
	for i := 0; i < 10; i++ {
		entity := &TestRepoModel{Name: "test-paginate"}
		err := repo.Create(ctx, entity)
		require.NoError(t, err)
	}

	// Paginate (page 1, pageSize 3)
	entities, total, err := repo.Paginate(ctx, 1, 3)
	assert.NoError(t, err)
	assert.Len(t, entities, 3)
	assert.GreaterOrEqual(t, total, int64(10))

	// Paginate (page 2, pageSize 3)
	entities2, _, err := repo.Paginate(ctx, 2, 3)
	assert.NoError(t, err)
	assert.Len(t, entities2, 3)
}

// TestBaseRepository_Transaction tests transactions
func TestBaseRepository_Transaction(t *testing.T) {
	db := setupRepoTestDB(t)
	repo := NewBaseRepository[TestRepoModel](db)
	ctx := context.Background()

	// Successful transaction
	err := repo.Transaction(ctx, func(tx *gorm.DB) error {
		entity := &TestRepoModel{Name: "test-tx"}
		return tx.Create(entity).Error
	})
	assert.NoError(t, err)

	// Rolled back transaction
	err = repo.Transaction(ctx, func(tx *gorm.DB) error {
		entity := &TestRepoModel{Name: "test-tx-rollback"}
		if err := tx.Create(entity).Error; err != nil {
			return err
		}
		return assert.AnError // Force rollback
	})
	assert.Error(t, err)
}
