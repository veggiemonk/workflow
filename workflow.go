package workflow

import (
	"context"
	"fmt"
	"log/slog"
	"reflect"
	"slices"
	"strings"

	"dario.cat/mergo"
	"golang.org/x/sync/errgroup"
)

// Step is the basic unit of work in a workflow. It is an interface with a
// single method, Run, that takes a context and a generic request type T and
// returns a response of the same type T and an error.
type Step[T any] interface {
	Run(context.Context, *T) (*T, error)
	fmt.Stringer
}

// Name returns the name of a step.
func Name[T any](s Step[T]) string {
	t := reflect.TypeOf(s)
	if t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	var z [0]T // zero alloc
	return strings.Replace(t.Name(), reflect.TypeOf(z).Elem().PkgPath()+".", "", 1)
}

type typ struct{}

var (
	_ Step[typ] = (*Pipeline[typ])(nil)
	_ Step[typ] = (*MidFunc[typ])(nil)
	_ Step[typ] = (*series[typ])(nil)
	_ Step[typ] = (*parallel[typ])(nil)
	_ Step[typ] = (*selector[typ])(nil)
)

// Pipeline is a step that executes a series of other steps in sequential order.
// It can also have middleware that is applied to each step in the pipeline.
type Pipeline[T any] struct {
	Steps []Step[T]
	Mid[T]
}

// Run executes the pipeline.
func (p *Pipeline[T]) Run(ctx context.Context, req *T) (*T, error) {
	resp := req
	var err error
	for i := range p.Steps {
		for _, m := range slices.Backward(p.Mid) {
			p.Steps[i] = m(p.Steps[i])
		}
		resp, err = p.Steps[i].Run(ctx, req)
		if err != nil {
			return nil, err
		}
		req = resp
	}
	return resp, nil
}

func (p *Pipeline[T]) String() string {
	if len(p.Steps) == 0 {
		return Name(p)
	}
	var buf strings.Builder
	buf.WriteString("\n")
	buf.WriteString(Name(p))
	for i, step := range p.Steps {
		buf.WriteString("\n")
		var prefix string
		var childPrefix string
		if i == len(p.Steps)-1 {
			prefix = "└── "
			childPrefix = "    "
		} else {
			prefix = "├── "
			childPrefix = "│   "
		}
		buf.WriteString(prefix)

		s := step.String()
		lines := strings.Split(s, "\n")
		buf.WriteString(lines[0])
		for _, line := range lines[1:] {
			buf.WriteString("\n")
			buf.WriteString(childPrefix)
			buf.WriteString(line)
		}
	}
	buf.WriteString("\n")
	return buf.String()
}

// NewPipeline creates a new pipeline with the given middleware.
func NewPipeline[T any](mid ...Middleware[T]) *Pipeline[T] {
	return &Pipeline[T]{
		Mid:   mid,
		Steps: make([]Step[T], 0),
	}
}

// StepFunc is an adapter to allow the use of ordinary functions as workflow steps.

type StepFunc[T any] func(context.Context, *T) (*T, error)

// Run executes the function.
func (f StepFunc[T]) Run(ctx context.Context, res *T) (*T, error) {
	return f(ctx, res)
}

// String returns the name of the function.
func (f StepFunc[T]) String() string {
	var z T
	return fmt.Sprintf("StepFunc[%T]", z)
}

// Middleware

// MidFunc is an adapter to allow the use of ordinary functions as middleware.
type MidFunc[T any] struct {
	Name string
	Fn   func(context.Context, *T) (*T, error)
	Next Step[T]
}

// Run executes the function.
func (m *MidFunc[T]) Run(ctx context.Context, req *T) (*T, error) {
	return m.Fn(ctx, req)
}

// String returns the name of the function.
func (m *MidFunc[T]) String() string {
	s := m.Next.String()
	return fmt.Sprintf("%s(%s)", m.Name, s)
}

// Middleware is a function that wraps a step to add functionality, such as
// logging or error handling.
type Middleware[T any] func(s Step[T]) Step[T]

// Mid is a slice of middleware.
type Mid[T any] []Middleware[T]

// Selector

// Selector is a function that returns true or false based on the context and
// the request.

type Selector[T any] func(context.Context, *T) bool

// selector is a step that executes one of two other steps based on the result
// of a selector function.
type selector[T any] struct {
	s        Selector[T]
	ifStep   Step[T]
	elseStep Step[T]
	Mid[T]
}

// String returns the name of the selector.
func (s selector[T]) String() string {
	var buf strings.Builder
	buf.WriteString(Name(&s))

	// IF
	buf.WriteString("\n")
	buf.WriteString("├── IF: ")
	if s.ifStep != nil {
		ifStr := s.ifStep.String()
		lines := strings.Split(ifStr, "\n")
		buf.WriteString(lines[0])
		for _, line := range lines[1:] {
			buf.WriteString("\n")
			buf.WriteString("│   ")
			buf.WriteString(line)
		}
	} else {
		buf.WriteString("none")
	}

	// ELSE
	buf.WriteString("\n")
	buf.WriteString("└── ELSE: ")
	if s.elseStep != nil {
		elseStr := s.elseStep.String()
		lines := strings.Split(elseStr, "\n")
		buf.WriteString(lines[0])
		for _, line := range lines[1:] {
			buf.WriteString("\n")
			buf.WriteString("    ")
			buf.WriteString(line)
		}
	} else {
		buf.WriteString("none")
	}
	return buf.String()
}

