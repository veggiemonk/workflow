package workflow

import (
	"context"
	"fmt"
)

// StepValidator provides validation for workflow steps
type StepValidator[T any] struct{}

// ValidateStep validates a step for common issues
func (v StepValidator[T]) ValidateStep(step Step[T]) error {
	if step == nil {
		return fmt.Errorf("step cannot be nil")
	}

	// Test string representation
	if step.String() == "" {
		return fmt.Errorf("step must provide a non-empty string representation")
	}

	return nil
}

// ValidatePipeline validates an entire pipeline
func (v StepValidator[T]) ValidatePipeline(pipeline *Pipeline[T]) error {
	if pipeline == nil {
		return fmt.Errorf("pipeline cannot be nil")
	}

	for i, step := range pipeline.Steps {
		if err := v.ValidateStep(step); err != nil {
			return fmt.Errorf("step %d validation failed: %w", i, err)
		}
	}

	return nil
}

// SafeRun provides a safe way to run steps with validation
func SafeRun[T any](ctx context.Context, step Step[T], data *T) (*T, error) {
	if step == nil {
		return nil, fmt.Errorf("cannot run nil step")
	}

	if data == nil {
		return nil, fmt.Errorf("cannot run step with nil data")
	}

	// Check context
	if ctx == nil {
		ctx = context.Background()
	}

	return step.Run(ctx, data)
}

// DeepCopyInterface defines an interface for types that can deep copy themselves
type DeepCopyInterface[T any] interface {
	DeepCopy() *T
}

// SafeCopy provides safe copying for parallel execution
func SafeCopy[T any](original *T) *T {
	if original == nil {
		return nil
	}

	// Check if the type implements DeepCopy
	if copyable, ok := any(original).(DeepCopyInterface[T]); ok {
		return copyable.DeepCopy()
	}

	// Fall back to shallow copy (existing behavior)
	cp := new(T)
	*cp = *original
	return cp
}
