package workflow_test

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	workflow "github.com/veggiemonk/workflow/v2"
)

// v1 rewrote its own step slice during Run, so every run added another layer:
// the middleware fired 1, then 2, then 3 times. Use applies it once.
func TestMiddlewareIsAppliedOnce(t *testing.T) {
	var calls atomic.Int64
	countMW := func(next workflow.Step[int, int]) workflow.Step[int, int] {
		return workflow.Func("Counted", func(ctx context.Context, n int) (int, error) {
			calls.Add(1)
			return next.Run(ctx, n)
		})
	}
	s := workflow.Pure("Id", func(n int) int { return n }).Use(countMW)

	for run := 1; run <= 3; run++ {
		calls.Store(0)
		if _, err := s.Run(t.Context(), 1); err != nil {
			t.Fatalf("Run: %v", err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("run %d fired the middleware %d times, want 1", run, got)
		}
	}
}

func TestUseAppliesFirstAsOutermost(t *testing.T) {
	var order []string
	mark := func(name string) workflow.Middleware[int, int] {
		return func(next workflow.Step[int, int]) workflow.Step[int, int] {
			return workflow.Func(name, func(ctx context.Context, n int) (int, error) {
				order = append(order, name)
				return next.Run(ctx, n)
			})
		}
	}
	s := workflow.Pure("Id", func(n int) int { return n }).Use(mark("outer"), mark("inner"))
	if _, err := s.Run(t.Context(), 1); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if strings.Join(order, ",") != "outer,inner" {
		t.Errorf("order = %v, want [outer inner]", order)
	}
}

func TestUseSkipsNilMiddleware(t *testing.T) {
	s := workflow.Pure("Id", func(n int) int { return n }).Use(nil, nil)
	if got, err := s.Run(t.Context(), 7); err != nil || got != 7 {
		t.Errorf("got %d, %v; want 7, nil", got, err)
	}
}

func TestRecover(t *testing.T) {
	s := workflow.Func("Bad", func(context.Context, int) (int, error) { panic("boom") }).Recover()
	_, err := s.Run(t.Context(), 0)
	var pe *workflow.PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("got %v, want *PanicError", err)
	}
	if pe.Step != "Bad" {
		t.Errorf("step = %q, want Bad", pe.Step)
	}
}

func TestRetrySucceedsAfterFailure(t *testing.T) {
	var attempts atomic.Int64
	s := workflow.Func("Flaky", func(context.Context, int) (int, error) {
		if attempts.Add(1) < 3 {
			return 0, errors.New("not yet")
		}
		return 42, nil
	}).Retry(workflow.RetryConfig{MaxAttempts: 5, InitialDelay: time.Millisecond})

	got, err := s.Run(t.Context(), 0)
	if err != nil || got != 42 {
		t.Fatalf("got %d, %v; want 42, nil", got, err)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestRetryGivesUp(t *testing.T) {
	var attempts atomic.Int64
	s := workflow.Func("Always", func(context.Context, int) (int, error) {
		attempts.Add(1)
		return 0, errors.New("nope")
	}).Retry(workflow.RetryConfig{MaxAttempts: 3, InitialDelay: time.Millisecond})

	if _, err := s.Run(t.Context(), 0); err == nil || !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("got %v, want a give-up error", err)
	}
	if attempts.Load() != 3 {
		t.Errorf("attempts = %d, want 3", attempts.Load())
	}
}

func TestRetryHonoursShouldRetry(t *testing.T) {
	fatal := errors.New("fatal")
	var attempts atomic.Int64
	s := workflow.Func("Fatal", func(context.Context, int) (int, error) {
		attempts.Add(1)
		return 0, fatal
	}).Retry(workflow.RetryConfig{
		MaxAttempts:  5,
		InitialDelay: time.Millisecond,
		ShouldRetry:  func(err error) bool { return !errors.Is(err, fatal) },
	})

	if _, err := s.Run(t.Context(), 0); !errors.Is(err, fatal) {
		t.Errorf("got %v, want the fatal error unwrapped", err)
	}
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1", attempts.Load())
	}
}

func TestRetryStopsWhenContextEnds(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	var attempts atomic.Int64
	s := workflow.Func("Slow", func(context.Context, int) (int, error) {
		if attempts.Add(1) == 1 {
			cancel()
		}
		return 0, errors.New("nope")
	}).Retry(workflow.RetryConfig{MaxAttempts: 10, InitialDelay: 50 * time.Millisecond})

	if _, err := s.Run(ctx, 0); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1", attempts.Load())
	}
}

func TestTimeout(t *testing.T) {
	s := workflow.Func("Slow", func(ctx context.Context, n int) (int, error) {
		select {
		case <-ctx.Done():
			return 0, ctx.Err()
		case <-time.After(time.Second):
			return n, nil
		}
	}).Timeout(20 * time.Millisecond)

	start := time.Now()
	_, err := s.Run(t.Context(), 1)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("got %v, want DeadlineExceeded", err)
	}
	if elapsed := time.Since(start); elapsed > 500*time.Millisecond {
		t.Errorf("took %s, want it to stop early", elapsed)
	}
}

