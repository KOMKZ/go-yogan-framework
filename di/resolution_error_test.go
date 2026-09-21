// 本文件验证 samber/do 缺失依赖错误的紧凑化契约。
// 测试保留完整诊断和 errors.Is 链，同时阻止全量服务清单进入主错误文本。
package di

import (
	"errors"
	"strings"
	"testing"

	"github.com/samber/do/v2"
	"github.com/stretchr/testify/require"
)

type resolutionTestRoot struct{}
type resolutionTestMissing struct{}

func TestNormalizeResolutionErrorCompactsMissingService(t *testing.T) {
	injector := do.New()
	do.Provide(injector, func(i do.Injector) (*resolutionTestRoot, error) {
		if _, err := do.Invoke[*resolutionTestMissing](i); err != nil {
			return nil, err
		}
		return &resolutionTestRoot{}, nil
	})

	_, rawErr := do.Invoke[*resolutionTestRoot](injector)
	require.Error(t, rawErr)
	require.Contains(t, rawErr.Error(), "available services:")

	compactErr := NormalizeResolutionError(injector, rawErr)
	var resolutionErr *ResolutionError
	require.ErrorAs(t, compactErr, &resolutionErr)
	require.ErrorIs(t, compactErr, do.ErrServiceNotFound)
	require.NotContains(t, compactErr.Error(), "available services:")
	require.Equal(t, "*github.com/KOMKZ/go-yogan-framework/di.resolutionTestMissing", resolutionErr.MissingService())
	require.Equal(t, []string{
		"*github.com/KOMKZ/go-yogan-framework/di.resolutionTestRoot",
		"*github.com/KOMKZ/go-yogan-framework/di.resolutionTestMissing",
	}, resolutionErr.DependencyPath())
	require.Len(t, resolutionErr.RegisteredServices(), 1)
	require.Contains(t, compactErr.Error(), "registered services: 1")
}

func TestNormalizeResolutionErrorLeavesOtherErrorsUntouched(t *testing.T) {
	originalErr := errors.New("database unavailable")

	require.Same(t, originalErr, NormalizeResolutionError(do.New(), originalErr))
	require.NoError(t, NormalizeResolutionError(do.New(), nil))
}

func TestNormalizeResolutionErrorUsesMissingServiceAsTopLevelPath(t *testing.T) {
	injector := do.New()
	_, rawErr := do.Invoke[*resolutionTestMissing](injector)

	compactErr := NormalizeResolutionError(injector, rawErr)
	var resolutionErr *ResolutionError
	require.ErrorAs(t, compactErr, &resolutionErr)
	require.Equal(t, []string{resolutionErr.MissingService()}, resolutionErr.DependencyPath())
}

func TestNormalizeResolutionErrorFallsBackWhenUpstreamFormatChanges(t *testing.T) {
	injector := do.New()
	do.ProvideValue(injector, "registered")
	rawErr := fmtWrapServiceNotFound("unexpected upstream wording")

	compactErr := NormalizeResolutionError(injector, rawErr)
	var resolutionErr *ResolutionError
	require.ErrorAs(t, compactErr, &resolutionErr)
	require.Empty(t, resolutionErr.MissingService())
	require.Empty(t, resolutionErr.DependencyPath())
	require.NotContains(t, compactErr.Error(), "unexpected upstream wording")
	require.True(t, strings.HasPrefix(compactErr.Error(), "DI: dependency resolution failed"))
}

func TestNormalizeResolutionErrorIsIdempotentAndReturnsDefensiveCopies(t *testing.T) {
	original := &ResolutionError{
		missingService:     "missing",
		dependencyPath:     []string{"root", "missing"},
		registeredServices: []string{"registered"},
		cause:              do.ErrServiceNotFound,
	}
	require.Same(t, original, NormalizeResolutionError(do.New(), original))

	path := original.DependencyPath()
	services := original.RegisteredServices()
	path[0] = "changed"
	services[0] = "changed"
	require.Equal(t, []string{"root", "missing"}, original.DependencyPath())
	require.Equal(t, []string{"registered"}, original.RegisteredServices())
	require.ErrorIs(t, original, do.ErrServiceNotFound)
}

func TestResolutionErrorNilReceiverAndParserFallbacks(t *testing.T) {
	var resolutionErr *ResolutionError
	require.Equal(t, "DI: dependency resolution failed", resolutionErr.Error())
	require.NoError(t, resolutionErr.Unwrap())
	require.Empty(t, resolutionErr.MissingService())
	require.Empty(t, resolutionErr.DependencyPath())
	require.Empty(t, resolutionErr.RegisteredServices())

	missing, path := parseResolutionDetails(errors.New("DI: could not find service `unterminated"))
	require.Empty(t, missing)
	require.Empty(t, path)

	compactErr := NormalizeResolutionError(nil, resolutionTestIsOnlyError{})
	var fallback *ResolutionError
	require.ErrorAs(t, compactErr, &fallback)
	require.Empty(t, fallback.RegisteredServices())
}

func fmtWrapServiceNotFound(message string) error {
	return &resolutionTestWrappedError{message: message, cause: do.ErrServiceNotFound}
}

type resolutionTestWrappedError struct {
	message string
	cause   error
}

func (e *resolutionTestWrappedError) Error() string { return e.message }
func (e *resolutionTestWrappedError) Unwrap() error { return e.cause }

type resolutionTestIsOnlyError struct{}

func (resolutionTestIsOnlyError) Error() string { return "opaque DI error" }
func (resolutionTestIsOnlyError) Is(target error) bool {
	return target == do.ErrServiceNotFound
}
