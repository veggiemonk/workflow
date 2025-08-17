// Package workflow provides a flexible and extensible engine for defining and executing complex workflows.
// It allows you to create pipelines of steps that can be run sequentially or in parallel.
// The package is designed to be generic and can be used with any data type.
//
// # Key Features
//
//   - **Pipelines**: Define a sequence of steps to be executed.
//   - **Sequential and Parallel Execution**: Run steps one after another or concurrently.
//   - **Middleware**: Intercept and modify the execution of steps.
//   - **Generic**: Works with any data type.
//   - **Context-aware**: Supports cancellation and deadlines through context.
//
// # Core Concepts
//
//   - **Step**: The basic unit of work in a workflow. It's an interface with a single method, `Run`.
//   - **Pipeline**: A series of steps that are executed in order.
//   - **Series**: A step that executes a list of other steps sequentially.
//   - **Parallel**: A step that executes a list of other steps in parallel and merges their results.
//   - **Middleware**: A function that wraps a step to add functionality, such as logging or error handling.
package workflow
