package workflow_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"dario.cat/mergo"
	wf "github.com/veggiemonk/workflow"
)

// TestSelectorLogicFix tests the fixed selector logic
func TestSelectorLogicFix(t *testing.T) {
	type TestData struct {
		Value   int
		Message string
	}

	ifStep := wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
		data.Message = "if branch executed"
		return data, nil
	})

	elseStep := wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
		data.Message = "else branch executed"
		return data, nil
	})

	tests := []struct {
		name           string
		conditionValue int
		expectedMsg    string
	}{
		{"condition true", 1, "if branch executed"},
		{"condition false", 0, "else branch executed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := wf.Select(nil,
				func(_ context.Context, data *TestData) bool {
					return data.Value > 0
				},
				ifStep,
				elseStep,
			)

			result, err := selector.Run(t.Context(), &TestData{Value: tt.conditionValue})
			if err != nil {
				t.Fatalf("Expected no error, got %v", err)
			}

			if result.Message != tt.expectedMsg {
				t.Errorf("Expected message %q, got %q", tt.expectedMsg, result.Message)
			}
		})
	}
}

// TestSelectorWithNilSteps tests selector behavior with nil steps
func TestSelectorWithNilSteps(t *testing.T) {
	type TestData struct {
		Value int
	}

	tests := []struct {
		name        string
		ifStep      wf.Step[TestData]
		elseStep    wf.Step[TestData]
		condition   bool
		expectError bool
	}{
		{"both steps nil", nil, nil, true, true},
		{"if step nil, condition true", nil, wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) { return data, nil }), true, true},
		{"else step nil, condition false", wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) { return data, nil }), nil, false, true},
		{"both steps present", wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) { return data, nil }), wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) { return data, nil }), true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			selector := wf.Select(nil,
				func(_ context.Context, _ *TestData) bool {
					return tt.condition
				},
				tt.ifStep,
				tt.elseStep,
			)

			_, err := selector.Run(t.Context(), &TestData{Value: 1})
			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Expected no error but got: %v", err)
			}
		})
	}
}

// TestPipelineErrorPropagation tests error propagation through pipeline
func TestPipelineErrorPropagation(t *testing.T) {
	type TestData struct {
		Value int
		Steps []string
	}

	expectedErr := errors.New("step error")

	pipeline := wf.NewPipeline[TestData]()
	pipeline.Steps = []wf.Step[TestData]{
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Steps = append(data.Steps, "step1")
			return data, nil
		}),
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Steps = append(data.Steps, "step2")
			return data, expectedErr // This should stop the pipeline
		}),
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Steps = append(data.Steps, "step3") // This should not execute
			return data, nil
		}),
	}

	result, err := pipeline.Run(t.Context(), &TestData{})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	// The result should be nil when an error occurs
	if result != nil {
		t.Error("Expected nil result when error occurs")
	}
}

// TestParallelExecutionWithErrors tests error handling in parallel execution
func TestParallelExecutionWithErrors(t *testing.T) {
	type TestData struct {
		Value int
		Count int
	}

	expectedErr := errors.New("parallel step error")

	successStep := wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
		data.Count++
		return data, nil
	})

	errorStep := wf.StepFunc[TestData](func(_ context.Context, _ *TestData) (*TestData, error) {
		return nil, expectedErr
	})

	parallelStep := wf.Parallel(nil, wf.Merge[TestData], successStep, errorStep, successStep)

	_, err := parallelStep.Run(t.Context(), &TestData{})

	if err == nil {
		t.Error("Expected error from parallel execution but got none")
	}

	if !errors.Is(err, expectedErr) {
		t.Errorf("Expected error to contain %v, got %v", expectedErr, err)
	}
}

// TestContextCancellation tests context cancellation behavior
func TestContextCancellation(t *testing.T) {
	type TestData struct {
		Message string
	}

	slowStep := wf.StepFunc[TestData](func(ctx context.Context, data *TestData) (*TestData, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			data.Message = "completed"
			return data, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	pipeline := wf.NewPipeline[TestData]()
	pipeline.Steps = []wf.Step[TestData]{slowStep}

	_, err := pipeline.Run(ctx, &TestData{})

	if err == nil {
		t.Error("Expected context cancellation error but got none")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected deadline exceeded error, got %v", err)
	}
}

// TestParallelContextCancellation tests context cancellation in parallel execution
func TestParallelContextCancellation(t *testing.T) {
	type TestData struct {
		Count int
	}

	slowStep := wf.StepFunc[TestData](func(ctx context.Context, data *TestData) (*TestData, error) {
		select {
		case <-time.After(100 * time.Millisecond):
			data.Count++
			return data, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	parallelStep := wf.Parallel(nil, wf.Merge[TestData], slowStep, slowStep, slowStep)

	_, err := parallelStep.Run(ctx, &TestData{})

	if err == nil {
		t.Error("Expected context cancellation error but got none")
	}

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Expected deadline exceeded error, got %v", err)
	}
}

// TestEmptyParallelExecution tests parallel execution with no steps
func TestEmptyParallelExecution(t *testing.T) {
	type TestData struct {
		Value int
	}

	parallelStep := wf.Parallel(nil, wf.Merge[TestData])

	result, err := parallelStep.Run(t.Context(), &TestData{Value: 42})
	if err != nil {
		t.Errorf("Expected no error for empty parallel execution, got %v", err)
	}

	if result == nil {
		t.Error("Expected non-nil result for empty parallel execution")
	}

	if result.Value != 42 {
		t.Errorf("Expected value to be preserved, got %d", result.Value)
	}
}

// TestSeriesErrorPropagation tests error propagation in series execution
func TestSeriesErrorPropagation(t *testing.T) {
	type TestData struct {
		Steps []string
	}

	expectedErr := errors.New("series step error")

	series := wf.Sequential(nil,
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Steps = append(data.Steps, "step1")
			return data, nil
		}),
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			return data, expectedErr
		}),
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Steps = append(data.Steps, "step3") // Should not execute
			return data, nil
		}),
	)

	result, err := series.Run(t.Context(), &TestData{})

	if err != expectedErr {
		t.Errorf("Expected error %v, got %v", expectedErr, err)
	}

	if result == nil {
		t.Error("Expected non-nil result even with error in series")
	}
}

