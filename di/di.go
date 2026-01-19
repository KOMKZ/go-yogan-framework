// Package di provides dependency injection utilities based on samber/do.
package di

import "github.com/samber/do/v2"

// Injector 类型别名
type Injector = do.Injector

// RootScope 类型别名
type RootScope = do.RootScope

// New 创建新的根注入器
var New = do.New

// NewWithOpts 使用选项创建新的根注入器
var NewWithOpts = do.NewWithOpts

// 注意：以下泛型函数不能直接导出为 var，需要通过包名调用
// - do.Provide[T](injector, provider)
// - do.ProvideNamed[T](injector, name, provider)
// - do.ProvideValue[T](injector, value)
// - do.Invoke[T](injector)
// - do.InvokeNamed[T](injector, name)
// - do.MustInvoke[T](injector)
// - do.MustInvokeNamed[T](injector, name)
//
// 使用示例:
//   injector := di.New()
//   do.Provide(injector, func(i do.Injector) (*MyService, error) {
//       return &MyService{}, nil
//   })
//   svc := do.MustInvoke[*MyService](injector)
