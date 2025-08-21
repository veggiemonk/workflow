package workflow

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ccoveille/go-safecast"
)

// TestData represents test data for middleware tests.
type TestData struct {
	Value   string
	Counter int
}

// String implements fmt.Stringer for TestData.
func (t *TestData) String() string {
	return fmt.Sprintf("TestData{Value: %s, Counter: %d}", t.Value, t.Counter)
}

// failingStep is a test step that fails for a specified number of attempts.
type failingStep struct {
	failCount   int32
	attempts    int32
	shouldFail  func(attempt int) bool
	processTime time.Duration
}

func (f *failingStep) Run(ctx context.Context, req *TestData) (*TestData, error) {
	attempt := atomic.AddInt32(&f.attempts, 1)

	if f.processTime > 0 {
		select {
		case <-time.After(f.processTime):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if f.shouldFail != nil && f.shouldFail(int(attempt)) {
		return nil, fmt.Errorf("step failed on attempt %d", attempt)
	}

	if atomic.LoadInt32(&f.failCount) > 0 {
		atomic.AddInt32(&f.failCount, -1)
		return nil, fmt.Errorf("step failed on attempt %d", attempt)
	}

	return &TestData{
		Value:   req.Value + "_processed",
		Counter: req.Counter + 1,
	}, nil
}

func (f *failingStep) String() string {
	return "failingStep"
}

func (f *failingStep) Reset() {
	atomic.StoreInt32(&f.attempts, 0)
}

func (f *failingStep) GetAttempts() int {
	return int(atomic.LoadInt32(&f.attempts))
}

func TestRetryMiddleware(t *testing.T) {
	tests := []struct {
		name           string
		config         RetryConfig
		step           *failingStep
		expectSuccess  bool
		expectAttempts int
		expectError    string
	}{
		{
			name: "success on first attempt",
			config: RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 10 * time.Millisecond,
			},
			step:           &failingStep{failCount: 0},
			expectSuccess:  true,
			expectAttempts: 1,
		},
		{
			name: "success on second attempt",
			config: RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 10 * time.Millisecond,
			},
			step:           &failingStep{failCount: 1},
			expectSuccess:  true,
			expectAttempts: 2,
		},
		{
			name: "fail all attempts",
			config: RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 10 * time.Millisecond,
			},
			step:           &failingStep{failCount: 5},
			expectSuccess:  false,
			expectAttempts: 3,
			expectError:    "step failed after 3 attempts",
		},
		{
			name: "custom should retry function - skip retry",
			config: RetryConfig{
				MaxAttempts:  3,
				InitialDelay: 10 * time.Millisecond,
				ShouldRetry: func(_ error) bool {
					return false // never retry
				},
			},
			step:           &failingStep{failCount: 1},
			expectSuccess:  false,
			expectAttempts: 1,
		},
		{
			name: "exponential backoff with max delay",
			config: RetryConfig{
				MaxAttempts:       4,
				InitialDelay:      10 * time.Millisecond,
				MaxDelay:          50 * time.Millisecond,
				BackoffMultiplier: 2.0,
			},
			step:           &failingStep{failCount: 3},
			expectSuccess:  true,
			expectAttempts: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.step.Reset()

			middleware := RetryMiddleware[TestData](tt.config)
			wrappedStep := middleware(tt.step)

			ctx := context.Background()
			req := &TestData{Value: "test", Counter: 0}

			start := time.Now()
			resp, err := wrappedStep.Run(ctx, req)
			duration := time.Since(start)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("expected success but got error: %v", err)
				}
				if resp == nil {
					t.Error("expected response but got nil")
				} else if resp.Value != "test_processed" {
					t.Errorf("expected processed value but got: %s", resp.Value)
				}
			} else {
				if err == nil {
					t.Error("expected error but got success")
				}
				if tt.expectError != "" && !errors.Is(err, errors.New(tt.expectError)) {
					// Check if error message contains expected text
					if err.Error() == "" || err.Error()[:len(tt.expectError)] != tt.expectError {
						t.Errorf("expected error containing '%s' but got: %v", tt.expectError, err)
					}
				}
			}

			if tt.step.GetAttempts() != tt.expectAttempts {
				t.Errorf("expected %d attempts but got %d", tt.expectAttempts, tt.step.GetAttempts())
			}

			// Verify that retries include delays (except for the last attempt)
			if tt.expectAttempts > 1 && tt.expectSuccess {
				expectedMinDuration := tt.config.InitialDelay
				if duration < expectedMinDuration {
					t.Errorf("expected minimum duration %v but got %v", expectedMinDuration, duration)
				}
			}
		})
	}
}