// TestDeepNestedPipelines tests deeply nested pipeline structures
func TestDeepNestedPipelines(t *testing.T) {
	type TestData struct {
		Depth int
		Path  []string
	}

	createNestedStep := func(name string) wf.Step[TestData] {
		return wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Path = append(data.Path, name)
			data.Depth++
			return data, nil
		})
	}

	// Create deeply nested structure
	innerParallel := wf.Parallel(nil,
		wf.MergeTransform[TestData](mergo.WithTransformers(addInt{})),
		createNestedStep("p1"),
		createNestedStep("p2"),
	)

	innerSeries := wf.Sequential(nil,
		createNestedStep("s1"),
		innerParallel,
		createNestedStep("s2"),
	)

	outerPipeline := wf.NewPipeline[TestData]()
	outerPipeline.Steps = []wf.Step[TestData]{
		createNestedStep("start"),
		innerSeries,
		createNestedStep("end"),
	}

	result, err := outerPipeline.Run(context.Background(), &TestData{})
	if err != nil {
		t.Errorf("Expected no error for nested pipeline, got %v", err)
	}

	if result.Depth != 10 { // start, s1, p1+p2, s2, end
		t.Errorf("Expected depth 10, got %d", result.Depth)
	}

	expectedSteps := []string{"start", "s1", "s2", "end"}
	for _, step := range expectedSteps {
		found := false
		for _, pathStep := range result.Path {
			if pathStep == step {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected to find step %s in path %v", step, result.Path)
		}
	}
}

// TestMiddlewareErrorHandling tests middleware behavior with errors
func TestMiddlewareErrorHandling(t *testing.T) {
	type TestData struct {
		Messages []string
	}

	errorRecoveryMiddleware := func(next wf.Step[TestData]) wf.Step[TestData] {
		return &wf.MidFunc[TestData]{
			Name: "ErrorRecovery",
			Next: next,
			Fn: func(ctx context.Context, data *TestData) (*TestData, error) {
				result, err := next.Run(ctx, data)
				if err != nil {
					// Log error but continue
					data.Messages = append(data.Messages, fmt.Sprintf("recovered from: %v", err))
					return data, nil // Convert error to success
				}
				return result, nil
			},
		}
	}

	pipeline := wf.NewPipeline(errorRecoveryMiddleware)
	pipeline.Steps = []wf.Step[TestData]{
		wf.StepFunc[TestData](func(_ context.Context, _ *TestData) (*TestData, error) {
			return nil, errors.New("intentional error")
		}),
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			data.Messages = append(data.Messages, "final step")
			return data, nil
		}),
	}

	result, err := pipeline.Run(context.Background(), &TestData{})
	if err != nil {
		t.Errorf("Expected no error due to middleware recovery, got %v", err)
	}

	if len(result.Messages) != 2 {
		t.Errorf("Expected 2 messages, got %d: %v", len(result.Messages), result.Messages)
	}

	if result.Messages[0] != "recovered from: intentional error" {
		t.Errorf("Expected recovery message, got %s", result.Messages[0])
	}

	if result.Messages[1] != "final step" {
		t.Errorf("Expected final step message, got %s", result.Messages[1])
	}
}

// TestConcurrentAccess tests concurrent access to pipeline execution
func TestConcurrentAccess(t *testing.T) {
	type TestData struct {
		Value int
		ID    string
	}

	pipeline := wf.NewPipeline[TestData]()
	pipeline.Steps = []wf.Step[TestData]{
		wf.StepFunc[TestData](func(_ context.Context, data *TestData) (*TestData, error) {
			// Simulate some work
			time.Sleep(10 * time.Millisecond)
			data.Value++
			return data, nil
		}),
	}

	const numGoroutines = 10
	results := make(chan *TestData, numGoroutines)
	errors := make(chan error, numGoroutines)
	ctx := context.Background()
	// Run pipeline concurrently
	for i := range numGoroutines {
		go func(id int) {
			result, err := pipeline.Run(ctx, &TestData{
				Value: id,
				ID:    fmt.Sprintf("goroutine-%d", id),
			})
			if err != nil {
				errors <- err
			} else {
				results <- result
			}
		}(i)
	}

	// Collect results
	for i := 0; i < numGoroutines; i++ {
		select {
		case result := <-results:
			if result.Value == 0 {
				t.Errorf("Expected value to be incremented for %s", result.ID)
			}
		case err := <-errors:
			t.Errorf("Unexpected error in concurrent execution: %v", err)
		case <-time.After(5 * time.Second):
			t.Fatal("Timeout waiting for concurrent execution results")
		}
	}
}
