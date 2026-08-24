package errcode

import (
	"errors"
	"strings"
	"testing"
)

func TestLayeredErrorWrapCapturesOriginForPlainError(t *testing.T) {
	base := New(25, 1, "test", "test.wrap", "包装失败")
	wrapped := base.Wrap(errors.New("root cause"))

	if !strings.Contains(wrapped.OriginStack(), "origin_stack_test.go") {
		t.Fatalf("expected wrap origin stack, got %s", wrapped.OriginStack())
	}
}

func TestLayeredErrorOriginStackIsPreservedWhenRewrapped(t *testing.T) {
	inner := New(25, 1, "test", "test.inner", "内部错误").Wrap(errors.New("root cause"))
	outer := New(25, 2, "test", "test.outer", "外层错误").Wrap(inner)

	if outer.OriginStack() != inner.OriginStack() {
		t.Fatalf("expected original origin stack to be preserved")
	}
}

func TestCapturePreservesTechnicalOriginThroughBusinessWrap(t *testing.T) {
	technical := Capture(errors.New("connection refused"), "queue.enqueue")
	wrapped := New(25, 5, "admin", "admin.export.failed", "导出失败").Wrap(technical)

	if wrapped.Operation() != "queue.enqueue" {
		t.Fatalf("expected operation queue.enqueue, got %q", wrapped.Operation())
	}
	if !strings.Contains(wrapped.OriginStack(), "origin_stack_test.go") {
		t.Fatalf("expected captured origin stack, got %s", wrapped.OriginStack())
	}
}

func TestCaptureNilReturnsNilError(t *testing.T) {
	if err := Capture(nil, "noop"); err != nil {
		t.Fatalf("Capture(nil) = %#v, want nil", err)
	}
}
