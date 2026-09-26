package database

import (
	"context"
	"errors"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type duplicateKeyTestModel struct {
	ID  uint   `gorm:"primaryKey"`
	Key string `gorm:"uniqueIndex"`
}

func TestBaseRepositoryCreateMapsDuplicateKeyAndPreservesCause(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{TranslateError: true})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&duplicateKeyTestModel{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := NewBaseRepository[duplicateKeyTestModel](db)
	if err := repo.Create(context.Background(), &duplicateKeyTestModel{Key: "same"}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	err = repo.Create(context.Background(), &duplicateKeyTestModel{Key: "same"})
	if !errors.Is(err, ErrDuplicateKey) {
		t.Fatalf("duplicate create error = %v, want ErrDuplicateKey", err)
	}
	if !errors.Is(err, gorm.ErrDuplicatedKey) {
		t.Fatalf("duplicate create error = %v, want preserved GORM cause", err)
	}
}
