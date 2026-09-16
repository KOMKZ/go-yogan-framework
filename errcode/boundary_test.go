package errcode

import (
	"errors"
	"strings"
	"testing"
)

func TestCaptureIntoNilPointerIsNoop(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("CaptureInto(nil) panicked: %v", r)
		}
	}()
	CaptureInto(nil, "noop")
}

func TestCaptureIntoNilErrorIsNoop(t *testing.T) {
	var err error
	CaptureInto(&err, "noop")
	if err != nil {
		t.Fatalf("expected err remain nil, got %v", err)
	}
}

func TestCaptureIntoPlainErrorBecomesLayeredWithOperation(t *testing.T) {
	var err error = errors.New("plain failure")
	CaptureInto(&err, "users.service.bind_phone")

	le, ok := err.(*LayeredError)
	if !ok {
		t.Fatalf("expected *LayeredError, got %T", err)
	}
	if le.Operation() != "users.service.bind_phone" {
		t.Fatalf("operation = %q, want users.service.bind_phone", le.Operation())
	}
	if le.OriginStack() == "" {
		t.Fatalf("expected non-empty origin stack")
	}
	if le.Cause() == nil || le.Cause().Error() != "plain failure" {
		t.Fatalf("cause lost, got %v", le.Cause())
	}
}

func TestCaptureIntoPreservesInnermostOriginStack(t *testing.T) {
	// 模拟 I/O 边界已 Capture；service 入口再 CaptureInto 应该保留内层栈。
	ioErr := Capture(errors.New("connection refused"), "queue.enqueue").(*LayeredError)
	business := New(25, 1, "users", "users.queue.failed", "队列失败").Wrap(ioErr)

	var finalErr error = business
	CaptureInto(&finalErr, "users.service.publish")

	le, ok := finalErr.(*LayeredError)
	if !ok {
		t.Fatalf("expected *LayeredError, got %T", finalErr)
	}
	if le.OriginStack() != ioErr.OriginStack() {
		t.Fatalf("innermost origin stack was overwritten: %q -> %q", ioErr.OriginStack(), le.OriginStack())
	}
	if le.Operation() != "queue.enqueue" {
		t.Fatalf("operation should remain queue.enqueue (innermost), got %q", le.Operation())
	}
}

func TestCaptureIntoFillsOperationWhenLayeredErrorMissingIt(t *testing.T) {
	le := New(25, 1, "test", "test.no_op", "no operation")
	var err error = le
	CaptureInto(&err, "test.service.foo")

	got, ok := err.(*LayeredError)
	if !ok {
		t.Fatalf("expected *LayeredError, got %T", err)
	}
	if got.Operation() != "test.service.foo" {
		t.Fatalf("operation = %q, want test.service.foo", got.Operation())
	}
	if got.OriginStack() == "" {
		t.Fatalf("origin stack should be computed when missing")
	}
}

func TestCaptureIntoTwiceIsIdempotent(t *testing.T) {
	var err error = errors.New("plain")
	CaptureInto(&err, "first")
	first := err
	CaptureInto(&err, "second")

	if err != first {
		t.Fatalf("second CaptureInto should not replace already-captured error")
	}
	if le, ok := err.(*LayeredError); ok {
		if le.Operation() != "first" {
			t.Fatalf("operation overwritten: %q", le.Operation())
		}
	}
}

func TestBoundaryDelegatesToCaptureInto(t *testing.T) {
	var err error = errors.New("boundary test")
	Boundary(&err, "boundary.operation")

	le, ok := err.(*LayeredError)
	if !ok {
		t.Fatalf("expected *LayeredError, got %T", err)
	}
	if !strings.Contains(le.OriginStack(), "boundary_test.go") {
		t.Fatalf("expected origin to point at boundary_test.go, got %s", le.OriginStack())
	}
	if le.Operation() != "boundary.operation" {
		t.Fatalf("operation = %q, want boundary.operation", le.Operation())
	}
}
