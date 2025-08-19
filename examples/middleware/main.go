package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"os"
	"strings"
	"time"

	wf "github.com/veggiemonk/workflow"
)

// OrderData represents an order processing workflow data.
type OrderData struct {
	OrderID     string
	CustomerID  string
	Amount      float64
	Status      string
	ProcessedAt time.Time
	Retries     int
}

func (o OrderData) String() string {
	return fmt.Sprintf("Order{ID: %s, Customer: %s, Amount: %.2f, Status: %s, Retries: %d}",
		o.OrderID, o.CustomerID, o.Amount, o.Status, o.Retries)
}

// validateOrderStep validates the order data.
type validateOrderStep struct{}

func (v *validateOrderStep) Run(ctx context.Context, order *OrderData) (*OrderData, error) {
	if order.OrderID == "" {
		return nil, errors.New("order ID is required")
	}
	if order.Amount <= 0 {
		return nil, errors.New("order amount must be positive")
	}

	order.Status = "validated"
	return order, nil
}

func (v *validateOrderStep) String() string {
	return "ValidateOrder"
}

// paymentStep processes payment (simulates external service that might fail).
type paymentStep struct{}

func (p *paymentStep) Run(ctx context.Context, order *OrderData) (*OrderData, error) {
	// Simulate random failures (30% chance)
	if rand.Float64() < 0.3 {
		return nil, errors.New("payment service temporarily unavailable")
	}

	// Simulate slow processing
	select {
	case <-time.After(100 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	order.Status = "paid"
	order.ProcessedAt = time.Now()
	return order, nil
}

func (p *paymentStep) String() string {
	return "ProcessPayment"
}

// inventoryStep checks and reserves inventory (might be slow).
type inventoryStep struct{}

func (i *inventoryStep) Run(ctx context.Context, order *OrderData) (*OrderData, error) {
	// Simulate slow inventory check
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Simulate occasional inventory issues (10% chance)
	if rand.Float64() < 0.1 {
		return nil, errors.New("insufficient inventory")
	}

	order.Status = "inventory_reserved"
	return order, nil
}

func (i *inventoryStep) String() string {
	return "CheckInventory"
}

// shippingStep creates shipping label.
type shippingStep struct{}

func (s *shippingStep) Run(ctx context.Context, order *OrderData) (*OrderData, error) {
	order.Status = "shipped"
	return order, nil
}

func (s *shippingStep) String() string {
	return "CreateShipping"
}

func main() {
	// Set up logger
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Create middleware configurations
	retryConfig := wf.RetryConfig{
		MaxAttempts:       3,
		InitialDelay:      50 * time.Millisecond,
		MaxDelay:          500 * time.Millisecond,
		BackoffMultiplier: 2.0,
		ShouldRetry: func(err error) bool {
			// Only retry for specific errors
			return err.Error() == "payment service temporarily unavailable" ||
				err.Error() == "insufficient inventory"
		},
	}

	circuitBreakerConfig := wf.CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenTimeout:      2 * time.Second,
		ShouldTrip: func(err error) bool {
			// Trip circuit breaker for service unavailability
			return err.Error() == "payment service temporarily unavailable"
		},
	}

	// Create middleware
	retryMiddleware := wf.RetryMiddleware[OrderData](retryConfig)
	timeoutMiddleware := wf.TimeoutMiddleware[OrderData](1 * time.Second)
	circuitBreakerMiddleware := wf.CircuitBreakerMiddleware[OrderData](circuitBreakerConfig)
	loggerMiddleware := wf.LoggerMiddleware[OrderData](logger)
	uuidMiddleware := wf.UUIDMiddleware[OrderData]()

	fmt.Println("=== Order Processing Pipeline with Middleware ===")
	fmt.Println()

	// Example 1: Successful order processing
	fmt.Println("Example 1: Processing a valid order")
	runOrderProcessing(
		"ORD-001", "CUST-123", 99.99,
		retryMiddleware, timeoutMiddleware, loggerMiddleware, uuidMiddleware,
	)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 2: Order with retry scenarios
	fmt.Println("Example 2: Processing order with potential retries")
	// Process multiple orders to show retry behavior
	for i := 0; i < 3; i++ {
		orderID := fmt.Sprintf("ORD-00%d", i+2)
		fmt.Printf("Processing order %s:\n", orderID)
		runOrderProcessing(
			orderID, "CUST-456", 149.99,
			retryMiddleware, timeoutMiddleware, loggerMiddleware,
		)
		fmt.Println()
	}

	fmt.Println(strings.Repeat("=", 50) + "\n")

	// Example 3: With circuit breaker
	fmt.Println("Example 3: Processing with circuit breaker protection")
	runOrderProcessingWithCircuitBreaker(
		retryMiddleware, timeoutMiddleware, circuitBreakerMiddleware, loggerMiddleware,
	)

	fmt.Println("\n" + strings.Repeat("=", 50) + "\n")

	// Example 4: Timeout demonstration
	fmt.Println("Example 4: Demonstrating timeout protection")
	runOrderProcessingWithTimeout(loggerMiddleware)
}

func runOrderProcessing(orderID, customerID string, amount float64, middleware ...wf.Middleware[OrderData]) {
	// Create pipeline with middleware
	pipeline := wf.NewPipeline(middleware...)

	// Add steps to pipeline
	pipeline.Steps = []wf.Step[OrderData]{
		&validateOrderStep{},
		&paymentStep{},
		&inventoryStep{},
		&shippingStep{},
	}

	// Print pipeline structure
	fmt.Println("Pipeline structure:")
	fmt.Println(pipeline.String())

	// Create order data
	order := &OrderData{
		OrderID:    orderID,
		CustomerID: customerID,
		Amount:     amount,
		Status:     "created",
	}

	fmt.Printf("Processing order: %s\n", order)

	// Execute pipeline
	ctx := context.Background()
	start := time.Now()

	result, err := pipeline.Run(ctx, order)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Order processing failed: %v (took %v)\n", err, duration)
	} else {
		fmt.Printf("✅ Order processed successfully: %s (took %v)\n", result, duration)
	}
}

