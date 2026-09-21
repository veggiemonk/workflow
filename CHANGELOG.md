# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [v0.4.0] - 2026-09-21

v0.4.0 replaces the whole API. The library is below v1.0.0, so the break lands
here instead of in a `/v2` import path: **the import path does not change**.
Pin v0.3.0 if you need the old API.

The code that v0.4.0 ships is the `v2/` directory of the repository, moved to
the root. Nothing else of v0.3.0 remains.

### Changed

**BREAKING. A step declares its own input type and its own output type.**

```go
// v0.3.0
type Step[T any] interface{ Run(context.Context, *T) (*T, error) }

// v0.4.0
type Step[I, O any] struct{ /* unexported */ }
func (s Step[I, O]) Run(ctx context.Context, in I) (O, error)
```

One type no longer has to carry the whole pipeline. Everything below follows
from that.

- **BREAKING.** `Step` is a concrete struct, not an interface. Go 1.27 allows
  type parameters on a method only when the receiver is a concrete type, and a
  generic method is what lets a chain change type. To write a step on a type
  that carries state, implement the `Runner[I, O]` interface and wrap it with
  `Of`.
- **BREAKING.** A step is immutable. Every method returns a new step and writes
  nothing to its receiver. Middleware is applied when the step is built, never
  while it runs.
- **BREAKING.** Middleware is a method on the step that needs it, not a
  setting on a pipeline: `Retry`, `Timeout`, `Recover`, `Log`, `WithID`,
  `Breaker`, and `Use` for your own.
- **BREAKING.** A `CircuitBreaker` is an explicit value that you create and
  share. In v0.3.0 the state sat in a closure, so every step the middleware
  wrapped shared one circuit without saying so.
- **BREAKING.** `Timeout` starts no goroutine. Go cannot stop a goroutine from
  outside, so the step must honour the context; a step that does no I/O must
  test `ctx.Err()` itself. v0.3.0 raced a goroutine and leaked it.
- **BREAKING.** `Log` never records the payload. A payload can hold a secret,
  and formatting it costs more than a short step.
- The concurrent combinators run every branch to the end and return every
  error joined with `errors.Join`. They do not stop at the first failure.
- The examples and the documentation were rewritten for the new API.
  `docs/llms.md` is now generated from `go doc -all`, because `gomarkdoc`
  cannot parse a generic method.

### Added

- `Func`, `Pure`, `Of` and `Identity` build a step. `Runner[I, O]` is the
  extension point for a type that carries state.
- `Then`, `Map` and `Par` are methods; `Fan`, `Each`, `Seq` and `If` are
  functions. `Each(n, s)` applies a step to every element of a slice, at most
  `n` at a time.
- `Recover` turns a panic into a `*PanicError`. `Par`, `Fan` and `Each` do it
  for every branch already, because a panic in a goroutine stops the program.
- `Step.Name`, `Step.Rename`, and `StepID(ctx)` for the UUID that `WithID`
  puts in the context.
- `ErrNilStep` for a zero step, and `ErrCircuitOpen` for an open circuit.
- CI now builds and tests every example module.

### Removed

**BREAKING.** Every one of these has a replacement in the table below:
`Pipeline`, `NewPipeline`, `StepFunc`, `Sequential`, `Series`, `Parallel`,
`Select`, `MergeRequest`, `Merge`, `MergeTransform`, `SafeCopy`,
`DeepCopyInterface`, `CapturePanic`, `MidFunc`, `Name`, `StepValidator`,
`SafeRun`, `RetryMiddleware`, `TimeoutMiddleware`, `LoggerMiddleware`,
`UUIDMiddleware`, `CircuitBreakerMiddleware`.

- **BREAKING.** The library has no dependency outside the standard library.
  `dario.cat/mergo`, `github.com/google/uuid`, `golang.org/x/sync` and
  `github.com/ccoveille/go-safecast` are gone, and so is `go.sum`.

### Fixed

The three faults of v0.3.0 cannot occur in v0.4.0, because the code that
caused them is gone:

- A merge no longer drops a parallel result in silence. There is no reflective
  merge; you pass a typed join function, and the compiler checks it.
- A deep copy no longer falls back to a shallow copy in silence. Branches
  return their own type, so nothing is copied.