func TestRetryMiddlewareWithContext(t *testing.T) {
	step := &failingStep{failCount: 5}
	config := RetryConfig{
		MaxAttempts:  5,
		InitialDelay: 100 * time.Millisecond,
	}

	middleware := RetryMiddleware[TestData](config)
	wrappedStep := middleware(step)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	req := &TestData{Value: "test", Counter: 0}

	_, err := wrappedStep.Run(ctx, req)

	if err == nil {
		t.Error("expected context timeout error")
	}

	if !errors.Is(err, context.DeadlineExceeded) && err.Error() != "context deadline exceeded" {
		t.Errorf("expected context deadline exceeded but got: %v", err)
	}
}

func TestTimeoutMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		timeout       time.Duration
		processTime   time.Duration
		expectSuccess bool
		expectTimeout bool
	}{
		{
			name:          "completes within timeout",
			timeout:       100 * time.Millisecond,
			processTime:   50 * time.Millisecond,
			expectSuccess: true,
			expectTimeout: false,
		},
		{
			name:          "times out",
			timeout:       50 * time.Millisecond,
			processTime:   100 * time.Millisecond,
			expectSuccess: false,
			expectTimeout: true,
		},
		{
			name:          "instant completion",
			timeout:       100 * time.Millisecond,
			processTime:   0,
			expectSuccess: true,
			expectTimeout: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			step := &failingStep{processTime: tt.processTime}

			middleware := TimeoutMiddleware[TestData](tt.timeout)
			wrappedStep := middleware(step)

			ctx := context.Background()
			req := &TestData{Value: "test", Counter: 0}

			start := time.Now()
			resp, err := wrappedStep.Run(ctx, req)
			duration := time.Since(start)

			if tt.expectSuccess {
				if err != nil {
					t.Errorf("expected success but got error: %v", err)
				}
				if resp == nil {
					t.Error("expected response but got nil")
				}
			} else {
				if err == nil {
					t.Error("expected error but got success")
				}
				if tt.expectTimeout {
					expectedMsg := fmt.Sprintf("step timed out after %v", tt.timeout)
					if err.Error()[:len(expectedMsg)] != expectedMsg {
						t.Errorf("expected timeout error but got: %v", err)
					}
				}
			}

			// Verify timeout enforcement
			if tt.expectTimeout {
				// Should timeout around the specified duration (with some tolerance)
				if duration < tt.timeout || duration > tt.timeout+50*time.Millisecond {
					t.Errorf("expected timeout around %v but took %v", tt.timeout, duration)
				}
			}
		})
	}
}

