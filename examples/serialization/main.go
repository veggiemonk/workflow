package main

import (
	"context"
	"fmt"
	"log"

	"github.com/veggiemonk/workflow"
)

type MyData struct {
	Value int
}

func main() {
	// 1. Create a registry and register tasks, selectors, and merge functions
	registry := workflow.NewRegistry[MyData]()

	// Register tasks
	registry.Register(workflow.NewTask("task1", workflow.StepFunc[MyData](func(ctx context.Context, data *MyData) (*MyData, error) {
		data.Value++
		fmt.Println("Executing task1, value is now:", data.Value)
		return data, nil
	})))
	registry.Register(workflow.NewTask("task2", workflow.StepFunc[MyData](func(ctx context.Context, data *MyData) (*MyData, error) {
		data.Value *= 2
		fmt.Println("Executing task2, value is now:", data.Value)
		return data, nil
	})))

	// Register a selector function
	registry.RegisterSelector("is_positive", func(ctx context.Context, data *MyData) bool {
		return data.Value > 0
	})

	// Register a merge function
	registry.RegisterMergeRequest("sum_values", func(ctx context.Context, original *MyData, responses ...*MyData) (*MyData, error) {
		for _, resp := range responses {
			original.Value += resp.Value
		}
		return original, nil
	})

	// 2. Build a pipeline with a series
	pipeline := workflow.NewPipeline[MyData]()
	task1, _ := registry.Get("task1")
	task2, _ := registry.Get("task2")

	seriesTask := workflow.NewTask("my_series", workflow.Sequential(nil, task1, task2))
	pipeline.Tasks = append(pipeline.Tasks, seriesTask)

	// 3. Save the pipeline to a file
	filePath := "pipeline.json"
	if err := pipeline.Save(filePath); err != nil {
		log.Fatalf("Error saving pipeline: %v", err)
	}
	fmt.Println("Pipeline saved to", filePath)

	// 4. Build a new pipeline from the file
	builder := workflow.NewBuilder(registry)
	loadedPipeline, err := builder.BuildFromFile(filePath)
	if err != nil {
		log.Fatalf("Error building pipeline from file: %v", err)
	}
	fmt.Println("Pipeline loaded from", filePath)

	// 5. Run the loaded pipeline
	initialData := &MyData{Value: 10}
	fmt.Println("Running loaded pipeline with initial value:", initialData.Value)
	result, err := loadedPipeline.Run(context.Background(), initialData)
	if err != nil {
		log.Fatalf("Error running loaded pipeline: %v", err)
	}

	fmt.Println("Pipeline finished. Final value:", result.Value)
	// Expected: (10 + 1) * 2 = 22
}
