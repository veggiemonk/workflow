package workflow_test

import (
	"context"
	"fmt"

	wf "github.com/veggiemonk/workflow"
)

// Example demonstrates how to use the workflow package to define and run a pipeline.
func Example() {
	// Create a new pipeline.
	p := wf.NewPipeline[Result]()

	// Define the steps of the pipeline.
	p.Steps = []wf.Step[Result]{
		// Step 1: A simple function that modifies the result.
		wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
			r.Messages = append(r.Messages, "starting pipeline")
			return r, nil
		}),
		// Step 2: A series of steps that run sequentially.
		wf.Series(nil,
			// Step 2a: A simple function.
			wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
				r.Messages = append(r.Messages, "in series")
				return r, nil
			}),
			// Step 2b: A parallel execution of steps.
			wf.Parallel(nil,
				// The merge function combines the results of the parallel steps.
				wf.Merge[Result],
				// Parallel task 1.
				wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
					r.State.Counter++
					r.Messages = append(r.Messages, "parallel task 1")
					return r, nil
				}),
				// Parallel task 2.
				wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
					r.State.Counter++
					r.Messages = append(r.Messages, "parallel task 2")
					return r, nil
				}),
			),
		),
		// Step 3: A final step.
		wf.StepFunc[Result](func(ctx context.Context, r *Result) (*Result, error) {
			r.Messages = append(r.Messages, "pipeline finished")
			return r, nil
		}),
	}

	// Print the pipeline structure.
	fmt.Println(p)

	// Run the pipeline.
	result, err := p.Run(context.Background(), &Result{})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Print the final result.
	fmt.Println("Final Result:", result)
	// Output: Pipeline[Result]
	// ├── StepFunc[workflow_test.Result]
	// ├── series[Result]
	// │   ├── StepFunc[workflow_test.Result]
	// │   └── parallel[Result]
	// │       ├── StepFunc[workflow_test.Result]
	// │       └── StepFunc[workflow_test.Result]
	// └── StepFunc[workflow_test.Result]
	// 
	// Final Result: Result{State: workflow_test.State{Counter:1}, Messages: [starting pipeline in series pipeline finished]}
}
