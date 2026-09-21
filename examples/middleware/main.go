package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"
	"time"

	wf "github.com/veggiemonk/workflow"
)

// The stages of an order. Each middleware method returns a new step, so the
// step it wraps is never changed and can be reused elsewhere.

type Order struct {
	ID       string
	Customer string
	Amount   float64
}

type Payment struct {
	Order Order
	Auth  string
	Paid  time.Time
}

type Reservation struct {
	Payment Payment
	Slot    string
}

type Shipment struct {
	Order    Order
	Tracking string
}

func (s Shipment) String() string {
	return fmt.Sprintf("Shipment{Order: %s, Tracking: %s}", s.Order.ID, s.Tracking)
}

var errGatewayDown = errors.New("payment service temporarily unavailable")

// gateway is a stateful step: it fails its first failFirst calls and then
// succeeds. A type with state implements Runner, and Of turns it into a Step.
type gateway struct {
	failFirst int32
	calls     atomic.Int32
}

func (g *gateway) Run(ctx context.Context, o Order) (Payment, error) {
	if g.calls.Add(1) <= g.failFirst {
		return Payment{}, errGatewayDown
	}
	select {
	case <-time.After(20 * time.Millisecond):
	case <-ctx.Done():
		return Payment{}, ctx.Err()
	}
	return Payment{Order: o, Auth: "AUTH-" + o.ID, Paid: time.Now()}, nil
}

// validate rejects a malformed order. Its error is not worth a retry.
var validate = wf.Func("Validate", func(_ context.Context, o Order) (Order, error) {
	switch {
	case o.ID == "":
		return Order{}, errors.New("order ID is required")
	case o.Amount <= 0:
		return Order{}, errors.New("order amount must be positive")
	}
	return o, nil
})

// reserve is slow on purpose: it is what the timeout example cuts short.
var reserve = wf.Func("Reserve", func(ctx context.Context, p Payment) (Reservation, error) {
	select {
	case <-time.After(200 * time.Millisecond):
	case <-ctx.Done():
		return Reservation{}, ctx.Err()
	}
	return Reservation{Payment: p, Slot: "WH-1"}, nil
})

var ship = wf.Pure("Ship", func(r Reservation) Shipment {
	return Shipment{Order: r.Payment.Order, Tracking: "TRK-" + r.Payment.Order.ID}
})

var retry = wf.RetryConfig{
	MaxAttempts:       3,
	InitialDelay:      50 * time.Millisecond,
	MaxDelay:          500 * time.Millisecond,
	BackoffMultiplier: 2,
	// Retry the gateway, never a rejected order.
	ShouldRetry: func(err error) bool { return errors.Is(err, errGatewayDown) },
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	fmt.Println("=== Order processing with middleware ===")

	example1(logger)
	separator()
	example2(logger)
	separator()
	example3(logger)
	separator()
	example4(logger)
}

// Example 1: middleware sits on the step that needs it, not on the pipeline.
// Only the payment call is retried; validation is not.
func example1(logger *slog.Logger) {
	fmt.Println("Example 1: a valid order")

	pay := wf.Of("Pay", &gateway{}).
		Retry(retry).
		Timeout(time.Second).
		Log(logger)

	pipeline := validate.Then(pay).Then(reserve).Then(ship).WithID()

	fmt.Println("Pipeline structure:")
	fmt.Println(pipeline)

	run(pipeline, Order{ID: "ORD-001", Customer: "CUST-123", Amount: 99.99})
}

// Example 2: the gateway fails twice, Retry waits and tries again.
func example2(logger *slog.Logger) {
	fmt.Println("Example 2: a flaky payment gateway")

	flaky := &gateway{failFirst: 2}
	pay := wf.Of("Pay", flaky).Retry(retry).Log(logger)
	pipeline := validate.Then(pay).Then(reserve).Then(ship)

	run(pipeline, Order{ID: "ORD-002", Customer: "CUST-456", Amount: 149.99})
	fmt.Printf("gateway calls: %d\n", flaky.calls.Load())
}

// Example 3: a CircuitBreaker is an explicit value. You decide what shares
// it. Here one breaker guards one step; after three failures it opens and
// the step is not called again until the timeout passes.
func example3(logger *slog.Logger) {
	fmt.Println("Example 3: a circuit breaker")

	breaker := wf.NewCircuitBreaker(wf.CircuitBreakerConfig{
		FailureThreshold: 3,
		OpenTimeout:      2 * time.Second,
		ShouldTrip:       func(err error) bool { return errors.Is(err, errGatewayDown) },
	})

	down := &gateway{failFirst: 100} // always down
	pay := wf.Of("Pay", down).Breaker(breaker).Log(logger)
	pipeline := validate.Then(pay).Then(reserve).Then(ship)

	for i := range 6 {
		order := Order{ID: fmt.Sprintf("CB-ORD-%03d", i+1), Customer: "CUST-CB", Amount: 50}
		_, err := pipeline.Run(context.Background(), order)
		switch {
		case errors.Is(err, wf.ErrCircuitOpen):
			fmt.Printf("Request %d: ⛔ circuit open, the gateway was not called\n", i+1)
		case err != nil:
			fmt.Printf("Request %d: ❌ %v\n", i+1, err)
		default:
			fmt.Printf("Request %d: ✅ ok\n", i+1)
		}
	}
	fmt.Printf("gateway calls: %d out of 6 requests\n", down.calls.Load())
}

// Example 4: Timeout gives the step a context with a deadline. It starts no
// goroutine, so the step must honour the context; Reserve does.
func example4(logger *slog.Logger) {
	fmt.Println("Example 4: a timeout")

	pay := wf.Of("Pay", &gateway{}).Log(logger)
	slow := reserve.Timeout(50 * time.Millisecond).Log(logger)
	pipeline := validate.Then(pay).Then(slow).Then(ship)

	run(pipeline, Order{ID: "TIMEOUT-ORD-001", Customer: "CUST-TIMEOUT", Amount: 75})
}

func run(pipeline wf.Step[Order, Shipment], order Order) {
	start := time.Now()
	shipment, err := pipeline.Run(context.Background(), order)
	took := time.Since(start).Round(time.Millisecond)
	if err != nil {
		fmt.Printf("❌ %s failed: %v (took %v)\n", order.ID, err, took)
		return
	}
	fmt.Printf("✅ %v (took %v)\n", shipment, took)
}

func separator() { fmt.Println("\n" + strings.Repeat("=", 50) + "\n") }
