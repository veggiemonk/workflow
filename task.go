package workflow

import (
	"context"
)

// TaskSpec is the serializable representation of a Task.
type TaskSpec struct {
	Name     string      `json:"name"`
	Type     string      `json:"type,omitempty"`
	Tasks    []*TaskSpec `json:"tasks,omitempty"`
	IfTask   *TaskSpec   `json:"if_task,omitempty"`
	ElseTask *TaskSpec   `json:"else_task,omitempty"`
	Selector string      `json:"selector,omitempty"`
	Merge    string      `json:"merge,omitempty"`
}

// Task is a named Step.
type Task[T any] struct {
	name string
	step Step[T]
}

// NewTask creates a new Task.
func NewTask[T any](name string, step Step[T]) *Task[T] {
	return &Task[T]{
		name: name,
		step: step,
	}
}

// Name returns the name of the task.
func (t *Task[T]) Name() string {
	return t.name
}

// Run executes the task's step.
func (t *Task[T]) Run(ctx context.Context, req *T) (*T, error) {
	return t.step.Run(ctx, req)
}

// String returns the string representation of the task's step.
func (t *Task[T]) String() string {
	return t.step.String()
}

// ToSpecer is an interface for types that can be converted to a TaskSpec.
type ToSpecer interface {
	ToSpec() *TaskSpec
}

// ToSpec returns the serializable representation of the Task.
func (t *Task[T]) ToSpec() *TaskSpec {
	if ts, ok := t.step.(ToSpecer); ok {
		spec := ts.ToSpec()
		spec.Name = t.name // ensure the name is preserved
		return spec
	}
	return &TaskSpec{
		Name: t.name,
		Type: "step",
	}
}
