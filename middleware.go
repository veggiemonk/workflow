package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// contextKey is a custom type for context keys to avoid collisions.
type contextKey string

// StepUUIDKey is the context key for storing step UUIDs.
const StepUUIDKey contextKey = "step_uuid"

// UUIDMiddleware returns a middleware that assigns a unique UUID to each step execution.
// The UUID is stored in the context with the key StepUUIDKey and can be retrieved
// using ctx.Value(StepUUIDKey).(string).
func UUIDMiddleware[T any]() Middleware[T] {
	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "UUID",
			Next: next,
			Fn: func(ctx context.Context, req *T) (*T, error) {
				stepUUID := uuid.New().String()
				ctx = context.WithValue(ctx, StepUUIDKey, stepUUID)
				return next.Run(ctx, req)
			},
		}
	}
}

// LoggerMiddleware returns a middleware that logs step execution using the provided slog.Logger.
func LoggerMiddleware[T any](l *slog.Logger) Middleware[T] {
	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "Logger",
			Next: next,
			Fn: func(ctx context.Context, res *T) (*T, error) {
				start := time.Now()
				name := Name(next)
				if name != "MidFunc" {
					l.Info("start", "Type", name, "STEP", next)
				}
				resp, err := next.Run(ctx, res)

				if name != "MidFunc" {
					l.Info("done", "Type", name, "duration", time.Since(start),
						"Result", fmt.Sprintf("%v", resp))
				}
				return resp, err
			},
		}
	}
}

// RetryConfig defines the configuration for retry middleware.
type RetryConfig struct {
	// MaxAttempts is the maximum number of attempts (including the initial attempt).
	// Must be at least 1. Default is 3.
	MaxAttempts int

	// InitialDelay is the delay before the first retry.
	// Default is 100ms.
	InitialDelay time.Duration

	// MaxDelay is the maximum delay between retries.
	// Default is 5 seconds.
	MaxDelay time.Duration

	// BackoffMultiplier is the multiplier for exponential backoff.
	// Default is 2.0.
	BackoffMultiplier float64

	// ShouldRetry is a function that determines whether an error should trigger a retry.
	// If nil, all errors will trigger retries.
	ShouldRetry func(error) bool
}

// DefaultRetryConfig returns a retry configuration with sensible defaults.
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      100 * time.Millisecond,
		MaxDelay:          5 * time.Second,
		BackoffMultiplier: 2.0,
		ShouldRetry:       nil, // retry all errors
	}
}

// RetryMiddleware returns a middleware that retries failed step executions
// according to the provided configuration.
func RetryMiddleware[T any](config RetryConfig) Middleware[T] {
	// Set defaults if not specified
	if config.MaxAttempts <= 0 {
		config.MaxAttempts = 3
	}
	if config.InitialDelay <= 0 {
		config.InitialDelay = 100 * time.Millisecond
	}
	if config.MaxDelay <= 0 {
		config.MaxDelay = 5 * time.Second
	}
	if config.BackoffMultiplier <= 0 {
		config.BackoffMultiplier = 2.0
	}

	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "Retry",
			Next: next,
			Fn: func(ctx context.Context, req *T) (*T, error) {
				var lastErr error
				delay := config.InitialDelay

				for attempt := 1; attempt <= config.MaxAttempts; attempt++ {
					// Check if context is cancelled before attempting
					if err := ctx.Err(); err != nil {
						return nil, err
					}

					resp, err := next.Run(ctx, req)
					if err == nil {
						return resp, nil
					}

					lastErr = err

					// Check if we should retry this error
					if config.ShouldRetry != nil && !config.ShouldRetry(err) {
						return nil, err
					}

					// Don't sleep after the last attempt
					if attempt < config.MaxAttempts {
						// Create a timer for the delay
						timer := time.NewTimer(delay)
						select {
						case <-ctx.Done():
							timer.Stop()
							return nil, ctx.Err()
						case <-timer.C:
							// Continue to next attempt
						}

						// Calculate next delay with exponential backoff
						nextDelay := time.Duration(float64(delay) * config.BackoffMultiplier)
						if nextDelay > config.MaxDelay {
							delay = config.MaxDelay
						} else {
							delay = nextDelay
						}
					}
				}

				// All attempts failed, return the last error wrapped with attempt info
				return nil, fmt.Errorf("step failed after %d attempts: %w", config.MaxAttempts, lastErr)
			},
		}
	}
}

