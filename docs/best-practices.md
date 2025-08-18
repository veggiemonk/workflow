# Best Practices Guide

## Pipeline Design

### 1. Keep Steps Small and Focused

**✅ Good:**
```go
// Each step has a single responsibility
wf.StepFunc[Data](validateInput),
wf.StepFunc[Data](transformData),
wf.StepFunc[Data](saveResults),
```

**❌ Avoid:**
```go
// Monolithic step doing multiple things
wf.StepFunc[Data](validateTransformAndSave),
```

### 2. Use Descriptive Step Names

**✅ Good:**
```go
type ValidateUserInput struct{}
func (v ValidateUserInput) String() string { return "ValidateUserInput" }
```

**❌ Avoid:**
```go
type Step1 struct{}
func (s Step1) String() string { return "Step1" }
```

### 3. Design for Composability

**✅ Good:**
```go
// Reusable step that can be composed
func CreateValidationStep[T Validatable]() wf.Step[T] {
    return wf.StepFunc[T](func(ctx context.Context, data *T) (*T, error) {
        return data.Validate()
    })
}
```

## Error Handling

### 1. Fail Fast vs Graceful Degradation

**Fail Fast (Default):**
```go
wf.StepFunc[Data](func(ctx context.Context, data *Data) (*Data, error) {
    if err := criticalValidation(data); err != nil {
        return nil, err // Stop pipeline
    }
    return data, nil
})
```

**Graceful Degradation:**
```go
func errorRecoveryMiddleware[T any]() wf.Middleware[T] {
    return func(next wf.Step[T]) wf.Step[T] {
        return &wf.MidFunc[T]{
            Name: "ErrorRecovery",
            Next: next,
            Fn: func(ctx context.Context, data *T) (*T, error) {
                result, err := next.Run(ctx, data)
                if err != nil {
                    // Log error but continue pipeline
                    slog.Error("Step failed, continuing", "error", err)
                    return data, nil // Return original data
                }
                return result, nil
            },
        }
    }
}
```

### 2. Use Typed Errors

**✅ Good:**
```go
type ValidationError struct {
    Field   string
    Message string
}

func (e ValidationError) Error() string {
    return fmt.Sprintf("validation failed for %s: %s", e.Field, e.Message)
}
```

### 3. Collect and Handle Multiple Errors

**✅ Good:**
```go
type ProcessingResult struct {
    Data   []ProcessedItem
    Errors []error
}

func processItems(ctx context.Context, data *ProcessingResult) (*ProcessingResult, error) {
    for _, item := range data.Data {
        if err := processItem(item); err != nil {
            data.Errors = append(data.Errors, err)
            continue // Process remaining items
        }
    }
    
    if len(data.Errors) > 0 {
        return data, fmt.Errorf("processing completed with %d errors", len(data.Errors))
    }
    
    return data, nil
}
```

## Data Management

### 1. Prefer Immutability

**✅ Good:**
```go
wf.StepFunc[Data](func(ctx context.Context, data *Data) (*Data, error) {
    newData := *data // Copy struct
    newData.ProcessedAt = time.Now()
    newData.Items = append([]Item{}, data.Items...) // Copy slice
    return &newData, nil
})
```

**❌ Avoid:**
```go
wf.StepFunc[Data](func(ctx context.Context, data *Data) (*Data, error) {
    data.ProcessedAt = time.Now() // Mutating input
    return data, nil
})
```

### 2. Handle Large Data Sets

**For streaming data:**
```go
type StreamProcessor struct {
    batchSize int
}

func (sp StreamProcessor) Run(ctx context.Context, data *StreamData) (*StreamData, error) {
    for i := 0; i < len(data.Items); i += sp.batchSize {
        end := i + sp.batchSize
        if end > len(data.Items) {
            end = len(data.Items)
        }
        
        batch := data.Items[i:end]
        if err := processBatch(ctx, batch); err != nil {
            return nil, err
        }
        
        // Check for cancellation
        select {
        case <-ctx.Done():
            return nil, ctx.Err()
        default:
        }
    }
    
    return data, nil
}
```

### 3. Use Deep Copy for Complex Data

```go
import "github.com/mitchellh/copystructure"

func deepCopyStep[T any](ctx context.Context, data *T) (*T, error) {
    copied, err := copystructure.Copy(data)
    if err != nil {
        return nil, err
    }
    return copied.(*T), nil
}
```

