# Architecture Documentation

## Overview

The workflow engine is built around a few core abstractions that provide flexibility and composability for building complex workflows.

## Core Abstractions

### Step[T]

The fundamental unit of work in the workflow engine.

```go
type Step[T any] interface {
    Run(context.Context, *T) (*T, error)
    fmt.Stringer
}
```

**Responsibilities:**
- Execute a single unit of work
- Transform input data `*T` to output data `*T`
- Handle errors gracefully
- Provide string representation for debugging

**Implementations:**
- `StepFunc[T]`: Function adapter for inline steps
- `Pipeline[T]`: Orchestrates multiple steps
- `series[T]`: Sequential execution of steps
- `parallel[T]`: Concurrent execution of steps
- `selector[T]`: Conditional execution

### Pipeline[T]

A step that orchestrates a sequence of other steps.

```go
type Pipeline[T any] struct {
    Steps      []Step[T]
    Middleware []Middleware[T]
}
```

**Features:**
- Sequential execution of steps
- Middleware application to all steps
- Error propagation
- Tree visualization

### Middleware[T]

Cross-cutting concerns that wrap step execution.

```go
type Middleware[T any] func(s Step[T]) Step[T]
```

**Common Use Cases:**
- Logging
- Metrics collection
- Error handling
- Authentication/authorization
- Caching
- Rate limiting

### Series and Parallel Composition

#### Series[T]
Sequential execution where each step receives the output of the previous step.

```go
func Series[T any](mid []Middleware[T], steps ...Step[T]) *series[T]
```

#### Parallel[T] 
Concurrent execution where all steps receive the same input, and results are merged.

```go
func Parallel[T any](mid []Middleware[T], merge MergeRequest[T], steps ...Step[T]) *parallel[T]
```

**Key Concepts:**
- Input copying for thread safety
- Error group for concurrent execution
- Custom merge functions for result aggregation

### Selector[T]

Conditional execution based on runtime conditions.

```go
func Select[T any](mid []Middleware[T], s Selector[T], ifStep, elseStep Step[T]) Step[T]
```

## Data Flow

### Input/Output Pattern
All steps follow the same pattern:
- Receive `*T` (pointer to data)
- Return `*T` (transformed data) and `error`
- Data flows through the pipeline, being transformed at each step

### Context Propagation
- All operations are context-aware
- Supports cancellation and timeouts
- Context values can be used for request scoping

### Error Handling
- Errors stop pipeline execution by default
- Middleware can intercept and handle errors
- Custom error handling strategies possible

## Execution Model

### Sequential Execution
```
Input → Step1 → Step2 → Step3 → Output
```

### Parallel Execution
```
Input → Copy1 → Step1 ↘
     → Copy2 → Step2 → Merge → Output
     → Copy3 → Step3 ↗
```

### Conditional Execution
```
Input → Condition → IfStep → Output
                 → ElseStep → Output
```

## Design Principles

### 1. Composability
Steps can be combined in various ways:
- Sequential chains
- Parallel fan-out/fan-in
- Conditional branching
- Nested compositions

### 2. Type Safety
- Generic types ensure compile-time safety
- No runtime type assertions
- Clear data contracts

### 3. Immutability Preference
- Steps should prefer creating new data over mutation
- Input copying in parallel execution
- Predictable data flow

### 4. Context Awareness
- All operations support cancellation
- Request scoping through context values
- Timeout and deadline support

### 5. Debuggability
- String representations for all components
- Tree visualization of pipeline structure
- Middleware for logging and tracing

## Extension Points

### Custom Steps
Implement the `Step[T]` interface:

```go
type MyStep struct {
    config MyConfig
}

func (s MyStep) Run(ctx context.Context, data *MyData) (*MyData, error) {
    // Custom logic here
    return transformedData, nil
}

func (s MyStep) String() string {
    return "MyStep"
}
```

### Custom Middleware
Create functions that wrap steps:

```go
func myMiddleware[T any]() Middleware[T] {
    return func(next Step[T]) Step[T] {
        return &MidFunc[T]{
            Name: "MyMiddleware",
            Next: next,
            Fn: func(ctx context.Context, data *T) (*T, error) {
                // Pre-processing
                result, err := next.Run(ctx, data)
                // Post-processing
                return result, err
            },
        }
    }
}
```

### Custom Merge Functions
For parallel execution:

```go
func myMerge[T any](ctx context.Context, base *T, results ...*T) (*T, error) {
    // Custom aggregation logic
    return aggregatedResult, nil
}
```

## Performance Considerations

### Memory Management
- Consider object pooling for high-frequency operations
- Be mindful of data copying in parallel operations
- Prefer streaming for large datasets

### Concurrency
- Parallel steps use goroutines and error groups
- Consider goroutine limits for large fan-out operations
- Use context for proper cancellation

### Error Handling
- Balance between fail-fast and error recovery
- Consider circuit breaker patterns for external dependencies
- Use middleware for centralized error handling

## Best Practices

1. **Keep Steps Small**: Single responsibility principle
2. **Use Middleware for Cross-Cutting Concerns**: Logging, metrics, etc.
3. **Design for Testability**: Steps should be easily unit testable
4. **Handle Errors Gracefully**: Consider partial failures and recovery
5. **Document Step Contracts**: Clear input/output expectations
6. **Use Context Properly**: Respect cancellation and timeouts
