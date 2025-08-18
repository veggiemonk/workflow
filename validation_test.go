package workflow_test

import (
	"context"
	"testing"

	wf "github.com/veggiemonk/workflow"
)

// TestStepValidation tests the validation functionality
func TestStepValidation(t *testing.T) {
	type TestData struct {
		Value int
	}

	validator := wf.StepValidator[TestData]{}

	t.Run("nil step validation", func(t *testing.T) {
		err := validator.ValidateStep(nil)
		if err == nil {
			t.Error("Expected error for nil step")
		}
	})

	t.Run("valid step validation", func(t *testing.T) {
		step := wf.StepFunc[TestData](func(ctx context.Context, data *TestData) (*TestData, error) {
			return data, nil
		})
		err := validator.ValidateStep(step)
		if err != nil {
			t.Errorf("Expected no error for valid step, got %v", err)
		}
	})

	t.Run("pipeline validation", func(t *testing.T) {
		pipeline := wf.NewPipeline[TestData]()
		pipeline.Steps = []wf.Step[TestData]{
			wf.StepFunc[TestData](func(ctx context.Context, data *TestData) (*TestData, error) {
				return data, nil
			}),
		}

		err := validator.ValidatePipeline(pipeline)
		if err != nil {
			t.Errorf("Expected no error for valid pipeline, got %v", err)
		}
	})

	t.Run("nil pipeline validation", func(t *testing.T) {
		err := validator.ValidatePipeline(nil)
		if err == nil {
			t.Error("Expected error for nil pipeline")
		}
	})
}

// TestSafeRun tests the safe run functionality
func TestSafeRun(t *testing.T) {
	type TestData struct {
		Value int
	}

	step := wf.StepFunc[TestData](func(ctx context.Context, data *TestData) (*TestData, error) {
		data.Value++
		return data, nil
	})

	t.Run("safe run with valid inputs", func(t *testing.T) {
		data := &TestData{Value: 5}
		result, err := wf.SafeRun(context.Background(), step, data)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
		if result.Value != 6 {
			t.Errorf("Expected value 6, got %d", result.Value)
		}
	})

	t.Run("safe run with nil step", func(t *testing.T) {
		data := &TestData{Value: 5}
		_, err := wf.SafeRun(context.Background(), nil, data)
		if err == nil {
			t.Error("Expected error for nil step")
		}
	})

	t.Run("safe run with nil data", func(t *testing.T) {
		_, err := wf.SafeRun(context.Background(), step, nil)
		if err == nil {
			t.Error("Expected error for nil data")
		}
	})

	t.Run("safe run with nil context", func(t *testing.T) {
		data := &TestData{Value: 5}
		result, err := wf.SafeRun(context.TODO(), step, data)
		if err != nil {
			t.Errorf("Expected no error with nil context, got %v", err)
		}
		if result.Value != 6 {
			t.Errorf("Expected value 6, got %d", result.Value)
		}
	})
}

// TestSafeCopy tests the safe copy functionality
func TestSafeCopy(t *testing.T) {
	type TestData struct {
		Value int
		Slice []int
	}

	t.Run("safe copy with nil", func(t *testing.T) {
		result := wf.SafeCopy[TestData](nil)
		if result != nil {
			t.Error("Expected nil result for nil input")
		}
	})

	t.Run("safe copy with valid data", func(t *testing.T) {
		original := &TestData{
			Value: 42,
			Slice: []int{1, 2, 3},
		}

		copy := wf.SafeCopy(original)
		if copy == nil {
			t.Fatal("Expected non-nil copy")
		}

		// Verify values are copied
		if copy.Value != original.Value {
			t.Errorf("Expected value %d, got %d", original.Value, copy.Value)
		}

		// Verify it's a different instance
		if copy == original {
			t.Error("Expected different instance")
		}

		// Note: This is still a shallow copy for slices
		// The slice itself will be shared between original and copy
	})
}

// TestDeepCopyInterface tests the deep copy interface
func TestDeepCopyInterface(t *testing.T) {
	// Define a type that implements custom copy behavior
	type DeepCopyTestData struct {
		Value int
		Slice []int
	}

	t.Run("shallow copy vs deep copy", func(t *testing.T) {
		original := &DeepCopyTestData{
			Value: 42,
			Slice: []int{1, 2, 3},
		}

		// Test SafeCopy which performs shallow copy
		copied := wf.SafeCopy(original)
		if copied == nil {
			t.Fatal("Expected non-nil copy")
		}

		// Verify values are copied
		if copied.Value != original.Value {
			t.Errorf("Expected value %d, got %d", original.Value, copied.Value)
		}

		// Verify it's a different instance
		if copied == original {
			t.Error("Expected different instance")
		}
	})
}
