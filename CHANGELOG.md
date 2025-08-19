# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/veggiemonk/workflow/compare/v0.1.0...HEAD
[v0.1.0]: https://github.com/veggiemonk/workflow/releases/tag/v0.1.0
