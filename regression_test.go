package workflow_test

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	wf "github.com/veggiemonk/workflow"
)

type reg struct {
	Counter int
	Msgs    []string
}

func countingMiddleware(calls *atomic.Int64) wf.Middleware[reg] {
	return func(next wf.Step[reg]) wf.Step[reg] {
		return &wf.MidFunc[reg]{
			Name: "Count",
			Next: next,
			Fn: func(ctx context.Context, r *reg) (*reg, error) {
				calls.Add(1)
				return next.Run(ctx, r)
			},
		}
	}
}

func noop() wf.Step[reg] {
	return wf.StepFunc[reg](func(_ context.Context, r *reg) (*reg, error) { return r, nil })
}

// Defect 1. Pipeline.Run used to write the wrapped step back into p.Steps, so
// each run added another layer: the middleware fired 1, then 2, then 3 times.
func TestPipelineDoesNotReapplyMiddleware(t *testing.T) {
	var calls atomic.Int64
	p := wf.NewPipeline(countingMiddleware(&calls))
	p.Steps = []wf.Step[reg]{noop()}

	for run := 1; run <= 3; run++ {
		calls.Store(0)
		if _, err := p.Run(t.Context(), &reg{}); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if got := calls.Load(); got != 1 {
			t.Errorf("run %d fired the middleware %d times, want 1", run, got)
		}
	}
}

// Defect 1, same fault in Sequential.
func TestSequentialDoesNotReapplyMiddleware(t *testing.T) {
	var calls atomic.Int64
	s := wf.Sequential([]wf.Middleware[reg]{countingMiddleware(&calls)}, noop(), noop())

	for run := 1; run <= 3; run++ {
		calls.Store(0)
		if _, err := s.Run(t.Context(), &reg{}); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}
		if got := calls.Load(); got != 2 {
			t.Errorf("run %d fired the middleware %d times, want 2", run, got)
		}
	}
}

// Defect 1. Run must leave the pipeline exactly as it was declared.
func TestRunDoesNotMutateThePipeline(t *testing.T) {
	var calls atomic.Int64
	p := wf.NewPipeline(countingMiddleware(&calls))
	p.Steps = []wf.Step[reg]{noop()}

	before := p.String()
	if _, err := p.Run(t.Context(), &reg{}); err != nil {
		t.Fatal(err)
	}
	if after := p.String(); after != before {
		t.Errorf("Run changed the pipeline:\nbefore: %s\nafter:  %s", before, after)
	}
}

// Defect 1. Two runs at the same time used to race on the step slice.
// Run this with -race.
func TestConcurrentRunsAreSafe(t *testing.T) {
	var calls atomic.Int64
	p := wf.NewPipeline(countingMiddleware(&calls))
	p.Steps = []wf.Step[reg]{noop(), noop()}

	var wg sync.WaitGroup
	for range 50 {
		wg.Go(func() {
			if _, err := p.Run(t.Context(), &reg{}); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if got := calls.Load(); got != 100 {
		t.Errorf("middleware fired %d times, want 100", got)
	}
}

// Defect 3. A panic in a parallel branch was logged, the goroutine returned
// nil, and the merge then panicked on the nil pointer. It is now an error.
func TestParallelPanicBecomesAnError(t *testing.T) {
	boom := wf.StepFunc[reg](func(context.Context, *reg) (*reg, error) { panic("boom") })
	p := wf.Parallel(nil, wf.Merge[reg], noop(), boom)

	_, err := p.Run(t.Context(), &reg{})
	if err == nil {
		t.Fatal("want an error, not a crash")
	}
	var pe *wf.PanicError
	if !errors.As(err, &pe) {
		t.Fatalf("got %T (%v), want *wf.PanicError", err, err)
	}
	if pe.Value != "boom" || len(pe.Stack) == 0 {
		t.Errorf("got value=%v stack=%d bytes, want boom and a stack", pe.Value, len(pe.Stack))
	}
}

// Defect 3, second half. A step that returns (nil, nil) made mergo panic.
func TestMergeSkipsANilResponse(t *testing.T) {
	got, err := wf.Merge(t.Context(), &reg{Counter: 7}, nil, &reg{}, nil)
	if err != nil {
		t.Fatalf("Merge: %v", err)
	}
	if got.Counter != 7 {
		t.Errorf("Counter = %d, want 7", got.Counter)
	}
}

// Defect 2. This is NOT fixed, and it cannot be fixed while a step returns the
// same type it reads. mergo fills only an empty destination field; it never
// combines two values. The test pins the real behaviour so that nobody has to
// discover it in production. Pass your own MergeRequest, or use v2.
func TestMergeDoesNotCombineValues(t *testing.T) {
	inc := func() wf.Step[reg] {
		return wf.StepFunc[reg](func(_ context.Context, r *reg) (*reg, error) {
			r.Counter++
			return r, nil
		})
	}
	got, err := wf.Parallel(nil, wf.Merge[reg], inc(), inc()).Run(t.Context(), &reg{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Counter != 1 {
		t.Errorf("Counter = %d; the default Merge keeps the first branch only, so 1 is expected here", got.Counter)
	}

	// A MergeRequest of your own adds the branches correctly.
	sum := func(_ context.Context, req *reg, resps ...*reg) (*reg, error) {
		out := &reg{}
		for _, r := range resps {
			out.Counter += r.Counter - req.Counter
		}
		out.Counter += req.Counter
		return out, nil
	}
	got, err = wf.Parallel(nil, sum, inc(), inc()).Run(t.Context(), &reg{})
	if err != nil {
		t.Fatal(err)
	}
	if got.Counter != 2 {
		t.Errorf("Counter = %d, want 2 with a merge of your own", got.Counter)
	}
}
