package workflow

import (
	"fmt"
)

// Registry is a repository for named Tasks, Selectors, and MergeRequests.
type Registry[T any] struct {
	tasks     map[string]*Task[T]
	selectors map[string]Selector[T]
	merges    map[string]MergeRequest[T]
}

// NewRegistry creates a new Registry.
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		tasks:     make(map[string]*Task[T]),
		selectors: make(map[string]Selector[T]),
		merges:    make(map[string]MergeRequest[T]),
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

// RegisterSelector adds a Selector to the Registry.
func (r *Registry[T]) RegisterSelector(name string, selector Selector[T]) {
	r.selectors[name] = selector
}

// GetSelector retrieves a Selector from the Registry by name.
func (r *Registry[T]) GetSelector(name string) (Selector[T], error) {
	selector, ok := r.selectors[name]
	if !ok {
		return nil, fmt.Errorf("selector not found: %s", name)
	}
	return selector, nil
}

// RegisterMergeRequest adds a MergeRequest to the Registry.
func (r *Registry[T]) RegisterMergeRequest(name string, merge MergeRequest[T]) {
	r.merges[name] = merge
}

// GetMergeRequest retrieves a MergeRequest from the Registry by name.
func (r *Registry[T]) GetMergeRequest(name string) (MergeRequest[T], error) {
	merge, ok := r.merges[name]
	if !ok {
		return nil, fmt.Errorf("merge request not found: %s", name)
	}
	return merge, nil
}
