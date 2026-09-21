# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Fixed

- `Pipeline.Run` and `Sequential.Run` no longer rewrite their own step slice
  while they run. Each run wrapped the steps again, so a middleware fired once
  on the first run, twice on the second and three times on the third, and a
  `Retry(3)` became 9 attempts. Two concurrent runs also raced on the slice.
  Middleware is now applied to a local copy for the length of the call.
- A panic in a `Parallel` branch is returned as a `PanicError`. It used to be
  logged and swallowed: the goroutine returned nil, `errgroup` saw success, the
  branch result stayed nil, and mergo then panicked on the nil pointer, which
  stopped the program.
- `Merge` and `MergeTransform` skip a nil response instead of panicking.

### Added

- `PanicError`, returned by `Parallel` when a branch panics.

### Changed

- `Pipeline.String()` and `Sequential.String()` now print the pipeline as it
  was declared. They used to show the middleware wrappers that a previous
  `Run` had left behind.

### Deprecated

- `CapturePanic` swallows the panic and leaves the caller with a zero result
  and no error. `Parallel` no longer uses it.

### Documented

- `Merge` keeps the first branch; it does not add the branches together. Two
  parallel branches that each add 1 to the same counter give 1, not 2. This
  follows from `Step[T]` reading and returning the same type, so a patch
  cannot remove it. `Merge`, `MergeTransform`, `Parallel`, the README and
  `docs/llms.md` now say so, and `TestMergeDoesNotCombineValues` pins the
  behaviour. Pass your own `MergeRequest`, or use `workflow/v2`.

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

[Unreleased]: https://github.com/veggiemonk/workflow/compare/v0.3.0...HEAD
[v0.3.0]: https://github.com/veggiemonk/workflow/compare/v0.2.0...v0.3.0
[v0.2.0]: https://github.com/veggiemonk/workflow/compare/v0.1.0...v0.2.0
[v0.1.0]: https://github.com/veggiemonk/workflow/releases/tag/v0.1.0