func TestCircuitBreakerMiddleware(t *testing.T) {
	tests := []struct {
		name            string
		config          CircuitBreakerConfig
		failures        int
		expectOpenAfter int
		testSequence    []bool // true = expect success, false = expect failure
	}{
		{
			name: "opens after threshold failures",
			config: CircuitBreakerConfig{
				FailureThreshold: 3,
				OpenTimeout:      100 * time.Millisecond,
			},
			failures:        5,
			expectOpenAfter: 3,
			testSequence:    []bool{false, false, false, false}, // 4th should be blocked
		},
		{
			name: "recovers after timeout",
			config: CircuitBreakerConfig{
				FailureThreshold: 2,
				OpenTimeout:      50 * time.Millisecond,
			},
			failures:        3,
			expectOpenAfter: 2,
		},
		{
			name: "custom should trip function",
			config: CircuitBreakerConfig{
				FailureThreshold: 2,
				OpenTimeout:      100 * time.Millisecond,
				ShouldTrip: func(err error) bool {
					return err.Error() == "critical"
				},
			},
			failures: 0, // Step won't fail in the normal sense
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b, err := safecast.ToInt32(tt.failures) // everything is fine
			if err != nil {
				t.Error(err)
			}
			step := &failingStep{failCount: b}

			middleware := CircuitBreakerMiddleware[TestData](tt.config)
			wrappedStep := middleware(step)

			ctx := context.Background()
			req := &TestData{Value: "test", Counter: 0}

			if len(tt.testSequence) > 0 {
				for i, expectSuccess := range tt.testSequence {
					_, err := wrappedStep.Run(ctx, req)

					if expectSuccess && err != nil {
						t.Errorf("attempt %d: expected success but got error: %v", i+1, err)
					} else if !expectSuccess && err == nil {
						t.Errorf("attempt %d: expected failure but got success", i+1)
					}

					// Check for circuit breaker specific error
					if !expectSuccess && i >= tt.expectOpenAfter-1 {
						if err.Error() != "circuit breaker is open" {
							// First few failures should be from the step, later ones from circuit breaker
							if i >= tt.expectOpenAfter && err.Error() != "circuit breaker is open" {
								t.Errorf("attempt %d: expected circuit breaker error but got: %v", i+1, err)
							}
						}
					}
				}
			} else {
				// Test recovery after timeout
				var failureCount int

				// Trigger failures to open circuit
				for i := 0; i < tt.expectOpenAfter+1; i++ {
					_, err := wrappedStep.Run(ctx, req)
					if err != nil {
						failureCount++
					}
				}

				if failureCount < tt.expectOpenAfter {
					t.Errorf("expected at least %d failures but got %d", tt.expectOpenAfter, failureCount)
				}

				// Wait for circuit to potentially recover
				time.Sleep(tt.config.OpenTimeout + 10*time.Millisecond)

				// Reset the step to succeed
				step.failCount = 0

				// Should allow one test request (half-open)
				_, err := wrappedStep.Run(ctx, req)
				if err != nil {
					t.Errorf("expected recovery attempt to succeed but got: %v", err)
				}
			}
		})
	}
}

func TestMiddlewareCombination(t *testing.T) {
	// Test combining retry and timeout middleware
	step := &failingStep{
		failCount: 2, // Fail twice, then succeed
	}

	retryMiddleware := RetryMiddleware[TestData](RetryConfig{
		MaxAttempts:  4,
		InitialDelay: 10 * time.Millisecond,
	})

	timeoutMiddleware := TimeoutMiddleware[TestData](200 * time.Millisecond)

	// Apply both middleware (timeout wraps retry)
	wrappedStep := timeoutMiddleware(retryMiddleware(step))

	ctx := context.Background()
	req := &TestData{Value: "test", Counter: 0}

	resp, err := wrappedStep.Run(ctx, req)
	if err != nil {
		t.Errorf("expected success but got error: %v", err)
	}

	if resp == nil {
		t.Error("expected response but got nil")
	}

	if step.GetAttempts() != 3 {
		t.Errorf("expected 3 attempts but got %d", step.GetAttempts())
	}
}