## Parallel Processing

### 1. Safe Parallel Data Handling

**✅ Good:**
```go
func safeParallelMerge[T any](ctx context.Context, base *T, results ...*T) (*T, error) {
    var mu sync.Mutex
    
    for _, result := range results {
        mu.Lock()
        // Thread-safe merging
        if err := mergeResult(base, result); err != nil {
            mu.Unlock()
            return nil, err
        }
        mu.Unlock()
    }
    
    return base, nil
}
```

### 2. Control Parallel Execution

```go
// Limit concurrent operations
func createLimitedParallel[T any](maxConcurrency int, steps ...wf.Step[T]) wf.Step[T] {
    semaphore := make(chan struct{}, maxConcurrency)
    
    return wf.Parallel(nil, wf.Merge[T], 
        wrapWithSemaphore(semaphore, steps...)...)
}

func wrapWithSemaphore[T any](sem chan struct{}, steps ...wf.Step[T]) []wf.Step[T] {
    wrapped := make([]wf.Step[T], len(steps))
    for i, step := range steps {
        wrapped[i] = &semaphoreStep[T]{step: step, sem: sem}
    }
    return wrapped
}
```

## Middleware Patterns

### 1. Composable Middleware

**✅ Good:**
```go
// Middleware that can be configured
func retryMiddleware[T any](maxRetries int, backoff time.Duration) wf.Middleware[T] {
    return func(next wf.Step[T]) wf.Step[T] {
        return &wf.MidFunc[T]{
            Name: fmt.Sprintf("Retry(%d)", maxRetries),
            Next: next,
            Fn: func(ctx context.Context, data *T) (*T, error) {
                var lastErr error
                for i := 0; i <= maxRetries; i++ {
                    result, err := next.Run(ctx, data)
                    if err == nil {
                        return result, nil
                    }
                    lastErr = err
                    
                    if i < maxRetries {
                        select {
                        case <-time.After(backoff):
                        case <-ctx.Done():
                            return nil, ctx.Err()
                        }
                    }
                }
                return nil, lastErr
            },
        }
    }
}
```

### 2. Conditional Middleware

```go
func conditionalMiddleware[T any](condition func(*T) bool, mid wf.Middleware[T]) wf.Middleware[T] {
    return func(next wf.Step[T]) wf.Step[T] {
        return &wf.MidFunc[T]{
            Name: "Conditional",
            Next: next,
            Fn: func(ctx context.Context, data *T) (*T, error) {
                if condition(data) {
                    return mid(next).Run(ctx, data)
                }
                return next.Run(ctx, data)
            },
        }
    }
}
```

## Testing Strategies

### 1. Unit Test Individual Steps

```go
func TestValidationStep(t *testing.T) {
    step := CreateValidationStep()
    
    tests := []struct {
        name    string
        input   *Data
        wantErr bool
    }{
        {"valid data", &Data{Value: "valid"}, false},
        {"invalid data", &Data{Value: ""}, true},
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            result, err := step.Run(context.Background(), tt.input)
            if (err != nil) != tt.wantErr {
                t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
            }
            if !tt.wantErr && result == nil {
                t.Error("Expected result but got nil")
            }
        })
    }
}
```

### 2. Integration Test Pipelines

```go
func TestPipelineIntegration(t *testing.T) {
    pipeline := wf.NewPipeline[Data]()
    pipeline.Steps = []wf.Step[Data]{
        CreateValidationStep(),
        CreateTransformStep(),
        CreateSaveStep(),
    }
    
    input := &Data{Value: "test"}
    result, err := pipeline.Run(context.Background(), input)
    
    assert.NoError(t, err)
    assert.NotNil(t, result)
    assert.True(t, result.Processed)
}
```

### 3. Test Middleware

```go
func TestLoggingMiddleware(t *testing.T) {
    var buf bytes.Buffer
    logger := slog.New(slog.NewTextHandler(&buf, nil))
    
    mid := LoggerMiddleware[Data](logger)
    step := wf.StepFunc[Data](func(ctx context.Context, d *Data) (*Data, error) {
        return d, nil
    })
    
    wrappedStep := mid(step)
    _, err := wrappedStep.Run(context.Background(), &Data{})
    
    assert.NoError(t, err)
    assert.Contains(t, buf.String(), "start")
    assert.Contains(t, buf.String(), "done")
}
```

