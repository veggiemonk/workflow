package workflow

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// Then returns a step that runs s, then feeds its output to next.
//
// P is a type parameter of the method, so the chain may change type at every
// link. Go infers P from next.
//
//	parse.Then(count).Then(format) // Step[Doc, string]
func (s Step[I, O]) Then[P any](next Step[O, P]) Step[I, P] {
	return Step[I, P]{
		name: "Then",
		kids: []fmt.Stringer{s, next},
		run: func(ctx context.Context, in I) (P, error) {
			mid, err := s.Run(ctx, in)
			if err != nil {
				var zero P
				return zero, err
			}
			return next.Run(ctx, mid)
		},
	}
}

// Map returns a step that runs s, then applies f to its output.
// It is [Step.Then] for a plain function.
func (s Step[I, O]) Map[P any](name string, f func(O) (P, error)) Step[I, P] {
	return s.Then(Func(name, func(_ context.Context, o O) (P, error) { return f(o) }))
}

// Par runs s and other on the same input at the same time, then joins their
// results with join.
//
// The join is typed, so the compiler will not let a result be dropped. Neither
// branch receives a copy of the input: give Par steps that do not write to it.
//
// Both branches always run to completion. If both fail, Par returns both
// errors joined with [errors.Join].
func (s Step[I, O]) Par[B, R any](other Step[I, B], join func(O, B) (R, error)) Step[I, R] {
	return Step[I, R]{
		name: "Par",
		kids: []fmt.Stringer{s, other},
		run: func(ctx context.Context, in I) (R, error) {
			var (
				left  O
				right B
				lerr  error
				rerr  error
				wg    sync.WaitGroup
			)
			wg.Go(func() {
				defer func() { lerr = errors.Join(lerr, recovered(recover(), s.displayName())) }()
				left, lerr = s.Run(ctx, in)
			})
			wg.Go(func() {
				defer func() { rerr = errors.Join(rerr, recovered(recover(), other.displayName())) }()
				right, rerr = other.Run(ctx, in)
			})
			wg.Wait()
			if err := errors.Join(lerr, rerr); err != nil {
				var zero R
				return zero, err
			}
			return join(left, right)
		},
	}
}

// Fan runs every step on the same input at the same time, then joins the
// results, in the order the steps were given, with join.
//
// Every step must share the same output type. Use [Step.Par] when the outputs
// differ. Every step runs to completion; Fan joins every error.
func Fan[I, O, R any](name string, join func([]O) (R, error), steps ...Step[I, O]) Step[I, R] {
	kids := make([]fmt.Stringer, len(steps))
	for i, s := range steps {
		kids[i] = s
	}
	return Step[I, R]{
		name: name,
		kids: kids,
		run: func(ctx context.Context, in I) (R, error) {
			out := make([]O, len(steps))
			errs := make([]error, len(steps))
			var wg sync.WaitGroup
			for i, s := range steps {
				wg.Go(func() {
					defer func() { errs[i] = errors.Join(errs[i], recovered(recover(), s.displayName())) }()
					out[i], errs[i] = s.Run(ctx, in)
				})
			}
			wg.Wait()
			if err := errors.Join(errs...); err != nil {
				var zero R
				return zero, err
			}
			return join(out)
		},
	}
}

// Each applies s to every element of a slice, at most concurrency at a time.
// Give a concurrency of 0 or less to run every element at the same time.
//
// Each cannot be a method. A method that returns Step[[]I, []O] would make the
// compiler instantiate Step[[][]I, [][]O], and so on without end. The compiler
// rejects that as an instantiation cycle.
func Each[I, O any](concurrency int, s Step[I, O]) Step[[]I, []O] {
	name := "Each"
	if concurrency > 0 {
		name = fmt.Sprintf("Each(max=%d)", concurrency)
	}
	return Step[[]I, []O]{
		name: name,
		kids: []fmt.Stringer{s},
		run: func(ctx context.Context, in []I) ([]O, error) {
			out := make([]O, len(in))
			errs := make([]error, len(in))
			limit := concurrency
			if limit <= 0 || limit > len(in) {
				limit = max(len(in), 1)
			}
			sem := make(chan struct{}, limit)
			var wg sync.WaitGroup
			for i := range in {
				sem <- struct{}{}
				wg.Go(func() {
					defer func() { <-sem }()
					defer func() { errs[i] = errors.Join(errs[i], recovered(recover(), s.displayName())) }()
					out[i], errs[i] = s.Run(ctx, in[i])
				})
			}
			wg.Wait()
			if err := errors.Join(errs...); err != nil {
				return nil, err
			}
			return out, nil
		},
	}
}

// Seq chains steps that share one type. It is [Step.Then] for the common case
// where the type does not change.
func Seq[T any](name string, steps ...Step[T, T]) Step[T, T] {
	kids := make([]fmt.Stringer, len(steps))
	for i, s := range steps {
		kids[i] = s
	}
	return Step[T, T]{
		name: name,
		kids: kids,
		run: func(ctx context.Context, in T) (T, error) {
			var err error
			for _, s := range steps {
				in, err = s.Run(ctx, in)
				if err != nil {
					var zero T
					return zero, err
				}
			}
			return in, nil
		},
	}
}

// If runs then when pred returns true, and els when it returns false.
func If[I, O any](name string, pred func(context.Context, I) bool, then, els Step[I, O]) Step[I, O] {
	return Step[I, O]{
		name: name,
		kids: []fmt.Stringer{then.Rename("IF " + then.displayName()), els.Rename("ELSE " + els.displayName())},
		run: func(ctx context.Context, in I) (O, error) {
			if pred(ctx, in) {
				return then.Run(ctx, in)
			}
			return els.Run(ctx, in)
		},
	}
}
