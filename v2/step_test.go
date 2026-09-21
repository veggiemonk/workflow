package workflow_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	workflow "github.com/veggiemonk/workflow/v2"
)

type Doc struct{ Body string }

type tokens []string

func parse() workflow.Step[Doc, tokens] {
	return workflow.Func("Parse", func(_ context.Context, d Doc) (tokens, error) {
		if d.Body == "" {
			return nil, errors.New("empty document")
		}
		return tokens(strings.Fields(d.Body)), nil
	})
}

func count() workflow.Step[tokens, int] {
	return workflow.Pure("Count", func(t tokens) int { return len(t) })
}

// counter is a Runner: a type that carries state.
type counter struct{ calls int }

func (c *counter) Run(_ context.Context, t tokens) (int, error) {
	c.calls++
	return len(t), nil
}

func TestThenChangesType(t *testing.T) {
	// Doc -> tokens -> int -> string, checked by the compiler.
	p := parse().
		Then(count()).
		Map("Label", func(n int) (string, error) { return strings.Repeat("*", n), nil })

	got, err := p.Run(t.Context(), Doc{Body: "a b c"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got != "***" {
		t.Errorf("got %q, want %q", got, "***")
	}
}

func TestThenStopsOnError(t *testing.T) {
	reached := false
	p := parse().Then(workflow.Pure("Mark", func(tokens) int { reached = true; return 0 }))
	if _, err := p.Run(t.Context(), Doc{}); err == nil {
		t.Fatal("want an error for an empty document")
	}
	if reached {
		t.Error("the second step ran after the first failed")
	}
}

func TestOfRunner(t *testing.T) {
	c := &counter{}
	p := parse().Then(workflow.Of("Counter", c))
	if _, err := p.Run(t.Context(), Doc{Body: "x y"}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if c.calls != 1 {
		t.Errorf("calls = %d, want 1", c.calls)
	}
}

func TestZeroStepReturnsError(t *testing.T) {
	var zero workflow.Step[int, int]
	if _, err := zero.Run(t.Context(), 1); !errors.Is(err, workflow.ErrNilStep) {
		t.Errorf("got %v, want ErrNilStep", err)
	}
}

func TestRunChecksContextFirst(t *testing.T) {
	ran := false
	s := workflow.Pure("Mark", func(int) int { ran = true; return 0 })
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := s.Run(ctx, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("got %v, want context.Canceled", err)
	}
	if ran {
		t.Error("the step ran with a cancelled context")
	}
}

// A step is a value. Rename must not change the receiver.
func TestRenameDoesNotMutate(t *testing.T) {
	a := workflow.Pure("A", func(i int) int { return i })
	b := a.Rename("B")
	if a.Name() != "A" || b.Name() != "B" {
		t.Errorf("a=%q b=%q, want A and B", a.Name(), b.Name())
	}
}

// One step, run from many goroutines at once. Run with -race.
func TestStepIsSafeForConcurrentUse(t *testing.T) {
	p := parse().Then(count()).Retry(workflow.RetryConfig{MaxAttempts: 2})
	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if n, err := p.Run(t.Context(), Doc{Body: "a b c"}); err != nil || n != 3 {
				t.Errorf("got %d, %v; want 3, nil", n, err)
			}
		})
	}
	wg.Wait()
}

func TestStringTree(t *testing.T) {
	p := parse().Then(count()).Rename("Analyse")
	want := "Analyse\n├── Parse\n└── Count"
	if got := p.String(); got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}