func TestTimeoutLeavesAFastStepAlone(t *testing.T) {
	s := workflow.Pure("Fast", func(n int) int { return n }).Timeout(time.Second)
	if got, err := s.Run(t.Context(), 5); err != nil || got != 5 {
		t.Errorf("got %d, %v; want 5, nil", got, err)
	}
}

func TestBreakerOpensThenRecovers(t *testing.T) {
	var fail atomic.Bool
	fail.Store(true)
	var calls atomic.Int64

	cb := workflow.NewCircuitBreaker(workflow.CircuitBreakerConfig{
		FailureThreshold: 2,
		OpenTimeout:      50 * time.Millisecond,
	})
	s := workflow.Func("Remote", func(context.Context, int) (int, error) {
		calls.Add(1)
		if fail.Load() {
			return 0, errors.New("down")
		}
		return 1, nil
	}).Breaker(cb)

	// Two failures open the circuit.
	for range 2 {
		if _, err := s.Run(t.Context(), 0); err == nil {
			t.Fatal("want a failure")
		}
	}
	if _, err := s.Run(t.Context(), 0); !errors.Is(err, workflow.ErrCircuitOpen) {
		t.Fatalf("got %v, want ErrCircuitOpen", err)
	}
	if calls.Load() != 2 {
		t.Errorf("the open circuit let %d calls through, want 2", calls.Load())
	}

	// After the timeout the circuit tries once more, and a success closes it.
	time.Sleep(60 * time.Millisecond)
	fail.Store(false)
	if _, err := s.Run(t.Context(), 0); err != nil {
		t.Fatalf("half-open call: %v", err)
	}
	if _, err := s.Run(t.Context(), 0); err != nil {
		t.Fatalf("circuit did not close: %v", err)
	}
}

// v1 kept the breaker state in the middleware closure, so every step it
// wrapped shared one circuit without saying so. The breaker is now a value.
func TestBreakersAreIndependentUnlessShared(t *testing.T) {
	cfg := workflow.CircuitBreakerConfig{FailureThreshold: 1, OpenTimeout: time.Minute}
	bad := workflow.Func("Bad", func(context.Context, int) (int, error) { return 0, errors.New("down") })
	good := workflow.Pure("Good", func(n int) int { return n })

	a := bad.Breaker(workflow.NewCircuitBreaker(cfg))
	b := good.Breaker(workflow.NewCircuitBreaker(cfg))

	_, _ = a.Run(t.Context(), 0) // opens a's circuit only
	if _, err := b.Run(t.Context(), 1); err != nil {
		t.Errorf("the second step was blocked by the first: %v", err)
	}

	shared := workflow.NewCircuitBreaker(cfg)
	c := bad.Breaker(shared)
	d := good.Breaker(shared)
	_, _ = c.Run(t.Context(), 0)
	if _, err := d.Run(t.Context(), 1); !errors.Is(err, workflow.ErrCircuitOpen) {
		t.Errorf("got %v, want a shared breaker to block", err)
	}
}

func TestWithID(t *testing.T) {
	var seen []string
	s := workflow.Func("Read", func(ctx context.Context, n int) (int, error) {
		id, ok := workflow.StepID(ctx)
		if !ok {
			t.Error("no id in the context")
		}
		seen = append(seen, id)
		return n, nil
	}).WithID()

	for range 2 {
		if _, err := s.Run(t.Context(), 0); err != nil {
			t.Fatalf("Run: %v", err)
		}
	}
	if len(seen) != 2 || seen[0] == "" || seen[0] == seen[1] {
		t.Errorf("ids = %v, want two different non-empty ids", seen)
	}
}

func TestStepIDAbsent(t *testing.T) {
	if id, ok := workflow.StepID(t.Context()); ok || id != "" {
		t.Errorf("got %q, %v; want an empty id", id, ok)
	}
}

// Log must never write the payload: it can hold a secret.
func TestLogOmitsThePayload(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	s := workflow.Pure("Secret", func(s string) string { return s + "-out" }).Log(l)

	if _, err := s.Run(t.Context(), "hunter2"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	out := buf.String()
	for _, secret := range []string{"hunter2", "-out"} {
		if strings.Contains(out, secret) {
			t.Errorf("the log holds the payload %q: %s", secret, out)
		}
	}
	for _, want := range []string{"Secret", "duration"} {
		if !strings.Contains(out, want) {
			t.Errorf("the log lacks %q: %s", want, out)
		}
	}
}

func TestLogRecordsFailure(t *testing.T) {
	var buf bytes.Buffer
	l := slog.New(slog.NewJSONHandler(&buf, nil))
	s := workflow.Func("Bad", func(context.Context, int) (int, error) {
		return 0, errors.New("down")
	}).Log(l)

	if _, err := s.Run(t.Context(), 0); err == nil {
		t.Fatal("want an error")
	}
	if !strings.Contains(buf.String(), "step failed") {
		t.Errorf("the log lacks the failure: %s", buf.String())
	}
}
