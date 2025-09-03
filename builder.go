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
		task, err := b.registry.Get(taskSpec.Name)
		if err != nil {
			return nil, err
		}
		pipeline.Tasks = append(pipeline.Tasks, task)
	}
	return pipeline, nil
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
