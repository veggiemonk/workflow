# Copilot instructions for `veggiemonk/workflow`

This repository is a small Go library that composes typed units of work into a
pipeline. It needs Go 1.27 and has no dependency outside the standard library.
Keep both true.

## The one abstraction

```go
type Step[I, O any] struct{ /* unexported */ }

func (s Step[I, O]) Run(ctx context.Context, in I) (O, error)
```

- `Step` is a **concrete struct**, not an interface. Go allows type parameters
  on a method only when the receiver is a concrete type, and a generic method
  (`Then[P any]`) is what lets a chain change type. Do not propose turning
  `Step` into an interface.
- The extension point for a type that carries state is `Runner[I, O]`, a plain
  interface, wrapped with `Of`.
- A step is **immutable**. Every method returns a new `Step` and writes nothing
  to its receiver. Middleware is applied when the step is built, never while it
  runs.

Build: `Func`, `Pure`, `Of`, `Identity`.
Compose: `Then`, `Map`, `Par`, `Fan`, `Each`, `Seq`, `If`.
Middleware: `Use`, `Retry`, `Timeout`, `Recover`, `Log`, `WithID`, `Breaker`.

`Each` is a function, not a method: a method returning `Step[[]I, []O]` would
make the compiler instantiate `Step[[][]I, [][]O]` without end, which the
compiler rejects as an instantiation cycle. `Fan`, `Seq` and `If` are functions
because they take a list of steps, or two branches, of one shape.

## Conventions

- A step takes what it needs and returns what it produced. Do not introduce a
  struct that carries the whole run; that shape is what v0.4.0 removed.
- The concurrent combinators run every branch to the end and return every
  error joined with `errors.Join`. They turn a panic in a branch into a
  `*PanicError`. Keep that.
- They do not copy the input. A branch reads; a branch that must write makes
  its own copy.
- `Log` never records the payload.
- `Timeout` starts no goroutine; the step honours the context.
- A `CircuitBreaker` is an explicit value the caller creates and shares.
- Doc comments are plain sentences that say why, not what the code already
  says.

## Workflows

- Build: `go build ./...`
- Test: `make test` (race, shuffle, coverage), or `go test ./...`
- Lint: `make lint`, vulnerabilities: `make vuln`
- Docs: `make docs` regenerates `docs/llms.md` from `go doc -all`.
- Examples: `make examples`. Each example under `examples/` is its own module
  with a `replace` directive to the working tree.
- Debug a pipeline: print it. `String()` gives the tree as it was declared.

## Key files

- `step.go`: `Step`, `Runner`, the constructors, `Run`, `String`.
- `compose.go`: `Then`, `Map`, `Par`, `Fan`, `Each`, `Seq`, `If`.
- `middleware.go`: `Middleware`, the built-in middleware, `CircuitBreaker`.
- `panic.go`: `PanicError`.
- `example_test.go`: runnable examples.
- `README.md`, `docs/architecture.md`, `docs/best-practices.md`.

## Example

```go
parse := workflow.Func("Parse", func(ctx context.Context, s string) (Doc, error) { … })
count := workflow.Pure("Count", func(d Doc) int { … })
uniq  := workflow.Pure("Unique", func(d Doc) int { … })

analyse := count.Par(uniq, func(n, u int) (Report, error) {
    return Report{Words: n, Unique: u}, nil
})

pipeline := parse.Then(analyse).Retry(workflow.RetryConfig{MaxAttempts: 3})
batch := workflow.Each(8, pipeline) // Step[[]string, []Report]
```
