package workflow

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"reflect"
	"runtime/debug"
	"slices"
	"strings"
	"sync"

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

// PipelineSpec is the serializable representation of a Pipeline.
type PipelineSpec struct {
	Tasks []*TaskSpec `json:"tasks"`
}

// Pipeline is a step that executes a series of other tasks in sequential order.
// It can also have middleware that is applied to each task in the pipeline.
type Pipeline[T any] struct {
	Tasks      []*Task[T]
	Middleware []Middleware[T]
}

// Run executes the pipeline.
func (p *Pipeline[T]) Run(ctx context.Context, req *T) (*T, error) {
	resp := req
	var err error
	for i := range p.Tasks {
		for _, m := range slices.Backward(p.Middleware) {
			p.Tasks[i] = m(p.Tasks[i])
		}
		resp, err = p.Tasks[i].Run(ctx, req)
		if err != nil {
			return nil, err
		}
		req = resp
	}
	return resp, nil
}

func (p *Pipeline[T]) String() string {
	if len(p.Tasks) == 0 {
		return Name(p)
	}
	var buf strings.Builder
	buf.WriteString("\n")
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

		s := task.String()
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

// ToSpec returns the serializable representation of the Pipeline.
func (p *Pipeline[T]) ToSpec() *PipelineSpec {
	spec := &PipelineSpec{
		Tasks: make([]*TaskSpec, len(p.Tasks)),
	}
	for i, task := range p.Tasks {
		spec.Tasks[i] = task.ToSpec()
	}
	return spec
}

// Save saves the pipeline to a file.
func (p *Pipeline[T]) Save(path string) error {
	spec := p.ToSpec()
	b, err := json.MarshalIndent(spec, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, b, 0644)
}

// NewPipeline creates a new pipeline with the given middleware.
func NewPipeline[T any](mid ...Middleware[T]) *Pipeline[T] {
	return &Pipeline[T]{
		Middleware: mid,
		Tasks:      make([]*Task[T], 0),
	}
}

// StepFunc is a function type for a unit of work in the workflow.
// It is an adapter to allow the use of ordinary functions as workflow steps.
type StepFunc[T any] func(context.Context, *T) (*T, error)

// Run executes the function that implements the Step interface.
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
	Next *Task[T]
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
type Middleware[T any] func(s *Task[T]) *Task[T]

// Selector

// Selector is a function that returns true or false based on the context and the request.
// Selector is a function type used to select steps conditionally in a workflow.
type Selector[T any] func(context.Context, *T) bool

// selector is a step that executes one of two other steps based on the result
// of a selector function.
type selector[T any] struct {
	s          Selector[T]
	ifTask     *Task[T]
	elseTask   *Task[T]
	middleware []Middleware[T]
	name       string
}

// String returns the name of the selector.
func (s selector[T]) String() string {
	var buf strings.Builder
	buf.WriteString(Name(&s))

	// IF
	buf.WriteString("\n")
	buf.WriteString("├── IF: ")
	if s.ifTask != nil {
		ifStr := s.ifTask.String()
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
	if s.elseTask != nil {
		elseStr := s.elseTask.String()
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
func Select[T any](mid []Middleware[T], name string, s Selector[T], ifTask, elseTask *Task[T]) Step[T] {
	return &selector[T]{
		s:          s,
		ifTask:     ifTask,
		elseTask:   elseTask,
		middleware: mid,
		name:       name,
	}
}

// Run executes the selector.
func (s selector[T]) Run(ctx context.Context, r *T) (*T, error) {
	var task *Task[T]
	if s.s(ctx, r) {
		task = s.ifTask
	} else {
		task = s.elseTask
	}
	if task == nil {
		return nil, fmt.Errorf("selector has no task for the selected condition")
	}
	for _, m := range slices.Backward(s.middleware) {
		task = m(task)
	}
	return task.Run(ctx, r)
}

// ToSpec returns the serializable representation of the selector.
func (s selector[T]) ToSpec() *TaskSpec {
	spec := &TaskSpec{
		Type:     "selector",
		Selector: s.name,
	}
	if s.ifTask != nil {
		spec.IfTask = s.ifTask.ToSpec()
	}
	if s.elseTask != nil {
		spec.ElseTask = s.elseTask.ToSpec()
	}
	return spec
}

// Series

// series is a step that executes a list of other steps sequentially.
type series[T any] struct {
	Tasks      []*Task[T]
	middleware []Middleware[T]
}

// String returns the name of the series.
func (s *series[T]) String() string {
	if s == nil {
		return "none"
	}
	if len(s.Tasks) == 0 {
		return Name(s)
	}
	var buf strings.Builder
	buf.WriteString(Name(s))
	for i, task := range s.Tasks {
		buf.WriteString("\n")
		var prefix string
		var childPrefix string
		if i == len(s.Tasks)-1 {
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

// Sequential executes a series of steps in sequential order.
func Sequential[T any](mid []Middleware[T], tasks ...*Task[T]) *series[T] {
	// Series creates a sequential pipeline of steps with optional middleware.
	// Returns a private type *series[T].
	return &series[T]{
		Tasks:      tasks,
		middleware: mid,
	}
}

// Run executes the series.
func (s *series[T]) Run(ctx context.Context, req *T) (*T, error) {
	var err error
	resp := req

	for i := range s.Tasks {
		for _, m := range slices.Backward(s.middleware) {
			s.Tasks[i] = m(s.Tasks[i])
		}
		resp, err = s.Tasks[i].Run(ctx, req)
		if err != nil {
			return resp, err
		}
		req = resp
	}
	return resp, nil
}

// ToSpec returns the serializable representation of the series.
func (s *series[T]) ToSpec() *TaskSpec {
	spec := &TaskSpec{
		Type:  "series",
		Tasks: make([]*TaskSpec, len(s.Tasks)),
	}
	for i, task := range s.Tasks {
		spec.Tasks[i] = task.ToSpec()
	}
	return spec
}

// Parallel

// parallel is a step that executes a list of other steps in parallel.
type parallel[T any] struct {
	merge      MergeRequest[T]
	Tasks      []*Task[T]
	middleware []Middleware[T]
	name       string
}

// ToSpec returns the serializable representation of the parallel.
func (p *parallel[T]) ToSpec() *TaskSpec {
	spec := &TaskSpec{
		Type:  "parallel",
		Tasks: make([]*TaskSpec, len(p.Tasks)),
		Merge: p.name,
	}
	for i, task := range p.Tasks {
		spec.Tasks[i] = task.ToSpec()
	}
	return spec
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
func Parallel[T any](mid []Middleware[T], name string, merge MergeRequest[T], tasks ...*Task[T]) *parallel[T] {
	return &parallel[T]{
		merge:      merge,
		Tasks:      tasks,
		middleware: mid,
		name:       name,
	}
}

// Run executes the parallel step.
func (p *parallel[T]) Run(ctx context.Context, req *T) (*T, error) {
	tasks := make([]*Task[T], len(p.Tasks))
	for i, s := range p.Tasks {
		tasks[i] = s
		for _, m := range slices.Backward(p.middleware) {
			tasks[i] = m(tasks[i])
		}
	}
	g, groupCtx := errgroup.WithContext(ctx)
	resps := make([]*T, len(p.Tasks))
	mu := sync.Mutex{}
	for i := range tasks {
		task := tasks[i] // Capture task
		g.Go(func() error {
			defer CapturePanic(groupCtx)

			copyReq := SafeCopy(req)
			resp, err := task.Run(groupCtx, copyReq)
			if err != nil {
				return err
			}
			mu.Lock()
			resps[i] = resp
			mu.Unlock()
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
func MergeTransform[T any](opts ...func(*mergo.Config)) MergeRequest[T] {
	return func(ctx context.Context, res *T, responses ...*T) (*T, error) {
		var err error
		for _, r := range responses {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("aborting: %w", ctx.Err())
			default:
				err = mergo.Merge(res, r, opts...)
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

// CapturePanic recovers from a panic and logs the error with stack trace.
func CapturePanic(ctx context.Context) {
	if r := recover(); r != nil {
		slog.Error("panic recovered",
			"error", r,
			"context_cancelled", ctx.Err() != nil,
			"stack", string(debug.Stack()),
		)
	}
}
