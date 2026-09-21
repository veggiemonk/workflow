# Architecture

The library composes typed units of work. It is an in-memory combinator
library: it holds no state between runs, and it cannot resume after a crash.

## The one type

```go
type Step[I, O any] struct {
    name string
    run  func(context.Context, I) (O, error)
    kids []fmt.Stringer
}

func (s Step[I, O]) Run(ctx context.Context, in I) (O, error)
```

Everything else in the package builds a `Step` or wraps one. There is no
pipeline type, no runner, no registry. A pipeline of twenty steps is a `Step`,
and so is each of the twenty.

Three consequences follow from the fields.

**`run` is a function, so a step is a value.** Every method returns a new
`Step` and writes nothing to its receiver. A step can be shared between
goroutines, stored in a package variable, and run any number of times.

**`I` and `O` are separate, so the chain can change type.** A step cannot read
a field that the step before it did not produce, because the field is not
there. Reflection is not needed to merge results, and no result can be
dropped in silence.

**`kids` holds the steps a combinator was built from, so a pipeline prints as
the tree you declared.** The tree is built when the step is built, not while
it runs, so printing a step is free of side effects.

### Step is a struct, not an interface

Go 1.27 allows type parameters on a method only when the receiver is a
concrete type. An interface method must have no type parameters:

```
interface method must have no type parameters
```

A generic method is what lets a chain change type:

```go
func (s Step[I, O]) Then[P any](next Step[O, P]) Step[I, P]
```

So `Step` is a concrete struct. The extension point for a type that carries
state is `Runner`, a plain interface whose method has no type parameters:

```go
type Runner[I, O any] interface {
    Run(context.Context, I) (O, error)
}

step := workflow.Of("Cache", myCache) // Step[Key, Value]
```

## Building a step

| Function | Source |
| --- | --- |
| `Func(name, f)` | a function that can fail |
| `Pure(name, f)` | a function that cannot fail |
| `Of(name, runner)` | a type that carries state |
| `Identity[T]()` | a step that returns its input |

The zero `Step` holds no function. Running it returns `ErrNilStep` rather than
a nil dereference.

## Composing steps

| Call | Shape |
| --- | --- |
| `s.Then(next)` | `Step[I,O]` + `Step[O,P]` → `Step[I,P]` |
| `s.Map(name, f)` | `Step[I,O]` + `func(O) (P, error)` → `Step[I,P]` |
| `s.Par(other, join)` | two steps on one input, one typed join → `Step[I,R]` |
| `Fan(name, join, steps...)` | any number of steps that share an output type |
| `Each(n, s)` | `Step[I,O]` → `Step[[]I,[]O]`, at most `n` at a time |
| `Seq(name, steps...)` | a chain that does not change type |
| `If(name, pred, then, else)` | a branch |

`Then` and `Map` are methods because their type parameter belongs to the call.
`Each` is a function: a method returning `Step[[]I, []O]` would make the
compiler instantiate `Step[[][]I, [][]O]`, and so on without end. The compiler
calls that an instantiation cycle and rejects it.

`Seq` and `If` are functions for the same reason `Each` is not a method: they
take a list, or two branches, of one shape.

## Execution model

**Sequential.** `Then`, `Map` and `Seq` call one step after another on the
calling goroutine. They allocate nothing. The first error stops the chain.

**Concurrent.** `Par`, `Fan` and `Each` start goroutines, and every branch runs
to the end. They do not stop at the first failure. Every error comes back
joined with `errors.Join`, and a panic in a branch comes back as a
`*PanicError` rather than stopping the program. `Each` bounds its concurrency
with the count you pass.

**Conditional.** `If` runs the predicate on the value that reaches it, then
runs one branch. Both branches have the same input and output type, so the
result of the gate has one type whichever way it went.

**Context.** `Run` checks `ctx.Err()` before it starts the work. From there
the context is the step's own business: a step that does I/O gets cancellation
from the standard library, and a step that does not must test `ctx.Err()`
itself.

## Middleware

```go
type Middleware[I, O any] func(Step[I, O]) Step[I, O]
```

A middleware is a function from a step to a step. Writing one needs nothing
from the package. `Use` applies them, outermost first:

```go
step := fetch.Use(myTracing, myMetrics)
```

The built-in middleware are methods, because they are the ones people reach
for: `Retry`, `Timeout`, `Recover`, `Log`, `WithID`, `Breaker`.

Middleware is applied when you build the step, never while it runs. This is
what makes a second run identical to the first. In v0.3.0 the pipeline
rewrote its own step slice at run time, so a `Retry(3)` became nine attempts
on the third run.

Three points of behaviour:

- `Log` records the name, the duration and the error, never the payload. A
  payload can hold a secret, and formatting it costs more than a short step.
- `Timeout` gives the step a context with a deadline and starts no goroutine.
  Go cannot stop a goroutine from outside, so the step must honour the
  context.
- A `CircuitBreaker` is a value you create and pass. You decide what shares
  it. The state is not hidden in a closure.

## Extension points

**A step with state.** Implement `Runner` and wrap it with `Of`. The state is
yours; the package does not touch it.

**A middleware.** Write a `Middleware[I, O]`. Build the replacement step with
`Func`, call `next.Run` inside it, and use `next.Name()` in the name so the
printed tree stays readable.

**A combinator.** Build a `Step` with `Func` and call other steps inside it.
Anything the package does, your code can do; there is no unexported behaviour
that a combinator needs.

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

The sequential path allocates nothing. The concurrent combinators allocate for
the goroutines and the result slices.

## What is not here

- No durable state, no resume after a crash. Use Temporal or Cadence.
- No general graph. A pipeline is a tree of sequence and fan-out; a step
  cannot depend on two steps that are not its own branches.
- No scheduler, no queue, no retry store. `Retry` is in-process and in-memory.
