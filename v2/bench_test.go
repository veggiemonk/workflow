package workflow_test

import (
	"context"
	"testing"
	"time"

	workflow "github.com/veggiemonk/workflow/v2"
)

func BenchmarkRun(b *testing.B) {
	s := workflow.Pure("Id", func(n int) int { return n + 1 })
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = s.Run(ctx, 1)
	}
}

func BenchmarkThenChain(b *testing.B) {
	inc := workflow.Pure("Inc", func(n int) int { return n + 1 })
	s := inc.Then(inc).Then(inc).Then(inc).Then(inc)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = s.Run(ctx, 0)
	}
}

func BenchmarkSeq(b *testing.B) {
	inc := workflow.Pure("Inc", func(n int) int { return n + 1 })
	s := workflow.Seq("Chain", inc, inc, inc, inc, inc)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = s.Run(ctx, 0)
	}
}

func BenchmarkPar(b *testing.B) {
	inc := workflow.Pure("Inc", func(n int) int { return n + 1 })
	s := inc.Par(inc, func(x, y int) (int, error) { return x + y, nil })
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = s.Run(ctx, 0)
	}
}

func BenchmarkEach100(b *testing.B) {
	s := workflow.Each(8, workflow.Pure("Inc", func(n int) int { return n + 1 }))
	in := make([]int, 100)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = s.Run(ctx, in)
	}
}

func BenchmarkMiddlewareStack(b *testing.B) {
	s := workflow.Pure("Id", func(n int) int { return n + 1 }).
		Retry(workflow.RetryConfig{MaxAttempts: 3}).
		Recover().
		Timeout(time.Minute)
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		_, _ = s.Run(ctx, 1)
	}
}
