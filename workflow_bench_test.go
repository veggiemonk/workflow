package workflow_test

import (
	"context"
	"testing"

	wf "github.com/veggiemonk/workflow"
)

type BenchData struct {
	Value   int
	Counter int
	Data    []byte
}

// BenchmarkPipelineExecution benchmarks basic pipeline execution
func BenchmarkPipelineExecution(b *testing.B) {
	pipeline := wf.NewPipeline[BenchData]()
	pipeline.Steps = []wf.Step[BenchData]{
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter++
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Value *= 2
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter += data.Value
			return data, nil
		}),
	}

	for i := 0; b.Loop(); i++ {
		_, err := pipeline.Run(context.Background(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkParallelExecution benchmarks parallel step execution
func BenchmarkParallelExecution(b *testing.B) {
	parallelStep := wf.Parallel(nil, wf.Merge[BenchData],
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter++
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Value *= 2
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Data = make([]byte, 100)
			return data, nil
		}),
	)

	for i := 0; b.Loop(); i++ {
		_, err := parallelStep.Run(b.Context(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSeriesExecution benchmarks series step execution
func BenchmarkSeriesExecution(b *testing.B) {
	seriesStep := wf.Sequential(nil,
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter++
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Value *= 2
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Data = make([]byte, 100)
			return data, nil
		}),
	)

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := seriesStep.Run(b.Context(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkComplexPipeline benchmarks a complex nested pipeline
func BenchmarkComplexPipeline(b *testing.B) {
	// Create complex pipeline with nested parallel and series steps
	innerParallel := wf.Parallel(nil, wf.Merge[BenchData],
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter++
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Value += 10
			return data, nil
		}),
	)

	innerSeries := wf.Sequential(nil,
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Value *= 2
			return data, nil
		}),
		innerParallel,
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter += data.Value
			return data, nil
		}),
	)

	pipeline := wf.NewPipeline[BenchData]()
	pipeline.Steps = []wf.Step[BenchData]{
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Data = make([]byte, 50)
			return data, nil
		}),
		innerSeries,
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Data = append(data.Data, make([]byte, 50)...)
			return data, nil
		}),
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := pipeline.Run(b.Context(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMiddlewareOverhead benchmarks middleware overhead
func BenchmarkMiddlewareOverhead(b *testing.B) {
	noopMiddleware := func(next wf.Step[BenchData]) wf.Step[BenchData] {
		return &wf.MidFunc[BenchData]{
			Name: "Noop",
			Next: next,
			Fn: func(ctx context.Context, data *BenchData) (*BenchData, error) {
				return next.Run(ctx, data)
			},
		}
	}

	// Create pipeline with multiple middleware layers
	pipeline := wf.NewPipeline(
		noopMiddleware,
		noopMiddleware,
		noopMiddleware,
	)
	pipeline.Steps = []wf.Step[BenchData]{
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter++
			return data, nil
		}),
	}

	b.ResetTimer()
	for i := 0; b.Loop(); i++ {
		_, err := pipeline.Run(b.Context(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkHighParallelism benchmarks high parallelism scenarios
func BenchmarkHighParallelism(b *testing.B) {
	// Create many parallel steps
	steps := make([]wf.Step[BenchData], 100)
	for i := range steps {
		steps[i] = wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			data.Counter++
			return data, nil
		})
	}

	parallelStep := wf.Parallel(nil, wf.Merge[BenchData], steps...)

	for i := 0; b.Loop(); i++ {
		_, err := parallelStep.Run(b.Context(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMemoryAllocation benchmarks memory allocation patterns
func BenchmarkMemoryAllocation(b *testing.B) {
	pipeline := wf.NewPipeline[BenchData]()
	pipeline.Steps = []wf.Step[BenchData]{
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			// Allocate memory to test GC pressure
			data.Data = make([]byte, 1024)
			return data, nil
		}),
		wf.StepFunc[BenchData](func(_ context.Context, data *BenchData) (*BenchData, error) {
			// Copy data to test allocation patterns
			newData := make([]byte, len(data.Data))
			copy(newData, data.Data)
			data.Data = newData
			return data, nil
		}),
	}

	b.ResetTimer()
	b.ReportAllocs() // Report memory allocations
	for i := 0; b.Loop(); i++ {
		_, err := pipeline.Run(b.Context(), &BenchData{Value: i})
		if err != nil {
			b.Fatal(err)
		}
	}
}
