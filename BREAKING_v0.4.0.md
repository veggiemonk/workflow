# Summary of Changes

This document summarizes the major changes made to the codebase to fix compilation and runtime errors.

## 1. API Changes for `Parallel` and `Select`

The `Parallel` and `Select` functions in `workflow.go` have been updated to accept a `name` parameter. This parameter is used for logging and identification purposes.

**Old Signature:**
```go
func Parallel[T any](mid []Middleware[T], merge MergeRequest[T], tasks ...*Task[T]) *parallel[T]
func Select[T any](mid []Middleware[T], s Selector[T], ifTask, elseTask *Task[T]) Step[T]
```

**New Signature:**
```go
func Parallel[T any](mid []Middleware[T], name string, merge MergeRequest[T], tasks ...*Task[T]) *parallel[T]
func Select[T any](mid []Middleware[T], name string, s Selector[T], ifTask, elseTask *Task[T]) Step[T]
```

All call sites of these functions have been updated to provide the new `name` parameter.

## 2. Middleware Refactoring

The `Middleware` type has been refactored to operate on `*Task[T]` instead of `Step[T]`. This provides more context to the middleware and allows it to modify the task itself.

**Old Definition:**
```go
type Middleware[T any] func(s Step[T]) Step[T]
```

**New Definition:**
```go
type Middleware[T any] func(s *Task[T]) *Task[T]
```

All middleware implementations have been updated to match the new signature. This involved wrapping the returned `Step` in a `NewTask` call.

## 3. Pipeline `Steps` to `Tasks`

The `Steps` field in the `Pipeline` struct has been renamed to `Tasks`. The type of the field has also been changed from a slice of `Step[T]` to a slice of `*Task[T]`.

**Old Definition:**
```go
type Pipeline[T any] struct {
	Steps      []Step[T]
	Middleware []Middleware[T]
}
```

**New Definition:**
```go
type Pipeline[T any] struct {
	Tasks      []*Task[T]
	Middleware []Middleware[T]
}
```

All usages of `pipeline.Steps` have been updated to `pipeline.Tasks`. The steps are now wrapped in `NewTask` before being added to the pipeline.

## 4. Logging Name Fix

The `Name` function in `workflow.go` has been updated to correctly extract the name of a step, even when it's wrapped in a `Task`.

The logging middlewares have been updated to use this `Name` function to ensure that the correct step name is logged, which fixed several failing tests.