## Performance Optimization

### 1. Use Object Pooling for High-Frequency Operations

```go
var dataPool = sync.Pool{
    New: func() interface{} {
        return &Data{}
    },
}

func pooledStep(ctx context.Context, data *Data) (*Data, error) {
    result := dataPool.Get().(*Data)
    defer dataPool.Put(result)
    
    // Process using pooled object
    *result = *data
    result.Processed = true
    
    // Return copy, not pooled object
    final := *result
    return &final, nil
}
```

### 2. Optimize for Large Datasets

```go
// Stream processing for large datasets
type StreamStep[T any] struct {
    processor func(context.Context, T) (T, error)
    batchSize int
}

func (s StreamStep[T]) Run(ctx context.Context, data *StreamData[T]) (*StreamData[T], error) {
    results := make([]T, 0, len(data.Items))
    
    for i := 0; i < len(data.Items); i += s.batchSize {
        batch := data.Items[i:min(i+s.batchSize, len(data.Items))]
        
        for _, item := range batch {
            processed, err := s.processor(ctx, item)
            if err != nil {
                return nil, err
            }
            results = append(results, processed)
        }
        
        // Yield control periodically
        if i%1000 == 0 {
            select {
            case <-ctx.Done():
                return nil, ctx.Err()
            default:
            }
        }
    }
    
    return &StreamData[T]{Items: results}, nil
}
```

## Monitoring and Observability

### 1. Comprehensive Metrics Middleware

```go
func metricsMiddleware[T any](metrics *Metrics) wf.Middleware[T] {
    return func(next wf.Step[T]) wf.Step[T] {
        return &wf.MidFunc[T]{
            Name: "Metrics",
            Next: next,
            Fn: func(ctx context.Context, data *T) (*T, error) {
                start := time.Now()
                stepName := wf.Name(next)
                
                result, err := next.Run(ctx, data)
                
                duration := time.Since(start)
                metrics.RecordStepDuration(stepName, duration)
                
                if err != nil {
                    metrics.IncrementStepErrors(stepName)
                } else {
                    metrics.IncrementStepSuccess(stepName)
                }
                
                return result, err
            },
        }
    }
}
```

### 2. Distributed Tracing

```go
import "go.opentelemetry.io/otel/trace"

func tracingMiddleware[T any](tracer trace.Tracer) wf.Middleware[T] {
    return func(next wf.Step[T]) wf.Step[T] {
        return &wf.MidFunc[T]{
            Name: "Tracing",
            Next: next,
            Fn: func(ctx context.Context, data *T) (*T, error) {
                stepName := wf.Name(next)
                ctx, span := tracer.Start(ctx, stepName)
                defer span.End()
                
                result, err := next.Run(ctx, data)
                if err != nil {
                    span.RecordError(err)
                    span.SetStatus(codes.Error, err.Error())
                }
                
                return result, err
            },
        }
    }
}
```

## Common Anti-Patterns to Avoid

### 1. ❌ Shared Mutable State

```go
// DON'T DO THIS
var globalCounter int

wf.StepFunc[Data](func(ctx context.Context, data *Data) (*Data, error) {
    globalCounter++ // Race condition in parallel execution
    return data, nil
})
```

### 2. ❌ Blocking Operations Without Context

```go
// DON'T DO THIS
wf.StepFunc[Data](func(ctx context.Context, data *Data) (*Data, error) {
    time.Sleep(5 * time.Second) // Ignores context cancellation
    return data, nil
})

// DO THIS INSTEAD
wf.StepFunc[Data](func(ctx context.Context, data *Data) (*Data, error) {
    select {
    case <-time.After(5 * time.Second):
        return data, nil
    case <-ctx.Done():
        return nil, ctx.Err()
    }
})
```

### 3. ❌ Deep Nested Pipelines

```go
// DON'T DO THIS - Hard to understand and debug
wf.Series(nil,
    wf.Parallel(nil, merge,
        wf.Series(nil,
            wf.Parallel(nil, merge,
                step1, step2),
            step3),
        step4),
    step5)

// DO THIS INSTEAD - Break into logical components
validation := wf.Parallel(nil, merge, validateInput, validateAuth)
processing := wf.Series(nil, processData, enrichData)
pipeline := wf.Series(nil, validation, processing, saveResults)
```