// Select creates a new selector step.
func Select[T any](mid Mid[T], s Selector[T], ifStep, elseStep Step[T]) Step[T] {
	return &selector[T]{
		s:        s,
		ifStep:   ifStep,
		elseStep: elseStep,
		Mid:      mid,
	}
}

// Run executes the selector.
func (s selector[T]) Run(ctx context.Context, r *T) (*T, error) {
	var step Step[T]
	if s.s(ctx, r) {
		step = s.ifStep
	}
	if s.elseStep != nil {
		step = s.elseStep
	}
	if step == nil {
		return nil, fmt.Errorf("selector chosed missing else branch: %v", r)
	}
	for _, m := range slices.Backward(s.Mid) {
		step = m(step)
	}
	return step.Run(ctx, r)
}

// Series

// series is a step that executes a list of other steps sequentially.
type series[T any] struct {
	Stages []Step[T]
	Mid[T]
}

// String returns the name of the series.
func (s *series[T]) String() string {
	if s == nil {
		return "none"
	}
	if len(s.Stages) == 0 {
		return Name(s)
	}
	var buf strings.Builder
	buf.WriteString(Name(s))
	for i, stage := range s.Stages {
		buf.WriteString("\n")
		var prefix string
		var childPrefix string
		if i == len(s.Stages)-1 {
			prefix = "└── "
			childPrefix = "    "
		} else {
			prefix = "├── "
			childPrefix = "│   "
		}
		buf.WriteString(prefix)

		st := stage.String()
		lines := strings.Split(st, "\n")
		buf.WriteString(lines[0])
		for _, line := range lines[1:] {
			buf.WriteString("\n")
			buf.WriteString(childPrefix)
			buf.WriteString(line)
		}
	}
	return buf.String()
}

// Series executes a series of steps in sequential order.
func Series[T any](mid Mid[T], steps ...Step[T]) *series[T] {
	return &series[T]{
		Stages: steps,
		Mid:    mid,
	}
}

// Run executes the series.
func (s *series[T]) Run(ctx context.Context, req *T) (*T, error) {
	var err error
	resp := req

	for i := range s.Stages {
		for _, m := range slices.Backward(s.Mid) {
			s.Stages[i] = m(s.Stages[i])
		}
		resp, err = s.Stages[i].Run(ctx, req)
		if err != nil {
			return resp, err
		}
		req = resp
	}
	return resp, nil
}

// Parallel

// parallel is a step that executes a list of other steps in parallel.
type parallel[T any] struct {
	merge MergeRequest[T]
	Tasks []Step[T]
	Mid[T]
}

// String returns the name of the parallel step.
func (p *parallel[T]) String() string {
	if p == nil {
		return "none"
	}
	if len(p.Tasks) == 0 {
		return Name(p)
	}
	var buf strings.Builder
	buf.WriteString(Name(p))
	for i, task := range p.Tasks {
		buf.WriteString("\n")
		var prefix string
		var childPrefix string
		if i == len(p.Tasks)-1 {
			prefix = "└── "
			childPrefix = "    "
		} else {
			prefix = "├── "
			childPrefix = "│   "
		}
		buf.WriteString(prefix)

		st := task.String()
		lines := strings.Split(st, "\n")
		buf.WriteString(lines[0])
		for _, line := range lines[1:] {
			buf.WriteString("\n")
			buf.WriteString(childPrefix)
			buf.WriteString(line)
		}
	}
	return buf.String()
}

// MergeRequest is a function that merges the results of multiple steps into a
// single result.
type MergeRequest[T any] func(context.Context, *T, ...*T) (*T, error)

// Parallel executes a list of steps in parallel.
// Once all the steps are done, the merge request [MergeRequest] will combine all the results into one struct T.
func Parallel[T any](mid Mid[T], merge MergeRequest[T], steps ...Step[T]) *parallel[T] {
	return &parallel[T]{
		merge: merge,
		Tasks: steps,
		Mid:   mid,
	}
}

// Run executes the parallel step.
func (p *parallel[T]) Run(ctx context.Context, req *T) (*T, error) {
	tasks := make([]Step[T], len(p.Tasks))
	for i, s := range p.Tasks {
		tasks[i] = s
		for _, m := range slices.Backward(p.Mid) {
			tasks[i] = m(tasks[i])
		}
	}
	g, groupCtx := errgroup.WithContext(ctx)
	resps := make([]*T, len(p.Tasks))
	for i := range tasks {
		g.Go(func() error {
			defer CapturePanic(groupCtx)

			copyReq := new(T)
			*copyReq = *req
			resp, err := tasks[i].Run(ctx, copyReq)
			if err != nil {
				return err
			}
			resps[i] = resp
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return p.merge(ctx, req, resps...)
}

// MergeTransform is a merge request that merges the results of multiple steps
// into a single result using the mergo library.
func MergeTransform[T any](t ...func(*mergo.Config)) MergeRequest[T] {
	return func(ctx context.Context, res *T, responses ...*T) (*T, error) {
		var err error
		for _, r := range responses {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("aborting: %w", ctx.Err())
			default:

				err = mergo.Merge(res, r, t...)
				if err != nil {
					return nil, err
				}
			}
		}
		return res, nil
	}
}

// Merge is a merge request that merges the results of multiple steps into a
// single result using the mergo library.
func Merge[T any](ctx context.Context, req *T, responses ...*T) (*T, error) {
	return MergeTransform[T]()(ctx, req, responses...)
}

// CapturePanic recovers from a panic and logs the error.
func CapturePanic(ctx context.Context) {
	if r := recover(); r != nil {
		slog.Error("panic recover", r)
	}
}
