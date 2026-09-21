package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"
	"uuid"
)

// Middleware wraps a step to add behaviour such as logging or retry.
//
// Middleware is applied once, when you build the step. [Step.Use] returns a
// new step and leaves the receiver unchanged, so a step can never collect a
// second layer of middleware when you run it again.
type Middleware[I, O any] func(Step[I, O]) Step[I, O]

// Use applies middleware to the step and returns the wrapped step.
// The first middleware given is the outermost.
func (s Step[I, O]) Use(mw ...Middleware[I, O]) Step[I, O] {
	for _, m := range slices.Backward(mw) {
		if m == nil {
			continue
		}
		s = m(s)
	}
	return s
}

// Recover turns a panic raised by the step into a [PanicError].
//
// [Par], [Fan] and [Each] already do this for every branch, because a panic in
// a goroutine stops the program. A step that runs in sequence does not need
// Recover: its panic travels up the caller's own stack, as Go intends.
func (s Step[I, O]) Recover() Step[I, O] {
	inner := s
	return s.wrap("Recover", func(ctx context.Context, in I) (out O, err error) {
		defer func() {
			if r := recover(); r != nil {
				var zero O
				out, err = zero, recovered(r, inner.displayName())
			}
		}()
		return inner.Run(ctx, in)
	})
}

// RetryConfig configures [Step.Retry].
type RetryConfig struct {
	// MaxAttempts counts the first try as well. Below 1 means 3.
	MaxAttempts int
	// InitialDelay is the wait before the first retry. Below 1 means 100ms.
	InitialDelay time.Duration
	// MaxDelay caps the wait. Below 1 means 5s.
	MaxDelay time.Duration
	// BackoffMultiplier multiplies the wait after each try. Below 1 means 2.
	BackoffMultiplier float64
	// ShouldRetry decides whether an error is worth another try.
	// A nil ShouldRetry retries every error.
	ShouldRetry func(error) bool
}

func (c RetryConfig) withDefaults() RetryConfig {
	if c.MaxAttempts < 1 {
		c.MaxAttempts = 3
	}
	if c.InitialDelay < 1 {
		c.InitialDelay = 100 * time.Millisecond
	}
	if c.MaxDelay < 1 {
		c.MaxDelay = 5 * time.Second
	}
	if c.BackoffMultiplier < 1 {
		c.BackoffMultiplier = 2
	}
	return c
}

// Retry runs the step again when it fails, with exponential backoff.
// It stops early when the context ends.
func (s Step[I, O]) Retry(cfg RetryConfig) Step[I, O] {
	cfg = cfg.withDefaults()
	inner := s
	return s.wrap(fmt.Sprintf("Retry(%d)", cfg.MaxAttempts), func(ctx context.Context, in I) (O, error) {
		var last error
		delay := cfg.InitialDelay
		for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
			if err := ctx.Err(); err != nil {
				var zero O
				return zero, err
			}
			out, err := inner.Run(ctx, in)
			if err == nil {
				return out, nil
			}
			last = err
			if cfg.ShouldRetry != nil && !cfg.ShouldRetry(err) {
				var zero O
				return zero, err
			}
			if attempt == cfg.MaxAttempts {
				break
			}
			timer := time.NewTimer(delay)
			select {
			case <-ctx.Done():
				timer.Stop()
				var zero O
				return zero, ctx.Err()
			case <-timer.C:
			}
			delay = min(time.Duration(float64(delay)*cfg.BackoffMultiplier), cfg.MaxDelay)
		}
		var zero O
		return zero, fmt.Errorf("%s failed after %d attempts: %w", inner.displayName(), cfg.MaxAttempts, last)
	})
}

// Timeout gives the step a context with a deadline.
//
// The step must honour the context. Timeout does not start a goroutine and it
// does not abandon a running step, because Go cannot stop a goroutine from
// outside. A step that does no I/O must test ctx.Err() itself.
func (s Step[I, O]) Timeout(d time.Duration) Step[I, O] {
	inner := s
	return s.wrap(fmt.Sprintf("Timeout(%s)", d), func(ctx context.Context, in I) (O, error) {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		out, err := inner.Run(ctx, in)
		if err != nil && ctx.Err() != nil {
			var zero O
			return zero, fmt.Errorf("%s timed out after %s: %w", inner.displayName(), d, err)
		}
		return out, err
	})
}