func TestDefaultConfigs(t *testing.T) {
	t.Run("default retry config", func(t *testing.T) {
		config := DefaultRetryConfig()

		if config.MaxAttempts != 3 {
			t.Errorf("expected MaxAttempts=3 but got %d", config.MaxAttempts)
		}

		if config.InitialDelay != 100*time.Millisecond {
			t.Errorf("expected InitialDelay=100ms but got %v", config.InitialDelay)
		}

		if config.MaxDelay != 5*time.Second {
			t.Errorf("expected MaxDelay=5s but got %v", config.MaxDelay)
		}

		if config.BackoffMultiplier != 2.0 {
			t.Errorf("expected BackoffMultiplier=2.0 but got %f", config.BackoffMultiplier)
		}
	})

	t.Run("default circuit breaker config", func(t *testing.T) {
		config := DefaultCircuitBreakerConfig()

		if config.FailureThreshold != 5 {
			t.Errorf("expected FailureThreshold=5 but got %d", config.FailureThreshold)
		}

		if config.OpenTimeout != 60*time.Second {
			t.Errorf("expected OpenTimeout=60s but got %v", config.OpenTimeout)
		}
	})
}

func TestMiddlewareInPipeline(t *testing.T) {
	// Test middleware integration with Pipeline
	step1 := &failingStep{failCount: 1} // Fails once then succeeds
	step2 := StepFunc[TestData](func(_ context.Context, req *TestData) (*TestData, error) {
		return &TestData{
			Value:   req.Value + "_step2",
			Counter: req.Counter + 1,
		}, nil
	})

	retryMiddleware := RetryMiddleware[TestData](DefaultRetryConfig())
	timeoutMiddleware := TimeoutMiddleware[TestData](1 * time.Second)

	pipeline := NewPipeline(retryMiddleware, timeoutMiddleware)
	pipeline.Steps = []Step[TestData]{step1, step2}

	ctx := context.Background()
	req := &TestData{Value: "initial", Counter: 0}

	resp, err := pipeline.Run(ctx, req)
	if err != nil {
		t.Errorf("expected success but got error: %v", err)
	}

	if resp == nil {
		t.Error("expected response but got nil")
		return
	}

	expectedValue := "initial_processed_step2"
	if resp.Value != expectedValue {
		t.Errorf("expected value '%s' but got '%s'", expectedValue, resp.Value)
	}

	if resp.Counter != 2 {
		t.Errorf("expected counter=2 but got %d", resp.Counter)
	}

	// Verify retry was used (step1 should have been called twice)
	if step1.GetAttempts() != 2 {
		t.Errorf("expected step1 to be called 2 times but was called %d times", step1.GetAttempts())
	}
}

func TestLoggerMiddleware(t *testing.T) {
	tests := []struct {
		name        string
		step        Step[TestData]
		expectError bool
		expectLogs  []string // Expected log messages to contain these substrings
	}{
		{
			name: "successful step execution",
			step: StepFunc[TestData](func(_ context.Context, req *TestData) (*TestData, error) {
				return &TestData{
					Value:   req.Value + "_logged",
					Counter: req.Counter + 1,
				}, nil
			}),
			expectError: false,
			expectLogs:  []string{"start", "done", "StepFunc", "duration"},
		},
		{
			name: "failing step execution",
			step: StepFunc[TestData](func(_ context.Context, req *TestData) (*TestData, error) {
				return nil, errors.New("test error")
			}),
			expectError: true,
			expectLogs:  []string{"start", "done", "StepFunc", "duration"},
		},
		{
			name:        "step with custom name",
			step:        &failingStep{failCount: 0},
			expectError: false,
			expectLogs:  []string{"start", "done", "failingStep", "duration"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a custom logger that captures log output
			var logOutput []string
			logger := slog.New(slog.NewTextHandler(&logWriter{
				write: func(p []byte) (int, error) {
					logOutput = append(logOutput, string(p))
					return len(p), nil
				},
			}, &slog.HandlerOptions{
				Level: slog.LevelInfo,
				ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
					if a.Key == "duration" {
						a.Value = slog.StringValue("1s")
					}
					if a.Key == slog.TimeKey {
						return slog.Attr{}
					}
					return a
				},
			}))

			middleware := LoggerMiddleware[TestData](logger)
			wrappedStep := middleware(tt.step)

			ctx := context.Background()
			req := &TestData{Value: "test", Counter: 0}

			resp, err := wrappedStep.Run(ctx, req)

			// Check error expectation
			if tt.expectError && err == nil {
				t.Error("expected error but got success")
			} else if !tt.expectError && err != nil {
				t.Errorf("expected success but got error: %v", err)
			}

			// Check response for successful cases
			if !tt.expectError {
				if resp == nil {
					t.Error("expected response but got nil")
				} else if tt.step.String() == "StepFunc" {
					// For the successful StepFunc test
					expectedValue := "test_logged"
					if resp.Value != expectedValue {
						t.Errorf("expected value '%s' but got '%s'", expectedValue, resp.Value)
					}
					if resp.Counter != 1 {
						t.Errorf("expected counter=1 but got %d", resp.Counter)
					}
				}
			}

			// Verify log messages contain expected content
			for _, expectedLog := range tt.expectLogs {
				found := false
				for _, log := range logOutput {
					if strings.Contains(log, expectedLog) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected log to contain '%s' but logs were: %v", expectedLog, logOutput)
				}
			}

			// Verify we have both start and done logs
			hasStart := false
			hasDone := false
			for _, log := range logOutput {
				if strings.Contains(log, "start") {
					hasStart = true
				}
				if strings.Contains(log, "done") {
					hasDone = true
				}
			}

			if !hasStart {
				t.Error("expected start log message")
			}
			if !hasDone {
				t.Error("expected done log message")
			}
		})
	}
}

