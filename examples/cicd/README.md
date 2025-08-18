# CI/CD Pipeline Example

This example demonstrates how to build a complete CI/CD pipeline using the workflow engine, showcasing parallel execution, conditional logic, and comprehensive logging.

## What it does

This CI/CD pipeline includes:

1. **Code Checkout**: Simulates pulling source code
2. **Parallel Quality Checks**: 
   - Unit tests
   - Code linting
   - Security scanning
3. **Conditional Build**: Only builds if quality checks pass
4. **Conditional Deployment**: 
   - Deploy to staging
   - Run smoke tests
   - Deploy to production (if staging succeeds)
5. **Reporting**: Generate final pipeline report

## Features demonstrated

- **Parallel execution**: Quality checks run concurrently
- **Conditional logic**: Using `Select` for branching workflows
- **Middleware**: UUID tracking and structured logging
- **Series workflows**: Multi-step deployment process
- **Error handling**: Graceful failure management
- **Real-world simulation**: Timing and realistic step names

## Running the example

```bash
cd examples/cicd
go run main.go
```

## Expected output

The pipeline will show:
- Real-time step execution with emojis
- Parallel execution of quality checks
- Conditional branching based on results
- Structured logging with UUIDs
- Final pipeline visualization
- Execution timing and results

## Key concepts

- **Complex workflows**: Multi-stage pipeline with dependencies
- **Conditional execution**: Steps that run based on previous results
- **Parallel processing**: Independent tasks running concurrently
- **Error propagation**: How failures affect downstream steps
- **Middleware composition**: Combining logging and UUID tracking