// Log records the name, the duration and the error of each run.
//
// Log never records the input or the output. A payload can hold a secret, and
// formatting it costs more than the step itself in a hot pipeline.
func (s Step[I, O]) Log(l *slog.Logger) Step[I, O] {
	if l == nil {
		l = slog.Default()
	}
	inner := s
	return s.wrap("Log", func(ctx context.Context, in I) (O, error) {
		name := inner.displayName()
		start := time.Now()
		l.DebugContext(ctx, "step start", "step", name)
		out, err := inner.Run(ctx, in)
		attrs := []any{"step", name, "duration", time.Since(start)}
		if err != nil {
			l.ErrorContext(ctx, "step failed", append(attrs, "error", err)...)
		} else {
			l.InfoContext(ctx, "step done", attrs...)
		}
		return out, err
	})
}

type idKey struct{}

// StepID returns the identifier that [Step.WithID] put in the context.
func StepID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(idKey{}).(string)
	return id, ok
}

// WithID puts a fresh UUID in the context for the length of the run.
// Read it with [StepID].
func (s Step[I, O]) WithID() Step[I, O] {
	inner := s
	return s.wrap("WithID", func(ctx context.Context, in I) (O, error) {
		return inner.Run(context.WithValue(ctx, idKey{}, uuid.NewV7().String()), in)
	})
}

// ErrCircuitOpen is returned while a [CircuitBreaker] is open.
var ErrCircuitOpen = errors.New("workflow: circuit breaker is open")

// CircuitBreakerConfig configures a [CircuitBreaker].
type CircuitBreakerConfig struct {
	// FailureThreshold is the count of failures in a row that opens the
	// circuit. Below 1 means 5.
	FailureThreshold int
	// OpenTimeout is how long the circuit stays open. Below 1 means 60s.
	OpenTimeout time.Duration
	// ShouldTrip decides whether an error counts as a failure.
	// A nil ShouldTrip counts every error.
	ShouldTrip func(error) bool
}

// CircuitBreaker stops calling a step that keeps failing.
//
// The breaker is an explicit value, so you decide what shares it. Give one
// breaker to one step to guard that step alone. Give the same breaker to
// several steps only when they all depend on the same remote service.
type CircuitBreaker struct {
	cfg CircuitBreakerConfig

	mu       sync.Mutex
	failures int
	openedAt time.Time
	halfOpen bool
}

// NewCircuitBreaker builds a breaker.
func NewCircuitBreaker(cfg CircuitBreakerConfig) *CircuitBreaker {
	if cfg.FailureThreshold < 1 {
		cfg.FailureThreshold = 5
	}
	if cfg.OpenTimeout < 1 {
		cfg.OpenTimeout = 60 * time.Second
	}
	return &CircuitBreaker{cfg: cfg}
}

// allow reports whether a call may proceed now.
func (cb *CircuitBreaker) allow() bool {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	if cb.failures < cb.cfg.FailureThreshold {
		return true
	}
	if time.Since(cb.openedAt) < cb.cfg.OpenTimeout {
		return false
	}
	cb.halfOpen = true
	return true
}

// record updates the breaker with the result of a call.
func (cb *CircuitBreaker) record(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	failed := err != nil && (cb.cfg.ShouldTrip == nil || cb.cfg.ShouldTrip(err))
	switch {
	case failed && cb.halfOpen:
		cb.openedAt = time.Now()
		cb.halfOpen = false
	case failed:
		cb.failures++
		if cb.failures >= cb.cfg.FailureThreshold {
			cb.openedAt = time.Now()
		}
	default:
		cb.failures = 0
		cb.halfOpen = false
	}
}

// Breaker blocks the step while cb is open.
func (s Step[I, O]) Breaker(cb *CircuitBreaker) Step[I, O] {
	inner := s
	return s.wrap("Breaker", func(ctx context.Context, in I) (O, error) {
		if !cb.allow() {
			var zero O
			return zero, fmt.Errorf("%s: %w", inner.displayName(), ErrCircuitOpen)
		}
		out, err := inner.Run(ctx, in)
		cb.record(err)
		return out, err
	})
}
