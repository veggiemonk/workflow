# Component: Core Workflow Engine

## Purpose

This directory contains the core logic for the `workflow` engine, a flexible and extensible library for defining and executing complex workflows in Go. It provides the fundamental building blocks for creating pipelines with sequential, parallel, and conditional execution of tasks.

## Key Files

-   **`workflow.go`**: The main file that defines the core interfaces and structs, including `Step[T]`, `Pipeline[T]`, `Sequential[T]`, and `Parallel[T]`.
-   **`task.go`**: Defines the `Task[T]` struct, which is a named wrapper around a `Step[T]`, allowing for more structured and identifiable workflow units.
-   **`middleware.go`**: Provides a collection of built-in middleware for cross-cutting concerns like logging, retries, timeouts, and circuit breakers.
-   **`builder.go`**: Contains the `Builder[T]`, which is responsible for constructing a `Pipeline[T]` from a serialized specification (e.g., a JSON file).
-   **`registry.go`**: Implements the `Registry[T]`, a repository for named tasks, selectors, and merge functions, which is crucial for deserializing pipelines.
-   **`go.mod`**: The Go module file that defines the module path (`github.com/veggiemonk/workflow`) and its dependencies.
-   **`Makefile`**: Contains various development commands for testing, linting, building, and running examples.

## Dependencies

### External

-   **`dario.cat/mergo`**: Used for merging data structures, particularly in parallel steps.
-   **`github.com/google/uuid`**: Used for generating unique IDs, for instance in the UUID middleware.
-   **`golang.org/x/sync`**: Provides synchronization primitives, such as `errgroup`, for managing concurrent operations in parallel steps.
-   **`github.com/google/go-cmp`**: Used for comparing complex data structures in tests.
-   **`github.com/ccoveille/go-safecast`**: Used for safe type casting.


### Internal

This is the core component, so it has no internal dependencies on other components within this codebase.

## Interactions

The core workflow engine is the central component of this application. All other components, such as the `examples`, depend on it to demonstrate its functionality. The `docs` component provides documentation for this engine. The `.github` component uses the `Makefile` to run tests and CI/CD pipelines for the engine.

## Important Notes

-   **Generics**: The engine makes extensive use of Go generics (`[T any]`) to provide type safety for the data flowing through the workflows.
-   **Extensibility**: The engine is designed to be highly extensible. Users can create their own custom steps by implementing the `Step[T]` interface and custom middleware to add functionality to their pipelines.
-   **Immutability**: The engine encourages immutability by passing pointers to data through the workflow and providing mechanisms for safe data copying in parallel execution.
