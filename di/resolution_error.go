// 本文件负责把 samber/do 的缺失依赖错误转换为稳定、紧凑的框架错误契约。
// 主错误文本只保留缺失服务、依赖路径和注册数量；完整服务快照由诊断访问器提供，
// application 生命周期可将其写入 DEBUG 结构化字段。维护时不得把全量清单重新拼回 Error()。
package di

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/samber/do/v2"
)

// ResolutionError 表示 DI 解析期间缺少服务。
// 它通过 Unwrap 保留 samber/do 原始错误链，供 errors.Is/As 与底层诊断继续使用。
type ResolutionError struct {
	missingService     string
	dependencyPath     []string
	registeredServices []string
	cause              error
}

// Error 返回适合 ERROR/FATAL 主日志的紧凑摘要。
func (e *ResolutionError) Error() string {
	if e == nil {
		return "DI: dependency resolution failed"
	}
	if e.missingService == "" {
		return fmt.Sprintf("DI: dependency resolution failed, registered services: %d", len(e.registeredServices))
	}

	message := fmt.Sprintf("DI: missing service `%s`", e.missingService)
	if len(e.dependencyPath) > 0 {
		message += ", path: " + quotedDependencyPath(e.dependencyPath)
	}
	return fmt.Sprintf("%s, registered services: %d", message, len(e.registeredServices))
}

// Unwrap 保留第三方 DI 错误链。
func (e *ResolutionError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.cause
}

// MissingService 返回无法解析的服务类型名；上游格式无法识别时为空。
func (e *ResolutionError) MissingService() string {
	if e == nil {
		return ""
	}
	return e.missingService
}

// DependencyPath 返回从根服务到缺失服务的解析路径副本。
func (e *ResolutionError) DependencyPath() []string {
	if e == nil {
		return nil
	}
	return append([]string(nil), e.dependencyPath...)
}

// RegisteredServices 返回错误发生时的完整服务快照副本，仅用于服务端诊断。
func (e *ResolutionError) RegisteredServices() []string {
	if e == nil {
		return nil
	}
	return append([]string(nil), e.registeredServices...)
}

// NormalizeResolutionError 将 samber/do 的缺失服务错误转换为紧凑 typed error。
// 非 DI 缺失错误保持原样；解析上游文案失败时仍返回不包含原始长文案的安全摘要。
func NormalizeResolutionError(injector do.Injector, err error) error {
	if err == nil {
		return nil
	}
	var existing *ResolutionError
	if errors.As(err, &existing) {
		return err
	}
	if !errors.Is(err, do.ErrServiceNotFound) && !errors.Is(err, do.ErrServiceNotMatch) {
		return err
	}

	missing, path := parseResolutionDetails(findResolutionCause(err))
	return &ResolutionError{
		missingService:     missing,
		dependencyPath:     path,
		registeredServices: providedServiceNames(injector),
		cause:              err,
	}
}

func findResolutionCause(err error) error {
	for current := err; current != nil; current = errors.Unwrap(current) {
		next := errors.Unwrap(current)
		if next == do.ErrServiceNotFound || next == do.ErrServiceNotMatch {
			return current
		}
	}
	return nil
}

func parseResolutionDetails(err error) (string, []string) {
	if err == nil {
		return "", nil
	}
	// samber/do v2 暂未公开 typed missing-service/path 字段；升级为 typed API 后删除本解析与豁免。
	// errlint:ignore LINT-ERR-008 reason=samber-do-v2-string-only-diagnostic until=2027-09-20
	message := err.Error()
	firstQuote := strings.IndexByte(message, '`')
	if firstQuote < 0 {
		return "", nil
	}
	rest := message[firstQuote+1:]
	secondQuote := strings.IndexByte(rest, '`')
	if secondQuote < 0 {
		return "", nil
	}
	missing := rest[:secondQuote]

	const pathMarker = ", path: "
	pathIndex := strings.LastIndex(message, pathMarker)
	if pathIndex < 0 {
		return missing, []string{missing}
	}
	return missing, parseQuotedPath(message[pathIndex+len(pathMarker):])
}

func parseQuotedPath(path string) []string {
	parts := strings.Split(path, " -> ")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		name := strings.Trim(strings.TrimSpace(part), "`")
		if name != "" {
			result = append(result, name)
		}
	}
	return result
}

func providedServiceNames(injector do.Injector) []string {
	if injector == nil {
		return nil
	}
	descriptions := injector.ListProvidedServices()
	names := make([]string, 0, len(descriptions))
	for _, description := range descriptions {
		names = append(names, description.Service)
	}
	sort.Strings(names)
	return names
}

func quotedDependencyPath(path []string) string {
	quoted := make([]string, 0, len(path))
	for _, name := range path {
		quoted = append(quoted, "`"+name+"`")
	}
	return strings.Join(quoted, " -> ")
}
