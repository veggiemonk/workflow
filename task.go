package workflow

import (
	"context"
)

// TaskSpec is the serializable representation of a Task.
type TaskSpec struct {
	Name string `json:"name"`
	// TODO: Add other properties needed for reconstruction, e.g., for selector, parallel.
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

// ToSpec returns the serializable representation of the Task.
func (t *Task[T]) ToSpec() *TaskSpec {
	return &TaskSpec{
		Name: t.name,
	}
}
