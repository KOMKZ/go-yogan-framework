package database

import (
	"strings"
	"testing"

	"gorm.io/gorm"
)

func TestResolveDialectorBuiltins(t *testing.T) {
	for _, name := range []string{"mysql", "postgres"} {
		dialector, err := resolveDialector(name, "test-dsn")
		if err != nil {
			t.Fatalf("resolve %s: %v", name, err)
		}
		if dialector == nil {
			t.Fatalf("resolve %s returned nil dialector", name)
		}
	}
}

func TestResolveDialectorRejectsUnregisteredDriver(t *testing.T) {
	_, err := resolveDialector("sqlite-not-registered-here", "test-dsn")
	if err == nil || !strings.Contains(err.Error(), "unsupported driver") {
		t.Fatalf("resolve unregistered driver error = %v", err)
	}
}

func TestRegisterDialectorValidatesInput(t *testing.T) {
	if err := RegisterDialector("", func(string) gorm.Dialector { return nil }); err == nil {
		t.Fatal("empty name must fail")
	}
	if err := RegisterDialector("custom-nil", nil); err == nil {
		t.Fatal("nil factory must fail")
	}
}
