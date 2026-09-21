# Middleware

Four scenarios that show what the middleware methods do to one order
pipeline.

## The scenarios

1. **A valid order.** `Retry`, `Timeout` and `Log` sit on the payment step
   only. Validation is not retried.
2. **A flaky gateway.** The gateway fails twice, `Retry` waits and calls it
   again. `ShouldRetry` keeps a rejected order from being retried.
3. **A circuit breaker.** After three failures the breaker opens and the
   gateway is not called again. The run reports how many calls it stopped.
4. **A timeout.** A 50ms deadline on a step that needs 200ms.

## Running the example

```bash
cd examples/middleware
go run main.go
```

## What to look at

- A step is a value. Every middleware method returns a new step and leaves
  the receiver unchanged, so a second run cannot add a second layer.
- `Of` wraps a type that carries state. The gateway counts its calls.
- A `CircuitBreaker` is an explicit value that you create and share. In v1 the
  state sat in a closure, so every step the middleware wrapped shared one
  circuit without saying so.
- `Timeout` starts no goroutine. The step must honour the context; both slow
  steps in the example do.
- `Log` records the name, the duration and the error, never the payload.