// TimeoutMiddleware returns a middleware that enforces a timeout on step execution.
// If the step doesn't complete within the specified duration, it returns a context
// deadline exceeded error.
func TimeoutMiddleware[T any](timeout time.Duration) Middleware[T] {
	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "Timeout",
			Next: next,
			Fn: func(ctx context.Context, req *T) (*T, error) {
				// Create a new context with timeout
				timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
				defer cancel()

				// Channel to receive the result
				type result struct {
					resp *T
					err  error
				}
				resultCh := make(chan result, 1)

				// Run the step in a goroutine
				go func() {
					resp, err := next.Run(timeoutCtx, req)
					resultCh <- result{resp: resp, err: err}
				}()

				// Wait for either completion or timeout
				select {
				case res := <-resultCh:
					return res.resp, res.err
				case <-timeoutCtx.Done():
					return nil, fmt.Errorf("step timed out after %v: %w", timeout, timeoutCtx.Err())
				}
			},
		}
	}
}

// CircuitBreakerState represents the state of a circuit breaker.
type CircuitBreakerState int

const (
	// CircuitClosed indicates the circuit is closed and operations are allowed.
	CircuitClosed CircuitBreakerState = iota
	// CircuitOpen indicates the circuit is open and operations are blocked.
	CircuitOpen
	// CircuitHalfOpen indicates the circuit is half-open and allows limited operations.
	CircuitHalfOpen
)

// CircuitBreakerConfig defines the configuration for circuit breaker middleware.
type CircuitBreakerConfig struct {
	// FailureThreshold is the number of consecutive failures that will open the circuit.
	// Default is 5.
	FailureThreshold int

	// OpenTimeout is how long the circuit stays open before transitioning to half-open.
	// Default is 60 seconds.
	OpenTimeout time.Duration

	// ShouldTrip is a function that determines whether an error should count as a failure.
	// If nil, all errors count as failures.
	ShouldTrip func(error) bool
}

// DefaultCircuitBreakerConfig returns a circuit breaker configuration with sensible defaults.
func DefaultCircuitBreakerConfig() CircuitBreakerConfig {
	return CircuitBreakerConfig{
		FailureThreshold: 5,
		OpenTimeout:      60 * time.Second,
		ShouldTrip:       nil, // all errors count as failures
	}
}

// CircuitBreakerMiddleware returns a middleware that implements the circuit breaker pattern.
// It prevents cascading failures by temporarily blocking requests when a step consistently fails.
func CircuitBreakerMiddleware[T any](config CircuitBreakerConfig) Middleware[T] {
	// Set defaults if not specified
	if config.FailureThreshold <= 0 {
		config.FailureThreshold = 5
	}
	if config.OpenTimeout <= 0 {
		config.OpenTimeout = 60 * time.Second
	}

	// Circuit breaker state
	var (
		state           = CircuitClosed
		failureCount    = 0
		lastFailureTime time.Time
		mu              sync.RWMutex
	)

	return func(next Step[T]) Step[T] {
		return &MidFunc[T]{
			Name: "CircuitBreaker",
			Next: next,
			Fn: func(ctx context.Context, req *T) (*T, error) {
				mu.RLock()
				currentState := state
				lastFailure := lastFailureTime
				mu.RUnlock()

				// Check if we should allow the request
				switch currentState {
				case CircuitOpen:
					if time.Since(lastFailure) > config.OpenTimeout {
						// Transition to half-open
						mu.Lock()
						if state == CircuitOpen { // Double-check under lock
							state = CircuitHalfOpen
						}
						mu.Unlock()
					} else {
						return nil, errors.New("circuit breaker is open")
					}
				case CircuitHalfOpen:
					// Allow one request to test if service is back
				case CircuitClosed:
					// Normal operation
				}

				// Execute the step
				resp, err := next.Run(ctx, req)

				mu.Lock()
				defer mu.Unlock()

				if err != nil && (config.ShouldTrip == nil || config.ShouldTrip(err)) {
					failureCount++
					lastFailureTime = time.Now()

					if state == CircuitHalfOpen {
						// Failed during half-open, go back to open
						state = CircuitOpen
					} else if failureCount >= config.FailureThreshold {
						// Too many failures, open the circuit
						state = CircuitOpen
					}

					return nil, err
				}

				// Success - reset failure count and close circuit if it was half-open
				if state == CircuitHalfOpen {
					state = CircuitClosed
				}
				failureCount = 0

				return resp, nil
			},
		}
	}
}
