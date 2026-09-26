package database

import (
	"errors"
	"fmt"
	"testing"

	"github.com/go-sql-driver/mysql"
	"gorm.io/gorm"
)

func TestIsDuplicateKeyError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "framework sentinel", err: ErrDuplicateKey, want: true},
		{name: "wrapped framework sentinel", err: fmt.Errorf("create: %w", ErrDuplicateKey), want: true},
		{name: "gorm translated", err: gorm.ErrDuplicatedKey, want: true},
		{name: "mysql 1062", err: &mysql.MySQLError{Number: 1062}, want: true},
		{name: "mysql other", err: &mysql.MySQLError{Number: 1146}, want: false},
		{name: "plain error", err: errors.New("duplicate key"), want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDuplicateKeyError(tt.err); got != tt.want {
				t.Fatalf("IsDuplicateKeyError() = %v, want %v", got, tt.want)
			}
		})
	}
}
