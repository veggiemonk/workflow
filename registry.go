package workflow

import (
	"fmt"
)

// Registry is a repository for named Tasks.
type Registry[T any] struct {
	tasks map[string]*Task[T]
}

// NewRegistry creates a new Registry.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		tasks: make(map[string]*Task[T]),
	}
}

// Register adds a Task to the Registry.
func (r *Registry[T]) Register(task *Task[T]) {
	r.tasks[task.Name()] = task
}

// Get retrieves a Task from the Registry by name.
func (r *Registry[T]) Get(name string) (*Task[T], error) {
	task, ok := r.tasks[name]
	if !ok {
		return nil, fmt.Errorf("task not found: %s", name)
	}
	return task, nil
}
