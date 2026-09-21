package workflow_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	workflow "github.com/veggiemonk/workflow"
)

func TestIdentity(t *testing.T) {
	if got, err := workflow.Identity[string]().Run(t.Context(), "same"); err != nil || got != "same" {
		t.Errorf("got %q, %v; want same, nil", got, err)
	}
}

func TestOfNilRunner(t *testing.T) {
	var r workflow.Runner[int, int]
	if _, err := workflow.Of("Nil", r).Run(t.Context(), 1); !errors.Is(err, workflow.ErrNilStep) {
		t.Errorf("got %v, want ErrNilStep", err)
	}
}

func TestZeroStepNamesItsTypes(t *testing.T) {
	var zero workflow.Step[int, string]
	if got := zero.String(); got != "Step[int,string]" {
		t.Errorf("got %q, want Step[int,string]", got)
	}
}

func TestPanicErrorMessage(t *testing.T) {
	s := workflow.Func("Bad", func(context.Context, int) (int, error) { panic("kaboom") }).Recover()
	_, err := s.Run(t.Context(), 0)
	msg := err.Error()
	for _, want := range []string{"panic in Bad", "kaboom"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message %q lacks %q", msg, want)
		}
	}
}

// A zero RetryConfig must still be usable.
func TestRetryDefaults(t *testing.T) {
	attempts := 0
	s := workflow.Func("Always", func(context.Context, int) (int, error) {
		attempts++
		return 0, errors.New("nope")
	}).Retry(workflow.RetryConfig{})

	start := time.Now()
	if _, err := s.Run(t.Context(), 0); err == nil {
		t.Fatal("want an error")
	}
	if attempts != 3 {
		t.Errorf("attempts = %d, want the default of 3", attempts)
	}
	// 100ms then 200ms of backoff.
	if d := time.Since(start); d < 250*time.Millisecond {
		t.Errorf("took %s, want the default backoff", d)
	}
}

// A zero CircuitBreakerConfig must still be usable.
func TestCircuitBreakerDefaults(t *testing.T) {
	cb := workflow.NewCircuitBreaker(workflow.CircuitBreakerConfig{})
	s := workflow.Func("Bad", func(context.Context, int) (int, error) {
		return 0, errors.New("down")
	}).Breaker(cb)

	for range 5 {
		if _, err := s.Run(t.Context(), 0); errors.Is(err, workflow.ErrCircuitOpen) {
			t.Fatal("the circuit opened before the default threshold of 5")
		}
	}
	if _, err := s.Run(t.Context(), 0); !errors.Is(err, workflow.ErrCircuitOpen) {
		t.Errorf("got %v, want the circuit to be open", err)
	}
}

func TestCircuitBreakerResetsOnSuccess(t *testing.T) {
	var fail bool
	cb := workflow.NewCircuitBreaker(workflow.CircuitBreakerConfig{FailureThreshold: 3})
	s := workflow.Func("Flaky", func(context.Context, int) (int, error) {
		if fail {
			return 0, errors.New("down")
		}
		return 1, nil
	}).Breaker(cb)

	// Two failures, then a success, then two more failures: never 3 in a row.
	for _, want := range []bool{true, true, false, true, true} {
		fail = want
		_, _ = s.Run(t.Context(), 0)
	}
	fail = false
	if _, err := s.Run(t.Context(), 0); err != nil {
		t.Errorf("got %v, want the circuit to be closed", err)
	}
}

func TestCircuitBreakerShouldTrip(t *testing.T) {
	ignored := errors.New("ignored")
	cb := workflow.NewCircuitBreaker(workflow.CircuitBreakerConfig{
		FailureThreshold: 2,
		ShouldTrip:       func(err error) bool { return !errors.Is(err, ignored) },
	})
	s := workflow.Func("Bad", func(context.Context, int) (int, error) {
		return 0, ignored
	}).Breaker(cb)

	for range 5 {
		if _, err := s.Run(t.Context(), 0); errors.Is(err, workflow.ErrCircuitOpen) {
			t.Fatal("an ignored error opened the circuit")
		}
	}
}

func TestFanReportsEveryError(t *testing.T) {
	bad := func(msg string) workflow.Step[int, int] {
		return workflow.Func(msg, func(context.Context, int) (int, error) { return 0, errors.New(msg) })
	}
	s := workflow.Fan("All", func([]int) (int, error) { return 0, nil },
		bad("one"), workflow.Pure("Ok", func(n int) int { return n }), bad("two"))

	_, err := s.Run(t.Context(), 0)
	if err == nil {
		t.Fatal("want an error")
	}
	for _, want := range []string{"one", "two"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestFanTurnsPanicIntoError(t *testing.T) {
	s := workflow.Fan("All", func([]int) (int, error) { return 0, nil },
		workflow.Func("Bad", func(context.Context, int) (int, error) { panic("boom") }))
	var pe *workflow.PanicError
	if _, err := s.Run(t.Context(), 0); !errors.As(err, &pe) {
		t.Errorf("got %v, want *PanicError", err)
	}
}

func TestEachTurnsPanicIntoError(t *testing.T) {
	s := workflow.Each(2, workflow.Func("Bad", func(context.Context, int) (int, error) { panic("boom") }))
	var pe *workflow.PanicError
	if _, err := s.Run(t.Context(), []int{1, 2}); !errors.As(err, &pe) {
		t.Errorf("got %v, want *PanicError", err)
	}
}

func TestLogWithNilLogger(t *testing.T) {
	s := workflow.Pure("Id", func(n int) int { return n }).Log(nil)
	if got, err := s.Run(t.Context(), 3); err != nil || got != 3 {
		t.Errorf("got %d, %v; want 3, nil", got, err)
	}
}
