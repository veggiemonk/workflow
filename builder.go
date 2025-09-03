package workflow

import (
	"encoding/json"
	"os"
)

// Builder builds a Pipeline from a specification.
type Builder[T any] struct {
	registry *Registry[T]
}

// NewBuilder creates a new Builder.
func NewBuilder[T any](registry *Registry[T]) *Builder[T] {
	return &Builder[T]{
		registry: registry,
	}
}

// BuildFromSpec constructs a Pipeline from a PipelineSpec.
func (b *Builder[T]) BuildFromSpec(spec *PipelineSpec) (*Pipeline[T], error) {
	pipeline := NewPipeline[T]()
	for _, taskSpec := range spec.Tasks {
		task, err := b.buildTaskFromSpec(taskSpec)
		if err != nil {
			return nil, err
		}
		pipeline.Tasks = append(pipeline.Tasks, task)
	}
	return pipeline, nil
}

func (b *Builder[T]) buildTaskFromSpec(spec *TaskSpec) (*Task[T], error) {
	switch spec.Type {
	case "step":
		return b.registry.Get(spec.Name)
	case "series":
		tasks := make([]*Task[T], len(spec.Tasks))
		for i, taskSpec := range spec.Tasks {
			task, err := b.buildTaskFromSpec(taskSpec)
			if err != nil {
				return nil, err
			}
			tasks[i] = task
		}
		return NewTask(spec.Name, Sequential(nil, tasks...)), nil
	case "parallel":
		tasks := make([]*Task[T], len(spec.Tasks))
		for i, taskSpec := range spec.Tasks {
			task, err := b.buildTaskFromSpec(taskSpec)
			if err != nil {
				return nil, err
			}
			tasks[i] = task
		}
		merge, err := b.registry.GetMergeRequest(spec.Merge)
		if err != nil {
			return nil, err
		}
		return NewTask(spec.Name, Parallel(nil, merge, tasks...)), nil
	case "selector":
		var ifTask, elseTask *Task[T]
		var err error
		if spec.IfTask != nil {
			ifTask, err = b.buildTaskFromSpec(spec.IfTask)
			if err != nil {
				return nil, err
			}
		}
		if spec.ElseTask != nil {
			elseTask, err = b.buildTaskFromSpec(spec.ElseTask)
			if err != nil {
				return nil, err
			}
		}
		selector, err := b.registry.GetSelector(spec.Selector)
		if err != nil {
			return nil, err
		}
		return NewTask(spec.Name, Select(nil, selector, ifTask, elseTask)), nil
	default:
		return b.registry.Get(spec.Name)
	}
}

// BuildFromFile constructs a Pipeline from a JSON file.
func (b *Builder[T]) BuildFromFile(path string) (*Pipeline[T], error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var spec PipelineSpec
	if err := json.Unmarshal(data, &spec); err != nil {
		return nil, err
	}
	return b.BuildFromSpec(&spec)
}
