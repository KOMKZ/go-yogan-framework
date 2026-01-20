// Package di provides dependency injection utilities based on samber/do.
package di

import "github.com/samber/do/v2"

// Injector type alias
type Injector = do.Injector

// RootScope type alias
type RootScope = do.RootScope

// Create new root injector
var New = do.New

// Create a new root injector using options
var NewWithOpts = do.NewWithOpts

// Note: The following generic function cannot be exported as var directly; it needs to be called via package name
// - do.Provide[T](injector, provider)
// - do.ProvideNamed[T](injector, name, provider)
// - do.ProvideValue[T](injector, value)
// - do.Invoke[T](injector)
// - do.InvokeNamed[T](injector, name)
// - do.MustInvoke[T](injector)
// - do.MustInvokeNamed[T](injector, name)
//
// Usage example:
//   injector := di.New()
//   do.Provide(injector, func(i do.Injector) (*MyService, error) {
//       return &MyService{}, nil
//   })
//   svc := do.MustInvoke[*MyService](injector)