- A second run no longer adds a second layer of middleware. `Pipeline.Run`
  rewrote its own step slice, which turned a `Retry(3)` into nine attempts on
  the third run and raced between two concurrent runs.
- A panic in a branch is returned as a `*PanicError` instead of being logged
  and swallowed.

### Migration

| v0.3.0 | v0.4.0 |
| --- | --- |
| `Step[T]` interface | `Step[I,O]` struct, or the `Runner[I,O]` interface |
| `StepFunc[T](f)` | `Func(name, f)` or `Pure(name, f)` |
| `NewPipeline(mid...)` + `p.Steps = …` | `a.Then(b).Then(c)`, or `Seq(name, …)` |
| `Sequential(mid, steps...)` | `Seq(name, steps...)` |
| `Parallel(mid, Merge, steps...)` | `s.Par(other, join)` or `Fan(name, join, steps...)` |
| `MergeRequest`, `Merge`, `MergeTransform` | the `join` function you pass |
| `SafeCopy`, `DeepCopyInterface` | not needed; branches return their own type |
| `Select(mid, pred, a, b)` | `If(name, pred, a, b)` |
| `RetryMiddleware(cfg)` | `s.Retry(cfg)` |
| `TimeoutMiddleware(d)` | `s.Timeout(d)` |
| `LoggerMiddleware(l)` | `s.Log(l)` |
| `UUIDMiddleware()` | `s.WithID()` |
| `CircuitBreakerMiddleware(cfg)` | `s.Breaker(NewCircuitBreaker(cfg))` |
| `CapturePanic` | `s.Recover()`, which returns a `*PanicError` |
| `Name(step)` (reflection) | `step.Name()` |
| `StepValidator`, `SafeRun` | the types check this; a zero step gives `ErrNilStep` |

A v0.3.0 pipeline that keeps one type throughout maps to `Seq[T]` with no
other change in shape, because `Step[T]` is `Step[T,T]`.

## [v0.3.0] - 2025-08-21

### Added
- Comprehensive unit tests for all core middleware in `middleware_test.go`:
  - RetryMiddleware: success, failure, custom retry logic, exponential backoff.
  - TimeoutMiddleware: step completion, timeout, instant execution.
  - CircuitBreakerMiddleware: open/close logic, custom tripping, recovery.
  - LoggerMiddleware: log output for both success and failure, custom logger injection.
  - UUIDMiddleware: unique UUID per execution and context propagation.
- Helper types for test logging and step simulation.
- Middleware composition and integration tests with Pipeline abstraction.
- Tests for default config values for retry and circuit breaker.
- Ensured all middleware are context-aware and type-safe, following project conventions.

## [v0.2.0] - 2025-08-19

### Added
- CI/CD workflow automation with GitHub Actions
- Comprehensive examples directory with basic, CI/CD, and advanced patterns
- Architecture documentation explaining design principles and extension points
- Best practices guide with common patterns and anti-patterns
- CHANGELOG.md for tracking project evolution

### Fixed
- Selector logic bug in workflow.go where else branch overwrote if branch selection
- Type inference issues in examples

### Improved
- Project structure with proper organization of examples and documentation
- Error handling patterns and documentation
- Testing patterns and examples

## [v0.1.0] - Initial Release

### Added
- Core workflow engine with generic type support
- Basic abstractions: Step, Pipeline, Series, Parallel, Select
- Middleware support for cross-cutting concerns
- Context-aware execution with cancellation support
- Built-in middleware: UUID tracking and logging
- Merge functions for parallel result aggregation
- String representation for pipeline visualization
- Basic test suite with examples

### Features
- **Type-safe workflows**: Full generic type support
- **Flexible composition**: Mix sequential, parallel, and conditional execution
- **Middleware system**: Extensible cross-cutting concerns
- **Context integration**: Cancellation and timeout support
- **Debuggable**: Tree visualization of pipeline structure
- **Concurrent execution**: Safe parallel processing with error groups

[Unreleased]: https://github.com/veggiemonk/workflow/compare/v0.4.0...HEAD
[v0.4.0]: https://github.com/veggiemonk/workflow/compare/v0.3.0...v0.4.0
[v0.3.0]: https://github.com/veggiemonk/workflow/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/veggiemonk/workflow/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/veggiemonk/workflow/releases/tag/v0.1.0