func TestUUIDMiddleware(t *testing.T) {
	step := StepFunc[TestData](func(ctx context.Context, req *TestData) (*TestData, error) {
		// Verify UUID is present in context
		uuid, ok := ctx.Value(StepUUIDKey).(string)
		if !ok || uuid == "" {
			return nil, errors.New("UUID not found in context")
		}

		// Return the UUID in the response for verification
		return &TestData{
			Value:   req.Value + "_" + uuid[:8], // Use first 8 chars of UUID
			Counter: req.Counter + 1,
		}, nil
	})

	middleware := UUIDMiddleware[TestData]()
	wrappedStep := middleware(step)

	ctx := context.Background()
	req := &TestData{Value: "test", Counter: 0}

	resp, err := wrappedStep.Run(ctx, req)
	if err != nil {
		t.Errorf("expected success but got error: %v", err)
	}

	if resp == nil {
		t.Error("expected response but got nil")
		return
	}

	// Verify the UUID was added to the value
	if len(resp.Value) != len("test_")+8 { // "test_" + 8 char UUID prefix
		t.Errorf("expected value to contain UUID prefix, got: %s", resp.Value)
	}

	if resp.Counter != 1 {
		t.Errorf("expected counter=1 but got %d", resp.Counter)
	}

	// Test that each execution gets a different UUID
	resp2, err := wrappedStep.Run(ctx, req)
	if err != nil {
		t.Errorf("expected success on second run but got error: %v", err)
	}

	if resp2 != nil && resp.Value == resp2.Value {
		t.Error("expected different UUIDs for each execution")
	}
}

// Helper types and functions for logger testing
type logWriter struct {
	write func([]byte) (int, error)
}

func (w *logWriter) Write(p []byte) (int, error) {
	return w.write(p)
}

// // Helper function to check if a string contains a substring
// func contains(s, substr string) bool {
// 	return len(s) >= len(substr) && (s == substr ||
// 		(len(s) > len(substr) &&
// 			(s[:len(substr)] == substr ||
// 				s[len(s)-len(substr):] == substr ||
// 				containsInMiddle(s, substr))))
// }

// func containsInMiddle(s, substr string) bool {
// 	for i := 0; i <= len(s)-len(substr); i++ {
// 		if s[i:i+len(substr)] == substr {
// 			return true
// 		}
// 	}
// 	return false
// }
