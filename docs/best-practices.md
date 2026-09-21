# Best practices

## Design

### Let the types carry the stages

A step declares what it reads and what it returns. Use that. A stage that
needs only a URL should take a URL, not the struct that holds the whole run.

```go
// Good: each stage says what it needs.
fetch := workflow.Func("Fetch", func(ctx context.Context, url string) (Page, error) { … })
parse := workflow.Func("Parse", func(ctx context.Context, p Page) (Doc, error) { … })

// Bad: one struct for everything, with fields that are empty half the time.
type State struct{ URL string; Page *Page; Doc *Doc; Err error }
```

A God struct brings back every fault that v0.3.0 had: branches that share
memory, merges that need reflection, and fields nobody can tell the lifetime
of.

### Name every step

The name is what `Log`, the printed tree and `PanicError` show. `Func("", …)`
prints as `Step[main.Page,main.Doc]`, which tells you the types and nothing
about the work.

### Build the pipeline once

A step is immutable, so build it at start-up, store it, and run it as often as
you like.

```go
var pipeline = parse.Then(analyse).Then(render) // package level is fine

func handler(w http.ResponseWriter, r *http.Request) {
    out, err := pipeline.Run(r.Context(), in)
}
```

Building inside the handler works, but it allocates the tree on every request
for no gain.

### Keep a step small enough to test alone

Every step is an ordinary value with one method. If a step needs a comment to
say what its three jobs are, it is three steps.

## Errors

### Return an error, not a zero value

The combinators propagate an error; they cannot see a zero value that means
"nothing happened".

```go
// Good
if r.Type == "" {
    return Processed{}, fmt.Errorf("record %s: no type", r.ID)
}
```

### Wrap with the identity of the work

```go
return Doc{}, fmt.Errorf("parse %s: %w", page.URL, err)
```

The step name is in the log line; the record ID is not, unless you put it
there.

### Expect joined errors from the concurrent combinators

`Par`, `Fan` and `Each` run every branch to the end and return every error
joined with `errors.Join`. Use `errors.Is` and `errors.As`, which walk a
joined error, instead of comparing with `==`.

```go
results, err := batch.Run(ctx, records)
if errors.Is(err, context.DeadlineExceeded) { … }

var pe *workflow.PanicError
if errors.As(err, &pe) {
    log.Error("branch panicked", "step", pe.Step, "value", pe.Value)
}
```

### Decide where a failure is fatal

A step that fails stops the chain it is in. To carry on instead, turn the
failure into a value and branch on it:

```go
tolerant := risky.Use(func(next workflow.Step[In, Out]) workflow.Step[In, Out] {
    return workflow.Func(next.Name(), func(ctx context.Context, in In) (Out, error) {
        out, err := next.Run(ctx, in)
        if err != nil {
            metrics.Degraded.Add(1)
            return fallback(in), nil
        }
        return out, nil
    })
})
```

Do this deliberately, at one place. A middleware that swallows every error
turns a broken pipeline into a quiet one.

## Concurrency

### Do not write to a shared input

`Par` and `Fan` give every branch the same input. They do not copy it, and
they will not: a copy is a cost the package cannot judge for you. Give them
steps that read.

```go
// Bad: both branches write to the same slice header's backing array.
left  := workflow.Pure("Left",  func(rs []Record) int { rs[0].Seen = true; … })

// Good: a branch returns what it computed.
left  := workflow.Pure("Left",  func(rs []Record) int { … })
right := workflow.Pure("Right", func(rs []Record) int { … })
```

If a branch must write, give it its own copy (`slices.Clone`, `maps.Clone`)
inside the step.

### Bound the fan-out

`Each(n, s)` runs at most `n` elements at a time. Pick `n` from the resource
the step contends on: the number of cores for CPU work, the size of the
connection pool for a database, whatever the API allows for an HTTP call.
`Each(0, …)` and a negative count mean unbounded; use that only for work that
blocks on nothing.

### Honour the context in a slow step

`Timeout` sets a deadline on the context. It cannot stop a goroutine, because
Go cannot. A step that does I/O through the standard library is covered. A
step that computes must check:

```go
for i, r := range records {
    if i%1000 == 0 && ctx.Err() != nil {
        return nil, ctx.Err()
    }
    …
}
```

### Put the breaker where the dependency is

One `CircuitBreaker` guards one dependency. Create it next to the client it
protects, and share it between every step that calls that dependency; do not
share one breaker between unrelated services.

