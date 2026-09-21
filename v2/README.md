# workflow/v2

A small library that composes typed units of work into a pipeline.

```bash
go get github.com/veggiemonk/workflow/v2
```

v2 needs Go 1.27, because it uses generic methods.

## Why v2 exists

In v1 a step was `Step[T]`: it read a `T` and returned a `T`. One type had to
carry the whole pipeline. That single decision caused everything else:

- Parallel branches all returned the same struct, so v1 needed a merge
  function. It used reflection, and it dropped a result without a word.
- Branches shared the struct, so v1 needed a deep copy. The copy fell back to
  a shallow copy in silence.
- v1 applied middleware while it ran, by writing to its own step slice. A
  second run added a second layer.

In v2 a step declares its own input type and its own output type. The compiler
then does the work that reflection did, and the three faults above cannot
happen.

## The design in one page

```go
type Step[I, O any] struct{ /* unexported */ }   // a value, not an interface

func (s Step[I, O]) Run(ctx context.Context, in I) (O, error)
```

`Step` is a **concrete struct**, not an interface. That is deliberate. Go 1.27
allows type parameters on a method only when the receiver is a concrete type:

```
interface method must have no type parameters
```

A generic method is what lets a chain change its type:

```go
func (s Step[I, O]) Then[P any](next Step[O, P]) Step[I, P]
```

To write a step on a type that holds state, implement `Runner` — a plain
interface, so it stays legal — and wrap it with `Of`:

```go
type Runner[I, O any] interface{ Run(context.Context, I) (O, error) }

step := workflow.Of("Cache", myCache)   // Step[Key, Value]
```

## Build a step

| Function | Use it for |
| --- | --- |
| `Func(name, f)` | a function that can fail |
| `Pure(name, f)` | a function that cannot fail |
| `Of(name, runner)` | a type that carries state |
| `Identity[T]()` | a step that returns its input |

## Compose steps

| Call | Shape |
| --- | --- |
| `s.Then(next)` | `Step[I,O]` + `Step[O,P]` → `Step[I,P]` |
| `s.Map(name, f)` | `Step[I,O]` + `func(O) (P, error)` → `Step[I,P]` |
| `s.Par(other, join)` | two steps on one input, one typed join → `Step[I,R]` |
| `Fan(name, join, steps...)` | any number of steps that share an output type |
| `Each(n, s)` | `Step[I,O]` → `Step[[]I,[]O]`, at most `n` at a time |
| `Seq(name, steps...)` | a chain that does not change type |
| `If(name, pred, then, else)` | a branch |

`Each` is a function, not a method. A method returning `Step[[]I,[]O]` would
make the compiler instantiate `Step[[][]I,[][]O]`, and so on without end. The
compiler calls that an instantiation cycle and rejects it.

```go
analyse := count.
    Par(uniq,  func(n, u int) ([2]int, error)            { return [2]int{n, u}, nil }).
    Par(upper, func(nu [2]int, s string) (Report, error) { return Report{nu[0], nu[1], s}, nil })

pipeline := parse.Then(analyse)      // Step[Doc, Report]
batch    := workflow.Each(8, pipeline) // Step[[]Doc, []Report]
```

## Middleware

A step is a value. Every method returns a new step and leaves the receiver
unchanged. Middleware is applied once, when you build the step, so a second
run can never add a second layer.

```go
step := fetch.
    Retry(workflow.RetryConfig{MaxAttempts: 3}).
    Timeout(2 * time.Second).
    Breaker(breaker).
    Log(logger)
```

| Method | What it does |
| --- | --- |
| `Use(mw...)` | applies your own `Middleware[I,O]`; the first is outermost |
| `Retry(cfg)` | runs again on failure, with exponential backoff |
| `Timeout(d)` | gives the step a context with a deadline |
| `Recover()` | turns a panic into a `*PanicError` |
| `Log(l)` | records the name, the duration and the error |
| `WithID()` | puts a UUID in the context; read it with `StepID` |
| `Breaker(cb)` | blocks the step while the circuit is open |

Three notes on behaviour:

- **`Log` never records the payload.** A payload can hold a secret, and
  formatting it costs more than a short step.
- **`Timeout` starts no goroutine.** Go cannot stop a goroutine from outside,
  so the step must honour the context. A step that does no I/O must test
  `ctx.Err()` itself. v1 raced a goroutine and leaked it.
- **A `CircuitBreaker` is an explicit value.** You decide what shares it. In v1
  the state sat in a closure, so every step the middleware wrapped shared one
  circuit without saying so.

## Errors and panics

`Par`, `Fan` and `Each` run every branch to the end and return every error,
joined with `errors.Join`. They do not stop at the first failure.

Each of them also turns a panic in a branch into a `*PanicError`, because a
panic in a goroutine would otherwise stop the program. A step that runs in
sequence is left alone: its panic travels up your own stack, as Go intends.
Call `Recover()` when you want that converted too.

## Cost

On an Apple M4 Max, Go 1.27:

```
BenchmarkRun-16                296803701     4.158 ns/op     0 B/op   0 allocs/op
BenchmarkThenChain-16           32746808    36.06  ns/op     0 B/op   0 allocs/op
BenchmarkSeq-16                 45185329    28.23  ns/op     0 B/op   0 allocs/op
BenchmarkPar-16                  1583120   758.9   ns/op   288 B/op   6 allocs/op
BenchmarkEach100-16                30480 39491     ns/op 20422 B/op 104 allocs/op
BenchmarkMiddlewareStack-16      4575774   262.6   ns/op   272 B/op   4 allocs/op
```

The sequential path allocates nothing. Only the concurrent combinators
allocate, for the goroutines and the result slices.

## Move from v1

| v1 | v2 |
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
| `Name(step)` (reflection) | `step.Name()` |
| `StepValidator`, `SafeRun` | the types check this; a zero step gives `ErrNilStep` |

A v1 pipeline that keeps one type throughout maps to `Seq[T]` with no other
change in shape, because `Step[T]` is `Step[T,T]`.

v2 has **no dependency outside the standard library**. v1 needed `mergo`,
`google/uuid` and `x/sync`.

## What v2 is not

v2 is an in-memory combinator library. It holds no state between runs. It
cannot resume after a crash, and it has no general graph with shared
dependencies: a pipeline is a tree of sequence and fan-out. Use Temporal or
Cadence when you need a durable workflow.
