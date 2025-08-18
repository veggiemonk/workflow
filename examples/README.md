# Examples

This directory contains comprehensive examples demonstrating various aspects of the workflow engine.

## Available Examples

### 1. [Basic](./basic/)
A simple sequential pipeline demonstrating fundamental concepts:
- Creating pipelines
- Defining steps with `StepFunc`
- Data transformation
- Pipeline visualization

**Best for**: Learning the basics, getting started

### 2. [CI/CD](./cicd/)
A realistic CI/CD pipeline showcasing:
- Parallel quality checks (tests, linting, security)
- Conditional deployment logic
- Structured logging with middleware
- Real-world workflow patterns

**Best for**: Understanding practical applications, DevOps workflows

### 3. [Advanced](./advanced/)
A sophisticated data processing pipeline featuring:
- Custom middleware development
- Complex parallel processing with custom merge functions
- Dynamic conditional workflows
- Comprehensive metrics and error handling
- Data export and reporting

**Best for**: Advanced patterns, production-ready workflows

## Running Examples

Each example is self-contained with its own `main.go` and documentation:

```bash
# Basic example
cd basic && go run main.go

# CI/CD example  
cd cicd && go run main.go

# Advanced example
cd advanced && go run main.go
```

## Learning Path

1. **Start with Basic**: Understand core concepts
2. **Try CI/CD**: See real-world application patterns
3. **Explore Advanced**: Learn sophisticated patterns and customization

## Common Patterns Demonstrated

- **Sequential Execution**: Steps running one after another
- **Parallel Execution**: Independent tasks running concurrently
- **Conditional Logic**: Branching workflows with `Select`
- **Middleware Usage**: Cross-cutting concerns like logging and metrics
- **Error Handling**: Graceful failure management
- **Data Transformation**: Passing and modifying data through pipelines
- **Pipeline Visualization**: Understanding workflow structure

Each example builds upon the previous one, introducing more sophisticated concepts and patterns.
