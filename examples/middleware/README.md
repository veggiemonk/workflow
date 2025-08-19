# Middleware Example

This example demonstrates the usage of various middleware components in the workflow engine, including:

- **Retry Middleware**: Automatically retries failed steps with configurable backoff
- **Timeout Middleware**: Enforces execution timeouts on steps
- **Circuit Breaker Middleware**: Prevents cascading failures by temporarily blocking requests
- **Logger Middleware**: Logs step execution for observability
- **UUID Middleware**: Assigns unique identifiers to step executions

## Features Demonstrated

### 1. Retry Middleware
- Configurable retry attempts with exponential backoff
- Custom retry conditions based on error types
- Proper context cancellation handling

### 2. Timeout Middleware
- Per-step timeout enforcement
- Graceful handling of long-running operations
- Context-aware cancellation

### 3. Circuit Breaker Middleware
- Failure threshold configuration
- Automatic recovery after timeout periods
- Half-open state for testing service recovery

### 4. Middleware Composition
- Combining multiple middleware in a pipeline
- Order-dependent middleware application
- Integration with existing logging and UUID middleware

## Running the Example

```bash
go run main.go
```

## Example Output

The example processes several orders through different scenarios:

1. **Successful Processing**: Shows normal workflow execution
2. **Retry Scenarios**: Demonstrates retry behavior with failing services
3. **Circuit Breaker**: Shows circuit breaker protection during service outages
4. **Timeout Protection**: Demonstrates timeout enforcement

## Key Concepts

### Middleware Ordering
Middleware is applied in reverse order (last middleware wraps first):
```go
pipeline := NewPipeline[OrderData](
    retryMiddleware,      // Applied last (outermost)
    timeoutMiddleware,    // Applied second
    loggerMiddleware,     // Applied first (innermost)
)
```

### Error Handling
Each middleware can define custom error handling:
- Retry middleware can filter which errors to retry
- Circuit breaker can define which errors should trip the circuit
- Timeout middleware wraps context deadline errors

### Configuration
All middleware support detailed configuration:
```go
retryConfig := RetryConfig{
    MaxAttempts:       3,
    InitialDelay:      50 * time.Millisecond,
    MaxDelay:          500 * time.Millisecond,
    BackoffMultiplier: 2.0,
    ShouldRetry: func(err error) bool {
        return err.Error() == "payment service temporarily unavailable"
    },
}
```

## Use Cases

This middleware pattern is useful for:
- **Microservice Communication**: Handling network failures and timeouts
- **External API Integration**: Retrying failed API calls with backoff
- **Database Operations**: Circuit breaking for database connection issues
- **Batch Processing**: Timeout protection for long-running operations
- **Observability**: Logging and tracing workflow execution