func runOrderProcessingWithCircuitBreaker(middleware ...wf.Middleware[OrderData]) {
	// Create pipeline with circuit breaker
	pipeline := wf.NewPipeline[OrderData](middleware...)
	pipeline.Steps = []wf.Step[OrderData]{
		&validateOrderStep{},
		&paymentStep{}, // This step might trigger circuit breaker
		&shippingStep{},
	}

	fmt.Println("Testing circuit breaker with multiple failing requests...")

	// Process multiple orders to potentially trigger circuit breaker
	for i := 0; i < 7; i++ {
		order := &OrderData{
			OrderID:    fmt.Sprintf("CB-ORD-%03d", i+1),
			CustomerID: "CUST-CB",
			Amount:     50.00,
			Status:     "created",
		}

		ctx := context.Background()
		start := time.Now()

		result, err := pipeline.Run(ctx, order)
		duration := time.Since(start)

		if err != nil {
			fmt.Printf("Request %d: ❌ Failed - %v (took %v)\n", i+1, err, duration)
		} else {
			fmt.Printf("Request %d: ✅ Success - %s (took %v)\n", i+1, result, duration)
		}

		// Small delay between requests
		time.Sleep(10 * time.Millisecond)
	}
}

func runOrderProcessingWithTimeout(middleware ...wf.Middleware[OrderData]) {
	// Create a very short timeout to demonstrate timeout protection
	shortTimeoutMiddleware := wf.TimeoutMiddleware[OrderData](50 * time.Millisecond)

	// Combine with other middleware
	allMiddleware := append([]wf.Middleware[OrderData]{shortTimeoutMiddleware}, middleware...)

	pipeline := wf.NewPipeline(allMiddleware...)
	pipeline.Steps = []wf.Step[OrderData]{
		&validateOrderStep{},
		&inventoryStep{}, // This step takes 200ms, will timeout
		&shippingStep{},
	}

	order := &OrderData{
		OrderID:    "TIMEOUT-ORD-001",
		CustomerID: "CUST-TIMEOUT",
		Amount:     75.00,
		Status:     "created",
	}

	fmt.Printf("Processing order with 50ms timeout: %s\n", order)

	ctx := context.Background()
	start := time.Now()

	result, err := pipeline.Run(ctx, order)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("❌ Order processing timed out: %v (took %v)\n", err, duration)
	} else {
		fmt.Printf("✅ Order processed successfully: %s (took %v)\n", result, duration)
	}
}
