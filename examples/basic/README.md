# Basic Workflow Example

This example demonstrates the fundamental usage of the workflow engine with a simple sequential pipeline.

## What it does

1. **Initialize**: Sets up the pipeline data
2. **Transform**: Processes the input string
3. **Count**: Increments a counter
4. **Finalize**: Marks the pipeline as complete

## Running the example

```bash
cd examples/basic
go run main.go
```

## Expected output

The example will show:
- Step-by-step execution progress
- Final results including transformed data
- Pipeline structure visualization

## Key concepts demonstrated

- Creating a basic pipeline
- Using `StepFunc` to define inline steps
- Sequential step execution
- Data transformation through steps
- Pipeline structure inspection
