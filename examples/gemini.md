# Component: Examples

## Purpose

This directory contains a collection of example applications that demonstrate how to use the `workflow` engine. Each example is a self-contained Go module that showcases a specific set of features, from basic usage to advanced patterns.

## Key Files and Directories

-   **`README.md`**: The main entry point for understanding the examples. It provides a summary of each example, instructions on how to run them, and a suggested learning path.
-   **`basic/`**: A simple example that demonstrates the fundamental concepts of the workflow engine, such as creating a pipeline and defining steps.
-   **`cicd/`**: A more realistic example that simulates a CI/CD pipeline, showcasing parallel execution, conditional logic, and middleware.
-   **`advanced/`**: A sophisticated example that demonstrates advanced features like custom middleware, complex data processing, and custom merge functions.
-   **`middleware/`**: An example that focuses on the usage of the built-in middleware, such as `Retry`, `Timeout`, and `CircuitBreaker`.
-   **`serialization/`**: An example that demonstrates how to serialize a pipeline to a JSON file and deserialize it back into a runnable pipeline using the `Builder` and `Registry`.

## Dependencies

### External

The examples have the same external dependencies as the core engine, which are defined in their respective `go.mod` files.

### Internal

All examples in this directory depend on the core workflow engine located in the root directory of the project. Each example's `go.mod` file uses a `replace` directive to point to the local copy of the engine.

## Interactions

The examples are standalone applications and do not interact with each other. They are designed to be run independently to demonstrate specific features of the workflow engine.

## Important Notes

-   **Self-Contained Modules**: Each example is a self-contained Go module with its own `go.mod` and `go.sum` files. This allows them to be built and run independently of each other.
-   **Learning Resource**: These examples are a key learning resource for understanding how to use the `workflow` engine. It is recommended to start with the `basic` example and progress to the more advanced ones.