```go
var paymentBreaker = workflow.NewCircuitBreaker(workflow.CircuitBreakerConfig{
    FailureThreshold: 5,
    OpenTimeout:      30 * time.Second,
})

charge := workflow.Of("Charge", gateway).Breaker(paymentBreaker)
refund := workflow.Of("Refund", gateway).Breaker(paymentBreaker)
```

## Middleware

### Order is what you write

`Use` applies the first middleware as the outermost layer, and the methods
wrap in the order you call them. `Retry` inside `Timeout` gives every attempt
one deadline; `Timeout` inside `Retry` gives the whole set of attempts one
deadline.

```go
fetch.Retry(cfg).Timeout(5*time.Second) // one deadline for all attempts
fetch.Timeout(time.Second).Retry(cfg)   // one second per attempt
```

Read the printed tree when in doubt: the outermost layer is at the top.

### Retry only what a retry can fix

```go
cfg := workflow.RetryConfig{
    MaxAttempts: 3,
    ShouldRetry: func(err error) bool {
        return errors.Is(err, errGatewayDown) || errors.Is(err, syscall.ECONNRESET)
    },
}
```

Without `ShouldRetry` every error is retried, including the ones that will
fail the same way three times.

### Write a middleware as a plain function

```go
func traced[I, O any](tracer trace.Tracer) workflow.Middleware[I, O] {
    return func(next workflow.Step[I, O]) workflow.Step[I, O] {
        return workflow.Func(next.Name(), func(ctx context.Context, in I) (O, error) {
            ctx, span := tracer.Start(ctx, next.Name())
            defer span.End()
            out, err := next.Run(ctx, in)
            if err != nil {
                span.RecordError(err)
            }
            return out, err
        })
    }
}
```

Keep `next.Name()` as the name of the replacement, or the tree and the logs
will name the middleware instead of the work.

### Keep the state outside the step

A step is a value with no state of its own. A middleware that counts or times
writes to a collector you own, which is also where you read it from.

## Testing

### Test a step as a function

```go
func TestTransform(t *testing.T) {
    out, err := transform.Run(t.Context(), Record{ID: "1", Type: "special", Value: 2})
    if err != nil {
        t.Fatal(err)
    }
    if out.ProcessedValue != 6 {
        t.Errorf("got %d, want 6", out.ProcessedValue)
    }
}
```

No mock framework, no pipeline, no context plumbing.

### Substitute a step, not an interface

To test a pipeline without the network, build it from steps you pass in:

```go
func build(fetch workflow.Step[string, Page]) workflow.Step[string, Report] {
    return fetch.Then(parse).Then(analyse)
}

pipeline := build(workflow.Pure("FakeFetch", func(string) Page { return testPage }))
```

### Pin the shape as well as the result

`String()` prints the tree. An assertion on it catches a composition that
changed by accident.

```go
want := "Then\n├── Fetch\n└── Parse"
if got := pipeline.String(); got != want {
    t.Errorf("pipeline shape changed:\n%s", got)
}
```

### Run the tests with `-race`

The concurrent combinators start goroutines. `go test -race ./...` is what
catches a branch that writes to a shared input.

## Observability

- `WithID()` puts a UUID in the context. Read it in any step with
  `workflow.StepID(ctx)` and put it in your own log lines to tie a run
  together.
- `Log(l)` gives you the name, the duration and the error of a step. It is one
  line per step; put it on the steps worth a line, not on every one.
- For metrics and traces, write a middleware. The examples directory has a
  timing one.

## Anti-patterns

### One struct for the whole pipeline

It is the shape the library moved away from. If every step takes and returns
`*State`, the compiler can no longer tell you anything, and you are back to
reading the code to know which fields are set.

### A middleware that hides an error

```go
// Bad: the pipeline is green and the work did not happen.
return fallback(in), nil
```

Fine as a deliberate degradation, with a metric next to it. Not fine as a
default.

### An unbounded `Each` over a remote call

`Each(0, callAPI)` on ten thousand records opens ten thousand goroutines and
ten thousand connections. Pass the number the dependency can take.

### A pipeline nobody can read

A tree ten levels deep is hard to follow, whatever it is made of. Name the
middle of it:

```go
quality := workflow.Fan("Quality", join, tests, lint, scan)
rollout := workflow.Seq("Rollout", staging, smoke, production)
pipeline := checkout.Then(quality).Then(build).Then(rollout)
```

### Retrying a step that is not idempotent

`Retry` calls the step again with the same input. A step that charges a card
needs an idempotency key before it needs a retry.
