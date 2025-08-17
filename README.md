# workflow

A tiny, flexible, and extensible workflow engine in Go, designed to be generic and suitable for various applications, including CI/CD pipelines.

[![Go Reference](https://pkg.go.dev/badge/github.com/veggiemonk/workflow.svg)](https://pkg.go.dev/github.com/veggiemonk/workflow)

## Features

- **Pipelines**: Define a sequence of steps to be executed.
- **Sequential and Parallel Execution**: Run steps one after another or concurrently.
- **Conditional Logic**: Use selectors to execute different steps based on conditions.
- **Middleware**: Intercept and modify the execution of steps for cross-cutting concerns like logging, metrics, or error handling.
- **Generic**: Works with any data type, providing type safety.
- **Context-aware**: Supports cancellation and deadlines through `context.Context`.
- **Extensible**: Easily create your own custom steps by implementing the `Step` interface.
- **Debuggable**: Provides a `String()` method to print the structure of the pipeline.

## Installation

```bash
go get github.com/veggiemonk/workflow
```

## Usage

Here is a simple example of how to use the `workflow` package to define and run a pipeline.

```go
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"

	wf "github.com/veggiemonk/workflow"
)

// Result is the data structure that will be passed through the workflow.
type Result struct {
	Messages []string
	State    struct {
		Counter int
	}
}

func main() {
	// Create middleware to log before and after every step.
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
	_, err := p.Run(context.Background(), &Result{})
	if err != nil {
		fmt.Println("Error:", err)
	}

	// Print the pipeline structure.
	fmt.Println(p)

	// Print buffer from the logs.
	fmt.Print(buf.String())
}

func logMiddleware[T any](l io.Writer) wf.Middleware[T] {
	return func(next wf.Step[T]) wf.Step[T] {
		return &wf.MidFunc[T]{
			Name: "Logger",
			Next: next,
			Fn: func(ctx context.Context, res *T) (*T, error) {
				name := wf.Name(next)
				if name != "MidFunc" {
					fmt.Fprintf(l, "start: name=%s ", name)
				}
				resp, err := next.Run(ctx, res)

				if name != "MidFunc" {
					fmt.Fprintf(l, "done: name=%s ", name)
				}
				return resp, err
			},
		}
	}
}
```

## Core Concepts

- **`Step[T]`**: The basic unit of work in a workflow. It's an interface with a single method, `Run`.
- **`Pipeline[T]`**: A series of steps that are executed in order.
- **`Series[T]`**: A step that executes a list of other steps sequentially.
- **`Parallel[T]`**: A step that executes a list of other steps in parallel and merges their results.
- **`Select[T]`**: A step that executes one of two other steps based on a selector function.
- **`Middleware[T]`**: A function that wraps a step to add functionality, such as logging or error handling.

## Contributing

Contributions are welcome! Please feel free to submit a pull request or open an issue.

## License

This project is licensed under the Apache License - see the [LICENSE](LICENSE) file for details.
