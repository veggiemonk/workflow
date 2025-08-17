package workflow_test

import (
	"bytes"
	"context"
	"fmt"
	"io"

	wf "github.com/veggiemonk/workflow"
)

// Example demonstrates how to use the workflow package to define and run a pipeline.
func Example() {
	// create middleware to log before and after every steps.
	var buf bytes.Buffer
	mid := logMiddleware[Result](&buf)

	// Create a new pipeline.
	p := wf.NewPipeline(mid)

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

	// Run the pipeline.
	result, err := p.Run(context.Background(), &Result{})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Print the pipeline structure.
	fmt.Println(p)

	// Print the final result.
	fmt.Println(result)

	// Print buffer from the logs.
	fmt.Print(buf.String())

	// Output:
	// Pipeline[Result]
	// ├── Logger(StepFunc[workflow_test.Result])
	// ├── Logger(series[Result]
	// │   ├── StepFunc[workflow_test.Result]
	// │   └── parallel[Result]
	// │       ├── StepFunc[workflow_test.Result]
	// │       └── StepFunc[workflow_test.Result])
	// └── Logger(StepFunc[workflow_test.Result])
	//
	// Result{State: workflow_test.State{Counter:1}, Messages: [starting pipeline in series pipeline finished]}
	// start: name=StepFunc[Result]
	// done: name=StepFunc[Result]
	// start: name=series[Result]
	// done: name=series[Result]
	// start: name=StepFunc[Result]
	// done: name=StepFunc[Result]
}

func logMiddleware[T any](l io.Writer) wf.Middleware[T] {
	return func(next wf.Step[T]) wf.Step[T] {
		return &wf.MidFunc[T]{
			Name: "Logger",
			Next: next,
			Fn: func(ctx context.Context, res *T) (*T, error) {
				name := wf.Name(next)
				if name != "MidFunc" {
					fmt.Fprintf(l, "start: name=%s\n", name)
				}
				resp, err := next.Run(ctx, res)

				if name != "MidFunc" {
					fmt.Fprintf(l, "done: name=%s\n", name)
				}
				return resp, err
			},
		}
	}
}
