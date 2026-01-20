package errcode

import (
	"testing"
)

// TestRegistry_Register test register error codes
func TestRegistry_Register(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	err2 := New(20, 1, "order", "error.order.not_found", "订单不存在")

	registry.Register(err1)
	registry.Register(err2)

	if registry.Count() != 2 {
		t.Errorf("expected 2 registered codes, got %d", registry.Count())
	}

	codes := registry.GetAll()
	if codes[100001] != "user:error.user.not_found" {
		t.Errorf("expected 'user:error.user.not_found', got %s", codes[100001])
	}
	if codes[200001] != "order:error.order.not_found" {
		t.Errorf("expected 'order:error.order.not_found', got %s", codes[200001])
	}
}

// TestRegistry_Register_Duplicate Testing idempotence for duplicate registration
func TestRegistry_Register_Duplicate(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	err2 := New(10, 1, "user", "error.user.not_found", "User not found")

	registry.Register(err1)
	registry.Register(err2) // idempotent, will not panic

	if registry.Count() != 1 {
		t.Errorf("expected 1 registered code, got %d", registry.Count())
	}
}

// TestRegistry_Register_Conflict test error code conflict (panic)
func TestRegistry_Register_Conflict(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	err2 := New(10, 1, "user", "error.user.exists", "用户已存在") // Error code is the same, but msgKey is different

	registry.Register(err1)

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic for conflicting error code")
		}
	}()

	registry.Register(err2) // Should panic
}

// TestRegistry_Lock test registry lock
func TestRegistry_Lock(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	registry.Register(err1)

	registry.Lock()

	if !registry.IsLocked() {
		t.Errorf("registry should be locked")
	}

	// Trying to register a new error code should cause a panic
	err2 := New(10, 2, "user", "error.user.exists", "用户已存在")
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic when registering after lock")
		}
	}()

	registry.Register(err2)
}

// TestRegistry_Unlock test registry unlock
func TestRegistry_Unlock(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	registry.Lock()
	if !registry.IsLocked() {
		t.Errorf("registry should be locked")
	}

	registry.Unlock()
	if registry.IsLocked() {
		t.Errorf("registry should be unlocked")
	}

	// Unlocked, registration is available
	err := New(10, 1, "user", "error.user.not_found", "User not found")
	registry.Register(err)

	if registry.Count() != 1 {
		t.Errorf("expected 1 registered code after unlock, got %d", registry.Count())
	}
}

// TestRegistry_Clear test clear registry
func TestRegistry_Clear(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	err2 := New(20, 1, "order", "error.order.not_found", "订单不存在")

	registry.Register(err1)
	registry.Register(err2)
	registry.Lock()

	if registry.Count() != 2 {
		t.Errorf("expected 2 registered codes, got %d", registry.Count())
	}

	registry.Clear()

	if registry.Count() != 0 {
		t.Errorf("expected 0 codes after clear, got %d", registry.Count())
	}

	if registry.IsLocked() {
		t.Errorf("registry should be unlocked after clear")
	}
}

// TestGlobalRegistry test global registry
func TestGlobalRegistry(t *testing.T) {
	// Clear global registry (before testing)
	ClearGlobalRegistry()

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	err2 := New(20, 1, "order", "error.order.not_found", "订单不存在")

	Register(err1)
	Register(err2)

	if GetRegistryCount() != 2 {
		t.Errorf("expected 2 registered codes, got %d", GetRegistryCount())
	}

	codes := GetAllRegisteredCodes()
	if codes[100001] != "user:error.user.not_found" {
		t.Errorf("expected 'user:error.user.not_found', got %s", codes[100001])
	}

	// Clear global registry (after testing)
	ClearGlobalRegistry()
}

// TestGlobalRegistry_Lock test global registry lock
func TestGlobalRegistry_Lock(t *testing.T) {
	// Clear global registry (before testing)
	ClearGlobalRegistry()

	err1 := New(10, 1, "user", "error.user.not_found", "User not found")
	Register(err1)

	LockGlobalRegistry()

	if !IsGlobalRegistryLocked() {
		t.Errorf("global registry should be locked")
	}

	// Registering new error codes should cause a panic
	err2 := New(10, 2, "user", "error.user.exists", "用户已存在")
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic when registering after lock")
		}
		// Unlock and clear (after testing)
		UnlockGlobalRegistry()
		ClearGlobalRegistry()
	}()

	Register(err2)
}

// TestRegistry_ConcurrentRegister test concurrent registration
func TestRegistry_ConcurrentRegister(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			err := New(10+idx, 1, "module", "error.key", "message")
			registry.Register(err)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if registry.Count() != 10 {
		t.Errorf("expected 10 registered codes, got %d", registry.Count())
	}
}

// TestRegistry_ConcurrentGetAll tests concurrent read operations
func TestRegistry_ConcurrentGetAll(t *testing.T) {
	registry := &Registry{codes: make(map[int]string)}

	// Pre-register some error codes
	for i := 0; i < 10; i++ {
		err := New(10+i, 1, "module", "error.key", "message")
		registry.Register(err)
	}

	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			codes := registry.GetAll()
			if len(codes) != 10 {
				t.Errorf("expected 10 codes, got %d", len(codes))
			}
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}

