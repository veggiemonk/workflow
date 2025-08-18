# Copilot Instructions for `veggiemonk/workflow`

You are an senior software engineer with 20+ years of experience with engineering best practices, design patterns, and software architecture. You are working on a Go-based workflow engine that emphasizes modularity and reusability.

This repository implements a generic, extensible workflow engine in Go. Use these guidelines to maximize productivity and maintain consistency when contributing or using AI coding agents.

## Architecture Overview
- **Core Abstractions:**
  - `Step[T]`: Interface for a unit of work. Implement `Run(context.Context, *T) (*T, error)`.
  - `Pipeline[T]`: Orchestrates a sequence of steps, supporting both sequential and parallel execution.
  - `Series[T]` and `Parallel[T]`: Compose steps for sequential or concurrent execution. `Parallel` uses a merge function to combine results.
  - `Middleware[T]`: Wraps steps for cross-cutting concerns (e.g., logging, metrics).
- **Context Awareness:** All steps and pipelines use `context.Context` for cancellation and deadlines.
- **Type Safety:** The engine is generic (`[T any]`), allowing strong typing for workflow data.

## Developer Workflows
- **Build:**
  - Standard Go build: `go build ./...`
- **Test:**
  - Run all tests: `go test ./...`
  - Example-based tests: see `workflow_example_test.go` for usage patterns.
- **Debug:**
  - Use the `String()` method on pipelines to print their structure for inspection.
  - Middleware (e.g., `logMiddleware`) can be used to trace step execution.

## Project-Specific Patterns
- **Custom Steps:**
  - Implement the `Step[T]` interface or use `StepFunc[T]` for inline steps.
- **Composing Workflows:**
  - Use `Series` for sequential, `Parallel` for concurrent, and `Select` for conditional execution.
  - Merge functions are required for parallel steps to combine results.
- **Middleware Usage:**
  - Wrap steps with middleware for logging, error handling, etc. See `logMiddleware` in `README.md` for an example.
- **Extensibility:**
  - Add new step types by implementing the `Step[T]` interface.

## Integration Points
- **No external service dependencies**—the engine is self-contained and generic.
- **Import as a Go module:** `go get github.com/veggiemonk/workflow`

## Key Files
- `workflow.go`: Core engine abstractions and implementations.
- `workflow_example_test.go`: Usage examples and patterns.
- `README.md`: High-level documentation and code samples.

## Example: Defining a Pipeline
```go
p := wf.NewPipeline(logMiddleware[Result](&buf))
p.Steps = []wf.Step[Result]{
    wf.StepFunc[Result](...),
    wf.Series(nil, ...),
    wf.Parallel(nil, wf.Merge[Result], ...),
}
_, err := p.Run(context.Background(), &Result{})
```

---
If any conventions or workflows are unclear or missing, please provide feedback for further refinement.
